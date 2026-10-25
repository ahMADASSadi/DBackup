package repository

import (
	"context"
	"database/sql"
	"time"

	"dbackup/internal/model"
)

func (s *Store) ListSources(ctx context.Context) ([]model.Source, error) {
	rows, err := s.DB.QueryContext(ctx, sourceSelect()+` ORDER BY sources.name COLLATE NOCASE, sources.created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Source
	for rows.Next() {
		src, err := scanSource(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, src)
	}
	return out, rows.Err()
}

func (s *Store) GetSource(ctx context.Context, id string) (model.Source, error) {
	row := s.DB.QueryRowContext(ctx, sourceSelect()+` WHERE sources.id = ?`, id)
	src, err := scanSource(row)
	if err != nil {
		return model.Source{}, err
	}
	return src, nil
}

func (s *Store) CreateSource(ctx context.Context, src model.Source) error {
	cfg, err := marshalConfig(src.Config)
	if err != nil {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	now := formatTime(time.Now())
	if _, err := tx.ExecContext(ctx, `INSERT INTO sources(id, name, db_type, enabled, config_json, password_ciphertext, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		src.ID, src.Name, src.Type, boolInt(src.Enabled), cfg, src.PasswordCipher, now, now); err != nil {
		return err
	}
	if err := insertSchedule(ctx, tx, src.ID, src.Schedule, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) UpdateSource(ctx context.Context, src model.Source) error {
	cfg, err := marshalConfig(src.Config)
	if err != nil {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	now := formatTime(time.Now())
	res, err := tx.ExecContext(ctx, `UPDATE sources SET name = ?, enabled = ?, config_json = ?, password_ciphertext = ?, updated_at = ? WHERE id = ?`,
		src.Name, boolInt(src.Enabled), cfg, src.PasswordCipher, now, src.ID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	if err := upsertSchedule(ctx, tx, src.ID, src.Schedule, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) DeleteSource(ctx context.Context, id string) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM sources WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) CountBackups(ctx context.Context, sourceID string) (int, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM backups WHERE source_id = ?`, sourceID).Scan(&n)
	return n, err
}

func (s *Store) ListDueSchedules(ctx context.Context, now time.Time) ([]model.Source, error) {
	rows, err := s.DB.QueryContext(ctx, sourceSelect()+`
		WHERE sources.enabled = 1 AND schedules.enabled = 1
		AND schedules.next_run_at IS NOT NULL AND schedules.next_run_at <= ?
		ORDER BY schedules.next_run_at`, formatTime(now))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Source
	for rows.Next() {
		src, err := scanSource(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, src)
	}
	return out, rows.Err()
}

func (s *Store) MarkScheduleRan(ctx context.Context, sourceID string, last, next time.Time) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE schedules SET last_run_at = ?, next_run_at = ?, updated_at = ? WHERE source_id = ?`,
		formatTime(last), formatTime(next), formatTime(time.Now()), sourceID)
	return err
}

func sourceSelect() string {
	return `SELECT sources.id, sources.name, sources.db_type, sources.enabled, sources.config_json, sources.password_ciphertext,
		sources.created_at, sources.updated_at,
		schedules.enabled, schedules.mode, schedules.interval_seconds, schedules.cron_expr, schedules.timezone, schedules.upload,
		schedules.local_retention_count, schedules.local_retention_days, schedules.remote_retention_count, schedules.remote_retention_days,
		schedules.last_run_at, schedules.next_run_at
		FROM sources JOIN schedules ON schedules.source_id = sources.id`
}

type scanner interface {
	Scan(dest ...any) error
}

func scanSource(row scanner) (model.Source, error) {
	var src model.Source
	var enabled, schEnabled, upload int
	var configRaw, created, updated string
	var interval, localCount, localDays, remoteCount, remoteDays sql.NullInt64
	var cron, timezone sql.NullString
	var lastRun, nextRun sql.NullString
	err := row.Scan(
		&src.ID, &src.Name, &src.Type, &enabled, &configRaw, &src.PasswordCipher, &created, &updated,
		&schEnabled, &src.Schedule.Mode, &interval, &cron, &timezone, &upload,
		&localCount, &localDays, &remoteCount, &remoteDays, &lastRun, &nextRun,
	)
	if err != nil {
		return model.Source{}, err
	}
	src.Enabled = enabled == 1
	src.PasswordSet = src.PasswordCipher != ""
	src.Config, err = unmarshalConfig(configRaw)
	if err != nil {
		return model.Source{}, err
	}
	src.CreatedAt, err = parseTime(created)
	if err != nil {
		return model.Source{}, err
	}
	src.UpdatedAt, err = parseTime(updated)
	if err != nil {
		return model.Source{}, err
	}
	src.Schedule.Enabled = schEnabled == 1
	src.Schedule.Upload = upload == 1
	if interval.Valid {
		src.Schedule.IntervalSeconds = int(interval.Int64)
	}
	if cron.Valid {
		src.Schedule.Cron = cron.String
	}
	if timezone.Valid {
		src.Schedule.Timezone = timezone.String
	}
	src.Schedule.Retention.LocalCount = nullIntPtr(localCount)
	src.Schedule.Retention.LocalDays = nullIntPtr(localDays)
	src.Schedule.Retention.RemoteCount = nullIntPtr(remoteCount)
	src.Schedule.Retention.RemoteDays = nullIntPtr(remoteDays)
	src.Schedule.LastRunAt, err = parseTimePtr(lastRun)
	if err != nil {
		return model.Source{}, err
	}
	src.Schedule.NextRunAt, err = parseTimePtr(nextRun)
	if err != nil {
		return model.Source{}, err
	}
	return src, nil
}

func nullIntPtr(v sql.NullInt64) *int {
	if !v.Valid {
		return nil
	}
	n := int(v.Int64)
	return &n
}

func insertSchedule(ctx context.Context, tx *sql.Tx, sourceID string, sch model.Schedule, now string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO schedules(
		source_id, enabled, mode, interval_seconds, cron_expr, timezone, upload,
		local_retention_count, local_retention_days, remote_retention_count, remote_retention_days,
		last_run_at, next_run_at, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sourceID, boolInt(sch.Enabled), sch.Mode, nullableInt(zeroInt(sch.IntervalSeconds)), nullableString(sch.Cron), sch.Timezone, boolInt(sch.Upload),
		nullableInt(sch.Retention.LocalCount), nullableInt(sch.Retention.LocalDays), nullableInt(sch.Retention.RemoteCount), nullableInt(sch.Retention.RemoteDays),
		nullableTime(sch.LastRunAt), nullableTime(sch.NextRunAt), now, now)
	return err
}

func upsertSchedule(ctx context.Context, tx *sql.Tx, sourceID string, sch model.Schedule, now string) error {
	res, err := tx.ExecContext(ctx, `UPDATE schedules SET enabled = ?, mode = ?, interval_seconds = ?, cron_expr = ?, timezone = ?, upload = ?,
		local_retention_count = ?, local_retention_days = ?, remote_retention_count = ?, remote_retention_days = ?,
		next_run_at = ?, updated_at = ? WHERE source_id = ?`,
		boolInt(sch.Enabled), sch.Mode, nullableInt(zeroInt(sch.IntervalSeconds)), nullableString(sch.Cron), sch.Timezone, boolInt(sch.Upload),
		nullableInt(sch.Retention.LocalCount), nullableInt(sch.Retention.LocalDays), nullableInt(sch.Retention.RemoteCount), nullableInt(sch.Retention.RemoteDays),
		nullableTime(sch.NextRunAt), now, sourceID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return insertSchedule(ctx, tx, sourceID, sch, now)
	}
	return nil
}

func zeroInt(v int) *int {
	if v == 0 {
		return nil
	}
	return &v
}

func nullableString(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

// EffectiveRetention applies a per-source override on top of the global policy.
func EffectiveRetention(global model.Retention, override model.RetentionOverride) model.Retention {
	out := global
	if override.LocalCount != nil {
		out.LocalCount = *override.LocalCount
	}
	if override.LocalDays != nil {
		out.LocalDays = *override.LocalDays
	}
	if override.RemoteCount != nil {
		out.RemoteCount = *override.RemoteCount
	}
	if override.RemoteDays != nil {
		out.RemoteDays = *override.RemoteDays
	}
	if out.LocalCount < 0 || out.LocalDays < 0 || out.RemoteCount < 0 || out.RemoteDays < 0 {
		return global
	}
	return out
}
