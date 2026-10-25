package service

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"dbackup/internal/backup"
	"dbackup/internal/model"
	"dbackup/internal/repository"
	"dbackup/internal/scheduler"
	"dbackup/internal/security"
	"dbackup/internal/storage"
)

// SourceInput is the administrator's source form.
type SourceInput struct {
	Name     string           `json:"name"`
	Type     string           `json:"type"`
	Enabled  *bool            `json:"enabled"`
	Config   model.ConnConfig `json:"config"`
	Password string           `json:"password"`
	Schedule model.Schedule   `json:"schedule"`
}

// Catalog manages database sources.
type Catalog struct {
	Store         *repository.Store
	Engine        *backup.Engine
	Cipher        *security.Cipher
	Files         *storage.Local
	Objects       storage.S3
	AllowInsecure bool
	BackupDir     string
	MetaPath      string
}

func (c *Catalog) List(ctx context.Context) ([]model.Source, error) {
	items, err := c.Store.ListSources(ctx)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.Source{}
	}
	for i := range items {
		items[i].PasswordCipher = ""
	}
	return items, nil
}

func (c *Catalog) Get(ctx context.Context, id string) (model.Source, error) {
	if !security.ValidID(id) {
		return model.Source{}, E(400, "validation_error", "invalid source id")
	}
	src, err := c.Store.GetSource(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Source{}, E(404, "not_found", "source not found")
	}
	if err != nil {
		return model.Source{}, err
	}
	src.PasswordCipher = ""
	return src, nil
}

func (c *Catalog) Create(ctx context.Context, in SourceInput, actor string) (model.Source, error) {
	src, err := c.normalize(ctx, in, model.Source{}, true)
	if err != nil {
		return model.Source{}, err
	}
	id, err := security.NewID()
	if err != nil {
		return model.Source{}, err
	}
	src.ID = id
	if src.Type != model.TypeSQLite && in.Password == "" {
		return model.Source{}, E(400, "validation_error", "password is required")
	}
	if in.Password != "" {
		sealed, err := c.Cipher.Encrypt("source:"+id, in.Password)
		if err != nil {
			return model.Source{}, err
		}
		src.PasswordCipher = sealed
	}
	if err := c.Store.CreateSource(ctx, src); err != nil {
		return model.Source{}, err
	}
	_ = c.Store.InsertAudit(ctx, actor, "source.create", "source", id, "{}")
	return c.Get(ctx, id)
}

func (c *Catalog) Update(ctx context.Context, id string, in SourceInput, actor string) (model.Source, error) {
	if !security.ValidID(id) {
		return model.Source{}, E(400, "validation_error", "invalid source id")
	}
	current, err := c.Store.GetSource(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Source{}, E(404, "not_found", "source not found")
	}
	if err != nil {
		return model.Source{}, err
	}
	if in.Type != "" && in.Type != current.Type {
		return model.Source{}, E(400, "validation_error", "database type cannot be changed")
	}
	in.Type = current.Type
	src, err := c.normalize(ctx, in, current, false)
	if err != nil {
		return model.Source{}, err
	}
	src.ID = id
	src.PasswordCipher = current.PasswordCipher
	if in.Password != "" {
		sealed, err := c.Cipher.Encrypt("source:"+id, in.Password)
		if err != nil {
			return model.Source{}, err
		}
		src.PasswordCipher = sealed
	}
	if src.Type != model.TypeSQLite && src.PasswordCipher == "" {
		return model.Source{}, E(400, "validation_error", "password is required")
	}
	if src.Type == model.TypeSQLite {
		src.PasswordCipher = ""
	}
	if err := c.Store.UpdateSource(ctx, src); err != nil {
		return model.Source{}, err
	}
	_ = c.Store.InsertAudit(ctx, actor, "source.update", "source", id, "{}")
	return c.Get(ctx, id)
}

func (c *Catalog) Delete(ctx context.Context, id string, deleteBackups bool, actor string) error {
	if !security.ValidID(id) {
		return E(400, "validation_error", "invalid source id")
	}
	if _, err := c.Store.GetSource(ctx, id); errors.Is(err, sql.ErrNoRows) {
		return E(404, "not_found", "source not found")
	} else if err != nil {
		return err
	}
	count, err := c.Store.CountBackups(ctx, id)
	if err != nil {
		return err
	}
	if count > 0 && !deleteBackups {
		return E(409, "backups_exist", "source still has backups; confirm deletion of its backups")
	}
	if deleteBackups {
		items, err := c.Store.ListBackupsBySource(ctx, id)
		if err != nil {
			return err
		}
		cfg, configured, err := loadS3(ctx, c.Store, c.Cipher, c.AllowInsecure)
		if err != nil {
			return err
		}
		for _, item := range items {
			if configured && item.Upload != nil && item.Upload.ObjectKey != "" && item.Upload.Status == model.StatusCompleted {
				if err := c.Objects.Delete(ctx, cfg, item.Upload.ObjectKey); err != nil {
					return E(502, "upload_delete_failed", "a remote object could not be deleted")
				}
			}
			if item.RelativePath != "" {
				if err := c.Files.Remove(item.RelativePath); err != nil {
					return E(500, "delete_failed", "a backup file could not be deleted")
				}
			}
		}
	}
	if err := c.Store.DeleteSource(ctx, id); err != nil {
		return err
	}
	_ = c.Store.InsertAudit(ctx, actor, "source.delete", "source", id, "{}")
	return nil
}

func (c *Catalog) Test(ctx context.Context, id string) error {
	src, err := c.loadSecret(ctx, id)
	if err != nil {
		return err
	}
	password, err := c.password(src)
	if err != nil {
		return err
	}
	if err := c.Engine.Ping(ctx, src, password); err != nil {
		return E(422, "connection_failed", security.Truncate(security.Redact(err.Error(), password), 300))
	}
	return nil
}

func (c *Catalog) loadSecret(ctx context.Context, id string) (model.Source, error) {
	if !security.ValidID(id) {
		return model.Source{}, E(400, "validation_error", "invalid source id")
	}
	src, err := c.Store.GetSource(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Source{}, E(404, "not_found", "source not found")
	}
	return src, err
}

func (c *Catalog) password(src model.Source) (string, error) {
	if src.PasswordCipher == "" {
		return "", nil
	}
	password, err := c.Cipher.Decrypt("source:"+src.ID, src.PasswordCipher)
	if err != nil {
		return "", E(500, "credential_error", "stored credential could not be read")
	}
	return password, nil
}

func (c *Catalog) normalize(ctx context.Context, in SourceInput, current model.Source, create bool) (model.Source, error) {
	name := strings.TrimSpace(in.Name)
	if err := security.ValidateSourceName(name); err != nil {
		return model.Source{}, E(400, "validation_error", err.Error())
	}
	switch in.Type {
	case model.TypePostgres, model.TypeMySQL, model.TypeSQLite:
	default:
		return model.Source{}, E(400, "validation_error", "type must be postgres, mysql, or sqlite")
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	} else if !create {
		enabled = current.Enabled
	}
	cfg := in.Config
	if strings.ContainsAny(in.Password, "\r\n\x00") || len(in.Password) > 200 {
		return model.Source{}, E(400, "validation_error", "password contains unsupported characters")
	}
	switch in.Type {
	case model.TypeSQLite:
		resolved, err := security.ValidateSQLitePath(cfg.Path, c.Engine.Roots)
		if err != nil {
			return model.Source{}, E(400, "validation_error", err.Error())
		}
		if c.MetaPath != "" && resolved == filepath.Clean(c.MetaPath) {
			return model.Source{}, E(400, "validation_error", "the metadata database cannot be configured as a source")
		}
		if c.BackupDir != "" && security.Within(c.BackupDir, resolved) {
			return model.Source{}, E(400, "validation_error", "sqlite path must not be inside the backup directory")
		}
		cfg = model.ConnConfig{Path: resolved}
	default:
		if cfg.Port == 0 {
			if in.Type == model.TypePostgres {
				cfg.Port = 5432
			} else {
				cfg.Port = 3306
			}
		}
		if err := security.ValidatePort(cfg.Port); err != nil {
			return model.Source{}, E(400, "validation_error", err.Error())
		}
		if err := security.ValidateIdent(cfg.Database, "database"); err != nil {
			return model.Source{}, E(400, "validation_error", err.Error())
		}
		if err := security.ValidateUsername(cfg.Username); err != nil {
			return model.Source{}, E(400, "validation_error", err.Error())
		}
		mode, err := security.ValidateTLSMode(cfg.TLSMode)
		if err != nil {
			return model.Source{}, E(400, "validation_error", err.Error())
		}
		cfg.TLSMode = mode
		if err := security.ValidateCAPem(cfg.TLSCAPem); err != nil {
			return model.Source{}, E(400, "validation_error", err.Error())
		}
		if mode == "verify-ca" && strings.TrimSpace(cfg.TLSCAPem) == "" {
			return model.Source{}, E(400, "validation_error", "tls ca bundle is required for verify-ca")
		}
		if err := security.ValidateParams(in.Type, cfg.Params); err != nil {
			return model.Source{}, E(400, "validation_error", err.Error())
		}
		if err := security.ResolveHost(ctx, c.Engine.Resolver, cfg.Host, c.Engine.Allowlist); err != nil {
			return model.Source{}, E(400, "validation_error", err.Error())
		}
		cfg.Path = ""
	}
	schedule, err := normalizeSchedule(in.Schedule)
	if err != nil {
		return model.Source{}, err
	}
	return model.Source{
		Name:     name,
		Type:     in.Type,
		Enabled:  enabled,
		Config:   cfg,
		Schedule: schedule,
	}, nil
}

func normalizeSchedule(in model.Schedule) (model.Schedule, error) {
	if in.Mode == "" {
		in.Mode = model.ModeCron
	}
	if in.Timezone == "" {
		in.Timezone = "UTC"
	}
	if in.Mode == model.ModeCron && strings.TrimSpace(in.Cron) == "" {
		in.Cron = "0 2 * * *"
	}
	if _, err := scheduler.NextRun(in.Mode, in.IntervalSeconds, in.Cron, in.Timezone, time.Now()); err != nil {
		return model.Schedule{}, E(400, "validation_error", err.Error())
	}
	for _, item := range []struct {
		name string
		val  *int
	}{
		{"local_count", in.Retention.LocalCount},
		{"local_days", in.Retention.LocalDays},
		{"remote_count", in.Retention.RemoteCount},
		{"remote_days", in.Retention.RemoteDays},
	} {
		if item.val != nil && (*item.val < 0 || *item.val > 10000) {
			return model.Schedule{}, E(400, "validation_error", item.name+" must be between 0 and 10000")
		}
	}
	if in.Enabled {
		next, err := scheduler.NextRun(in.Mode, in.IntervalSeconds, in.Cron, in.Timezone, time.Now())
		if err != nil {
			return model.Schedule{}, E(400, "validation_error", err.Error())
		}
		in.NextRunAt = &next
	} else {
		in.NextRunAt = nil
	}
	in.LastRunAt = nil
	return in, nil
}
