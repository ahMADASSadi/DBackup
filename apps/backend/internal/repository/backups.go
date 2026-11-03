package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"dbackup/internal/model"
	"dbackup/internal/security"
)

// ErrBusy means a backup is already pending or running for the source.
var ErrBusy = errors.New("source busy")

// InsertBackupIfIdle inserts a backup when the source has no active job.
// When advance is true, the source schedule moves to next in the same transaction.
func (s *Store) InsertBackupIfIdle(ctx context.Context, b model.Backup, advance bool, next time.Time) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM backups WHERE source_id = ? AND status IN ('pending', 'running')`, b.SourceID).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return ErrBusy
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO backups(
		id, source_id, db_type, status, format, tool_version, relative_path, size_bytes, checksum_sha256,
		error_summary, trigger, upload_requested, created_at, started_at, completed_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		b.ID, b.SourceID, b.DBType, b.Status, b.Format, b.ToolVersion, b.RelativePath, b.SizeBytes, b.ChecksumSHA256,
		b.ErrorSummary, b.Trigger, boolInt(b.UploadRequested), formatTime(b.CreatedAt), nullableTime(b.StartedAt), nullableTime(b.CompletedAt)); err != nil {
		return err
	}
	if advance {
		if _, err := tx.ExecContext(ctx, `UPDATE schedules SET last_run_at = ?, next_run_at = ?, updated_at = ? WHERE source_id = ?`,
			formatTime(b.CreatedAt), formatTime(next), formatTime(time.Now()), b.SourceID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) InsertBackup(ctx context.Context, b model.Backup) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO backups(
		id, source_id, db_type, status, format, tool_version, relative_path, size_bytes, checksum_sha256,
		error_summary, trigger, upload_requested, created_at, started_at, completed_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		b.ID, b.SourceID, b.DBType, b.Status, b.Format, b.ToolVersion, b.RelativePath, b.SizeBytes, b.ChecksumSHA256,
		b.ErrorSummary, b.Trigger, boolInt(b.UploadRequested), formatTime(b.CreatedAt), nullableTime(b.StartedAt), nullableTime(b.CompletedAt))
	return err
}

func (s *Store) GetBackup(ctx context.Context, id string) (model.Backup, error) {
	row := s.DB.QueryRowContext(ctx, backupSelect()+` WHERE backups.id = ?`, id)
	return scanBackup(row)
}

func (s *Store) ListBackups(ctx context.Context, filter model.BackupFilter) (model.BackupPage, error) {
	where, args := backupWhere(filter)
	var total int
	countQuery := `SELECT COUNT(*) FROM backups JOIN sources ON sources.id = backups.source_id ` + where
	if err := s.DB.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return model.BackupPage{}, err
	}
	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	query := backupSelect() + where + ` ORDER BY backups.created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return model.BackupPage{}, err
	}
	defer rows.Close()
	page := model.BackupPage{Limit: limit, Offset: offset, Total: total, Items: []model.Backup{}}
	for rows.Next() {
		item, err := scanBackup(rows)
		if err != nil {
			return model.BackupPage{}, err
		}
		page.Items = append(page.Items, item)
	}
	return page, rows.Err()
}

func (s *Store) ListBackupsBySource(ctx context.Context, sourceID string) ([]model.Backup, error) {
	rows, err := s.DB.QueryContext(ctx, backupSelect()+` WHERE backups.source_id = ? ORDER BY backups.created_at DESC`, sourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Backup
	for rows.Next() {
		item, err := scanBackup(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) CountRunningBackups(ctx context.Context) (int, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM backups WHERE status = 'running'`).Scan(&n)
	return n, err
}

func (s *Store) SourceBusy(ctx context.Context, sourceID string) (bool, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM backups WHERE source_id = ? AND status IN ('pending', 'running')`, sourceID).Scan(&n)
	return n > 0, err
}

func (s *Store) ListPendingBackups(ctx context.Context, limit int) ([]model.Backup, error) {
	rows, err := s.DB.QueryContext(ctx, backupSelect()+` WHERE backups.status = 'pending' ORDER BY backups.created_at ASC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Backup
	for rows.Next() {
		item, err := scanBackup(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) ReleaseBackup(ctx context.Context, id string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE backups SET status = 'pending', started_at = NULL, completed_at = NULL
		WHERE id = ? AND status = 'running' AND relative_path = ''`, id)
	return err
}

func (s *Store) ClaimBackup(ctx context.Context, id string) (bool, error) {
	res, err := s.DB.ExecContext(ctx, `UPDATE backups SET status = 'running', started_at = ? WHERE id = ? AND status = 'pending'`, formatTime(time.Now()), id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

func (s *Store) CompleteBackup(ctx context.Context, id, rel, checksum, tool string, size int64) error {
	now := formatTime(time.Now())
	res, err := s.DB.ExecContext(ctx, `UPDATE backups SET status = 'completed', relative_path = ?, size_bytes = ?, checksum_sha256 = ?,
		tool_version = ?, error_summary = '', completed_at = ? WHERE id = ? AND status = 'running'`,
		rel, size, checksum, tool, now, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("backup %s was not running", id)
	}
	return nil
}

func (s *Store) FailBackup(ctx context.Context, id, summary string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE backups SET status = 'failed', error_summary = ?, completed_at = ?
		WHERE id = ? AND status IN ('pending', 'running')`, security.Truncate(summary, 500), formatTime(time.Now()), id)
	return err
}

func (s *Store) RequeueBackup(ctx context.Context, id string) (bool, error) {
	res, err := s.DB.ExecContext(ctx, `UPDATE backups SET status = 'pending', error_summary = '', started_at = NULL, completed_at = NULL
		WHERE id = ? AND status = 'failed' AND relative_path = ''`, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

func (s *Store) ReconcileInterrupted(ctx context.Context) error {
	now := formatTime(time.Now())
	summary := "interrupted by process restart"
	if _, err := s.DB.ExecContext(ctx, `UPDATE backups SET status = 'failed', error_summary = ?, completed_at = ?
		WHERE status = 'running'`, summary, now); err != nil {
		return err
	}
	_, err := s.DB.ExecContext(ctx, `UPDATE uploads SET status = 'failed', error_summary = ?, updated_at = ?
		WHERE status = 'running'`, summary, now)
	return err
}

func (s *Store) MarkBackupExpired(ctx context.Context, id string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE backups SET status = 'expired', relative_path = '', size_bytes = 0 WHERE id = ? AND status = 'completed'`, id)
	return err
}

func (s *Store) DeleteBackupRow(ctx context.Context, id string) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM backups WHERE id = ?`, id)
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

func (s *Store) CountCompleted(ctx context.Context, sourceID, exceptID string) (int, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM backups WHERE source_id = ? AND status = 'completed' AND id <> ?`, sourceID, exceptID).Scan(&n)
	return n, err
}

func (s *Store) DeleteFailedBackup(ctx context.Context, id string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM backups WHERE id = ? AND status = 'failed'`, id)
	return err
}

func (s *Store) InsertUpload(ctx context.Context, backupID, status, summary string) (string, error) {
	id, err := security.NewID()
	if err != nil {
		return "", err
	}
	now := formatTime(time.Now())
	_, err = s.DB.ExecContext(ctx, `INSERT INTO uploads(id, backup_id, status, object_key, attempts, error_summary, updated_at) VALUES (?, ?, ?, '', 0, ?, ?)`,
		id, backupID, status, summary, now)
	return id, err
}

func (s *Store) ListPendingUploads(ctx context.Context, limit int) ([]model.Upload, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, backup_id, status, object_key, attempts, error_summary, started_at, completed_at, updated_at
		FROM uploads WHERE status = 'pending' ORDER BY updated_at ASC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Upload
	for rows.Next() {
		item, err := scanUpload(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) CountActiveUploads(ctx context.Context) (int, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM uploads WHERE status = 'running'`).Scan(&n)
	return n, err
}

func (s *Store) ClaimUpload(ctx context.Context, id string) (bool, error) {
	now := formatTime(time.Now())
	res, err := s.DB.ExecContext(ctx, `UPDATE uploads SET status = 'running', started_at = ?, attempts = attempts + 1, error_summary = '', updated_at = ?
		WHERE id = ? AND status = 'pending'`, now, now, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

func (s *Store) CompleteUpload(ctx context.Context, id, objectKey string) error {
	now := formatTime(time.Now())
	_, err := s.DB.ExecContext(ctx, `UPDATE uploads SET status = 'completed', object_key = ?, error_summary = '', completed_at = ?, updated_at = ? WHERE id = ?`,
		objectKey, now, now, id)
	return err
}

func (s *Store) FailUpload(ctx context.Context, id, summary string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE uploads SET status = 'failed', error_summary = ?, updated_at = ? WHERE id = ? AND status IN ('pending', 'running')`,
		security.Truncate(summary, 500), formatTime(time.Now()), id)
	return err
}

func (s *Store) RequeueUpload(ctx context.Context, backupID string) (bool, error) {
	res, err := s.DB.ExecContext(ctx, `UPDATE uploads SET status = 'pending', error_summary = '', updated_at = ? WHERE backup_id = ? AND status = 'failed'`,
		formatTime(time.Now()), backupID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

func (s *Store) ExpireUpload(ctx context.Context, id string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE uploads SET status = 'expired', updated_at = ? WHERE id = ? AND status = 'completed'`, formatTime(time.Now()), id)
	return err
}

func (s *Store) RecentBackups(ctx context.Context, limit int) ([]model.Backup, error) {
	rows, err := s.DB.QueryContext(ctx, backupSelect()+` ORDER BY backups.created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Backup
	for rows.Next() {
		item, err := scanBackup(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) CountByStatus(ctx context.Context, status string) (int, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM backups WHERE status = ?`, status).Scan(&n)
	return n, err
}

func (s *Store) CountUploadsByStatus(ctx context.Context, statuses ...string) (int, error) {
	if len(statuses) == 0 {
		return 0, nil
	}
	placeholders := make([]string, len(statuses))
	args := make([]any, len(statuses))
	for i, status := range statuses {
		placeholders[i] = "?"
		args[i] = status
	}
	query := `SELECT COUNT(*) FROM uploads WHERE status IN (` + strings.Join(placeholders, ",") + `)`
	var n int
	err := s.DB.QueryRowContext(ctx, query, args...).Scan(&n)
	return n, err
}

// LastSuccess is the newest completed backup for a source, if one exists.
type LastSuccess struct {
	SourceID   string
	SourceName string
	BackupID   string
	At         *time.Time
}

func (s *Store) LastSuccesses(ctx context.Context) ([]LastSuccess, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT sources.id, sources.name,
		(SELECT backups.id FROM backups WHERE backups.source_id = sources.id AND backups.status = 'completed' ORDER BY backups.completed_at DESC LIMIT 1),
		(SELECT backups.completed_at FROM backups WHERE backups.source_id = sources.id AND backups.status = 'completed' ORDER BY backups.completed_at DESC LIMIT 1)
		FROM sources ORDER BY sources.name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LastSuccess
	for rows.Next() {
		var item LastSuccess
		var id, at sql.NullString
		if err := rows.Scan(&item.SourceID, &item.SourceName, &id, &at); err != nil {
			return nil, err
		}
		if id.Valid {
			item.BackupID = id.String
		}
		item.At, err = parseTimePtr(at)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// NextRun is the next scheduled backup for a source.
type NextRun struct {
	SourceID   string
	SourceName string
	At         time.Time
}

func (s *Store) NextRuns(ctx context.Context) ([]NextRun, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT sources.id, sources.name, schedules.next_run_at
		FROM schedules JOIN sources ON sources.id = schedules.source_id
		WHERE schedules.enabled = 1 AND sources.enabled = 1 AND schedules.next_run_at IS NOT NULL
		ORDER BY schedules.next_run_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NextRun
	for rows.Next() {
		var item NextRun
		var at string
		if err := rows.Scan(&item.SourceID, &item.SourceName, &at); err != nil {
			return nil, err
		}
		item.At, err = parseTime(at)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func backupSelect() string {
	return `SELECT backups.id, backups.source_id, sources.name, backups.db_type, backups.status, backups.format, backups.tool_version,
		backups.relative_path, backups.size_bytes, backups.checksum_sha256, backups.error_summary, backups.trigger, backups.upload_requested,
		backups.created_at, backups.started_at, backups.completed_at,
		uploads.id, uploads.status, uploads.object_key, uploads.attempts, uploads.error_summary, uploads.started_at, uploads.completed_at, uploads.updated_at
		FROM backups
		JOIN sources ON sources.id = backups.source_id
		LEFT JOIN uploads ON uploads.backup_id = backups.id `
}

func backupWhere(filter model.BackupFilter) (string, []any) {
	var clauses []string
	var args []any
	if filter.SourceID != "" {
		clauses = append(clauses, "backups.source_id = ?")
		args = append(args, filter.SourceID)
	}
	if filter.Status != "" {
		clauses = append(clauses, "backups.status = ?")
		args = append(args, filter.Status)
	}
	if filter.From != nil {
		clauses = append(clauses, "backups.created_at >= ?")
		args = append(args, formatTime(*filter.From))
	}
	if filter.To != nil {
		clauses = append(clauses, "backups.created_at <= ?")
		args = append(args, formatTime(*filter.To))
	}
	if len(clauses) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(clauses, " AND ") + " ", args
}

func scanBackup(row scanner) (model.Backup, error) {
	var b model.Backup
	var uploadRequested int
	var created string
	var started, completed sql.NullString
	var uploadID, uploadStatus, objectKey, uploadErr sql.NullString
	var attempts sql.NullInt64
	var uploadStarted, uploadCompleted, uploadUpdated sql.NullString
	err := row.Scan(
		&b.ID, &b.SourceID, &b.SourceName, &b.DBType, &b.Status, &b.Format, &b.ToolVersion,
		&b.RelativePath, &b.SizeBytes, &b.ChecksumSHA256, &b.ErrorSummary, &b.Trigger, &uploadRequested,
		&created, &started, &completed,
		&uploadID, &uploadStatus, &objectKey, &attempts, &uploadErr, &uploadStarted, &uploadCompleted, &uploadUpdated,
	)
	if err != nil {
		return model.Backup{}, err
	}
	b.UploadRequested = uploadRequested == 1
	b.CreatedAt, err = parseTime(created)
	if err != nil {
		return model.Backup{}, err
	}
	b.StartedAt, err = parseTimePtr(started)
	if err != nil {
		return model.Backup{}, err
	}
	b.CompletedAt, err = parseTimePtr(completed)
	if err != nil {
		return model.Backup{}, err
	}
	if uploadID.Valid {
		upload := model.Upload{
			ID:        uploadID.String,
			BackupID:  b.ID,
			Status:    uploadStatus.String,
			ObjectKey: objectKey.String,
			Attempts:  int(attempts.Int64),
		}
		if uploadErr.Valid {
			upload.ErrorSummary = uploadErr.String
		}
		upload.StartedAt, err = parseTimePtr(uploadStarted)
		if err != nil {
			return model.Backup{}, err
		}
		upload.CompletedAt, err = parseTimePtr(uploadCompleted)
		if err != nil {
			return model.Backup{}, err
		}
		if uploadUpdated.Valid {
			upload.UpdatedAt, err = parseTime(uploadUpdated.String)
			if err != nil {
				return model.Backup{}, err
			}
		}
		b.Upload = &upload
	}
	return b, nil
}

func scanUpload(row scanner) (model.Upload, error) {
	var u model.Upload
	var started, completed sql.NullString
	var updated string
	err := row.Scan(&u.ID, &u.BackupID, &u.Status, &u.ObjectKey, &u.Attempts, &u.ErrorSummary, &started, &completed, &updated)
	if err != nil {
		return model.Upload{}, err
	}
	u.StartedAt, err = parseTimePtr(started)
	if err != nil {
		return model.Upload{}, err
	}
	u.CompletedAt, err = parseTimePtr(completed)
	if err != nil {
		return model.Upload{}, err
	}
	u.UpdatedAt, err = parseTime(updated)
	if err != nil {
		return model.Upload{}, err
	}
	return u, nil
}
