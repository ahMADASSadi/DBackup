// Package repository persists control-plane metadata in SQLite.
package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"dbackup/internal/model"
	"dbackup/internal/security"
)

const timeLayout = "2006-01-02T15:04:05.000000000Z"

// Store is the metadata repository.
type Store struct {
	DB *sql.DB
}

func New(db *sql.DB) *Store { return &Store{DB: db} }

func formatTime(t time.Time) string { return t.UTC().Format(timeLayout) }

func parseTime(value string) (time.Time, error) {
	t, err := time.Parse(timeLayout, value)
	if err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339Nano, value)
}

func parseTimePtr(value sql.NullString) (*time.Time, error) {
	if !value.Valid || value.String == "" {
		return nil, nil
	}
	t, err := parseTime(value.String)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func nullableTime(t *time.Time) any {
	if t == nil || t.IsZero() {
		return nil
	}
	return formatTime(*t)
}

func nullableInt(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

type Admin struct {
	ID           string
	Username     string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (s *Store) GetAdmin(ctx context.Context) (Admin, error) {
	var admin Admin
	var created, updated string
	err := s.DB.QueryRowContext(ctx, `SELECT id, username, password_hash, created_at, updated_at FROM admin_user LIMIT 1`).
		Scan(&admin.ID, &admin.Username, &admin.PasswordHash, &created, &updated)
	if err != nil {
		return Admin{}, err
	}
	admin.CreatedAt, err = parseTime(created)
	if err != nil {
		return Admin{}, err
	}
	admin.UpdatedAt, err = parseTime(updated)
	if err != nil {
		return Admin{}, err
	}
	return admin, nil
}

func (s *Store) CreateAdmin(ctx context.Context, username, hash string) error {
	id, err := security.NewID()
	if err != nil {
		return err
	}
	now := formatTime(time.Now())
	_, err = s.DB.ExecContext(ctx, `INSERT INTO admin_user(id, username, password_hash, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		id, username, hash, now, now)
	return err
}

func (s *Store) UpdateAdminPassword(ctx context.Context, hash string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE admin_user SET password_hash = ?, updated_at = ?`, hash, formatTime(time.Now()))
	return err
}

type Session struct {
	ID         string
	TokenHash  string
	CSRFSecret string
	CreatedAt  time.Time
	ExpiresAt  time.Time
}

func (s *Store) CreateSession(ctx context.Context, tokenHash, csrf string, expires time.Time) (string, error) {
	id, err := security.NewID()
	if err != nil {
		return "", err
	}
	_, err = s.DB.ExecContext(ctx, `INSERT INTO sessions(id, token_hash, csrf_secret, created_at, expires_at) VALUES (?, ?, ?, ?, ?)`,
		id, tokenHash, csrf, formatTime(time.Now()), formatTime(expires))
	if err != nil {
		return "", err
	}
	return id, nil
}

func (s *Store) GetSession(ctx context.Context, tokenHash string) (Session, error) {
	var session Session
	var created, expires string
	err := s.DB.QueryRowContext(ctx, `SELECT id, token_hash, csrf_secret, created_at, expires_at FROM sessions WHERE token_hash = ?`, tokenHash).
		Scan(&session.ID, &session.TokenHash, &session.CSRFSecret, &created, &expires)
	if err != nil {
		return Session{}, err
	}
	session.CreatedAt, err = parseTime(created)
	if err != nil {
		return Session{}, err
	}
	session.ExpiresAt, err = parseTime(expires)
	if err != nil {
		return Session{}, err
	}
	return session, nil
}

func (s *Store) DeleteSession(ctx context.Context, id string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id)
	return err
}

func (s *Store) DeleteAllSessions(ctx context.Context) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM sessions`)
	return err
}

func (s *Store) CountLiveSessions(ctx context.Context, now time.Time) (int, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions WHERE expires_at > ?`, formatTime(now)).Scan(&n)
	return n, err
}

func (s *Store) CountFailedLogins(ctx context.Context, ip, username string, since time.Time) (ipCount, userCount int, err error) {
	sinceText := formatTime(since)
	if err = s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM login_attempts WHERE ip = ? AND success = 0 AND attempted_at >= ?`, ip, sinceText).Scan(&ipCount); err != nil {
		return 0, 0, err
	}
	if err = s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM login_attempts WHERE username = ? AND success = 0 AND attempted_at >= ?`, username, sinceText).Scan(&userCount); err != nil {
		return 0, 0, err
	}
	return ipCount, userCount, nil
}

func (s *Store) InsertLoginAttempt(ctx context.Context, ip, username string, success bool) error {
	flag := 0
	if success {
		flag = 1
	}
	_, err := s.DB.ExecContext(ctx, `INSERT INTO login_attempts(ip, username, success, attempted_at) VALUES (?, ?, ?, ?)`,
		ip, username, flag, formatTime(time.Now()))
	return err
}

func (s *Store) PruneLoginAttempts(ctx context.Context, before time.Time) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM login_attempts WHERE attempted_at < ?`, formatTime(before))
	return err
}

func (s *Store) InsertAudit(ctx context.Context, actor, action, targetType, targetID, metadata string) error {
	if metadata == "" {
		metadata = "{}"
	}
	id, err := security.NewID()
	if err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, `INSERT INTO audit_events(id, actor, action, target_type, target_id, metadata_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, actor, action, targetType, targetID, metadata, formatTime(time.Now()))
	return err
}

func (s *Store) RecentAudit(ctx context.Context, limit int) ([]model.AuditEvent, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT id, actor, action, target_type, target_id, created_at FROM audit_events ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.AuditEvent
	for rows.Next() {
		var event model.AuditEvent
		var created string
		if err := rows.Scan(&event.ID, &event.Actor, &event.Action, &event.TargetType, &event.TargetID, &created); err != nil {
			return nil, err
		}
		event.CreatedAt, err = parseTime(created)
		if err != nil {
			return nil, err
		}
		out = append(out, event)
	}
	return out, rows.Err()
}

func marshalConfig(cfg model.ConnConfig) (string, error) {
	if cfg.Params == nil {
		cfg.Params = map[string]string{}
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func unmarshalConfig(raw string) (model.ConnConfig, error) {
	var cfg model.ConnConfig
	if raw == "" {
		return cfg, nil
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return model.ConnConfig{}, fmt.Errorf("decode source config")
	}
	return cfg, nil
}
