package service

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"dbackup/internal/backup"
	"dbackup/internal/model"
	"dbackup/internal/repository"
	"dbackup/internal/scheduler"
	"dbackup/internal/security"
	"dbackup/internal/storage"
)

// Jobs runs backup and upload work outside the HTTP request.
type Jobs struct {
	Store         *repository.Store
	Engine        *backup.Engine
	Files         *storage.Local
	Objects       storage.S3
	Cipher        *security.Cipher
	Log           *slog.Logger
	HardLimit     int
	Timeout       time.Duration
	AllowInsecure bool

	wake   chan struct{}
	root   context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewJobs builds the worker. Call Start before serving traffic.
func NewJobs(store *repository.Store, engine *backup.Engine, files *storage.Local, objects storage.S3, cipher *security.Cipher, log *slog.Logger, hardLimit int, timeout time.Duration, allowInsecure bool) *Jobs {
	if log == nil {
		log = slog.Default()
	}
	if hardLimit < 1 {
		hardLimit = 1
	}
	if timeout < time.Minute {
		timeout = 2 * time.Hour
	}
	return &Jobs{
		Store: store, Engine: engine, Files: files, Objects: objects, Cipher: cipher, Log: log,
		HardLimit: hardLimit, Timeout: timeout, AllowInsecure: allowInsecure,
		wake: make(chan struct{}, 1),
	}
}

// Start launches the dispatcher and scheduler until ctx is cancelled.
func (j *Jobs) Start(ctx context.Context) {
	j.root, j.cancel = context.WithCancel(ctx)
	go j.loop(j.root)
	go j.scheduleLoop(j.root)
	j.Poke()
}

// Stop cancels workers and waits for active jobs to notice.
func (j *Jobs) Stop() {
	if j.cancel != nil {
		j.cancel()
	}
	done := make(chan struct{})
	go func() {
		j.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		j.Log.Warn("backup workers did not finish before shutdown")
	}
}

// Poke asks the dispatcher to look for pending work.
func (j *Jobs) Poke() {
	select {
	case j.wake <- struct{}{}:
	default:
	}
}

// Enqueue records a backup and returns immediately.
func (j *Jobs) Enqueue(ctx context.Context, sourceID, trigger string, upload *bool, actor string) (model.Backup, error) {
	if trigger != model.TriggerManual && trigger != model.TriggerSchedule {
		return model.Backup{}, E(400, "validation_error", "invalid trigger")
	}
	if !security.ValidID(sourceID) {
		return model.Backup{}, E(400, "validation_error", "invalid source id")
	}
	src, err := j.Store.GetSource(ctx, sourceID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Backup{}, E(404, "not_found", "source not found")
	}
	if err != nil {
		return model.Backup{}, err
	}
	if !src.Enabled {
		return model.Backup{}, E(409, "disabled", "source is disabled")
	}
	wantUpload := src.Schedule.Upload
	if upload != nil {
		wantUpload = *upload
	}
	id, err := security.NewID()
	if err != nil {
		return model.Backup{}, err
	}
	now := time.Now().UTC()
	item := model.Backup{
		ID:              id,
		SourceID:        src.ID,
		DBType:          src.Type,
		Status:          model.StatusPending,
		Format:          model.FormatFor(src.Type),
		Trigger:         trigger,
		UploadRequested: wantUpload,
		CreatedAt:       now,
	}
	advance := trigger == model.TriggerSchedule
	var next time.Time
	if advance {
		next, err = scheduler.NextRun(src.Schedule.Mode, src.Schedule.IntervalSeconds, src.Schedule.Cron, src.Schedule.Timezone, now)
		if err != nil {
			return model.Backup{}, E(400, "validation_error", err.Error())
		}
	}
	if err := j.Store.InsertBackupIfIdle(ctx, item, advance, next); err != nil {
		if errors.Is(err, repository.ErrBusy) {
			return model.Backup{}, E(409, "busy", "a backup is already active for this source")
		}
		return model.Backup{}, err
	}
	if actor == "" {
		actor = "scheduler"
	}
	_ = j.Store.InsertAudit(ctx, actor, "backup.create", "backup", id, "{}")
	j.Poke()
	return j.mustGet(ctx, id)
}

// Retry re-queues a failed backup that never published a file.
func (j *Jobs) Retry(ctx context.Context, id, actor string) (model.Backup, error) {
	item, err := j.mustGet(ctx, id)
	if err != nil {
		return model.Backup{}, err
	}
	if item.Status != model.StatusFailed || item.RelativePath != "" {
		return model.Backup{}, E(409, "not_retryable", "only a failed backup without a published file can be retried")
	}
	src, err := j.Store.GetSource(ctx, item.SourceID)
	if err != nil {
		return model.Backup{}, err
	}
	if !src.Enabled {
		return model.Backup{}, E(409, "disabled", "source is disabled")
	}
	ok, err := j.Store.RequeueBackup(ctx, id)
	if err != nil {
		return model.Backup{}, err
	}
	if !ok {
		return model.Backup{}, E(409, "not_retryable", "backup could not be retried")
	}
	_ = j.Store.InsertAudit(ctx, actor, "backup.retry", "backup", id, "{}")
	j.Poke()
	return j.mustGet(ctx, id)
}

// RetryUpload re-queues a failed upload for a completed backup.
func (j *Jobs) RetryUpload(ctx context.Context, id, actor string) (model.Backup, error) {
	item, err := j.mustGet(ctx, id)
	if err != nil {
		return model.Backup{}, err
	}
	if item.Status != model.StatusCompleted || item.RelativePath == "" {
		return model.Backup{}, E(409, "not_retryable", "only a completed backup can be uploaded")
	}
	_, configured, err := loadS3(ctx, j.Store, j.Cipher, j.AllowInsecure)
	if err != nil {
		return model.Backup{}, err
	}
	if !configured {
		return model.Backup{}, E(409, "s3_unconfigured", "object storage is not configured")
	}
	if item.Upload == nil {
		if _, err := j.Store.InsertUpload(ctx, id, model.StatusPending, ""); err != nil {
			return model.Backup{}, err
		}
	} else {
		ok, err := j.Store.RequeueUpload(ctx, id)
		if err != nil {
			return model.Backup{}, err
		}
		if !ok {
			return model.Backup{}, E(409, "not_retryable", "this upload cannot be retried")
		}
	}
	_ = j.Store.InsertAudit(ctx, actor, "backup.retry_upload", "backup", id, "{}")
	j.Poke()
	return j.mustGet(ctx, id)
}

// Delete removes a backup, refusing to delete the only completed copy unless force is set.
func (j *Jobs) Delete(ctx context.Context, id string, force bool, actor string) error {
	item, err := j.mustGet(ctx, id)
	if err != nil {
		return err
	}
	if item.Status == model.StatusPending || item.Status == model.StatusRunning {
		return E(409, "busy", "a running backup cannot be deleted")
	}
	if item.Status == model.StatusCompleted {
		others, err := j.Store.CountCompleted(ctx, item.SourceID, item.ID)
		if err != nil {
			return err
		}
		if others == 0 && !force {
			return E(409, "last_backup", "refusing to delete the only completed backup")
		}
	}
	if item.Upload != nil && item.Upload.ObjectKey != "" && (item.Upload.Status == model.StatusCompleted || item.Upload.Status == model.StatusExpired) {
		cfg, configured, err := loadS3(ctx, j.Store, j.Cipher, j.AllowInsecure)
		if err != nil {
			return err
		}
		if configured && item.Upload.Status == model.StatusCompleted {
			if err := j.Objects.Delete(ctx, cfg, item.Upload.ObjectKey); err != nil {
				return E(502, "upload_delete_failed", "the remote object could not be deleted")
			}
		}
	}
	if item.RelativePath != "" {
		if err := j.Files.Remove(item.RelativePath); err != nil {
			return E(500, "delete_failed", "the backup file could not be deleted")
		}
	}
	if err := j.Store.DeleteBackupRow(ctx, id); err != nil {
		return err
	}
	_ = j.Store.InsertAudit(ctx, actor, "backup.delete", "backup", id, "{}")
	return nil
}

// Download is an open backup file. The caller closes File.
type Download struct {
	File io.ReadSeekCloser
	Info os.FileInfo
	Name string
}

// Open returns a completed backup file for download.
func (j *Jobs) Open(ctx context.Context, id string) (Download, error) {
	item, err := j.mustGet(ctx, id)
	if err != nil {
		return Download{}, err
	}
	if item.Status != model.StatusCompleted || item.RelativePath == "" {
		return Download{}, E(404, "not_found", "backup file is not available")
	}
	file, info, err := j.Files.Open(item.RelativePath)
	if err != nil {
		return Download{}, E(404, "not_found", "backup file is not available")
	}
	return Download{File: file, Info: info, Name: item.ID + model.Extension(item.DBType)}, nil
}

func (j *Jobs) mustGet(ctx context.Context, id string) (model.Backup, error) {
	if !security.ValidID(id) {
		return model.Backup{}, E(400, "validation_error", "invalid backup id")
	}
	item, err := j.Store.GetBackup(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Backup{}, E(404, "not_found", "backup not found")
	}
	return item, err
}

// Get returns one backup.
func (j *Jobs) Get(ctx context.Context, id string) (model.Backup, error) {
	return j.mustGet(ctx, id)
}

// List returns a page of backups.
func (j *Jobs) List(ctx context.Context, filter model.BackupFilter) (model.BackupPage, error) {
	if filter.SourceID != "" && !security.ValidID(filter.SourceID) {
		return model.BackupPage{}, E(400, "validation_error", "invalid source id")
	}
	switch filter.Status {
	case "", model.StatusPending, model.StatusRunning, model.StatusCompleted, model.StatusFailed, model.StatusExpired:
	default:
		return model.BackupPage{}, E(400, "validation_error", "invalid status")
	}
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 50
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	page, err := j.Store.ListBackups(ctx, filter)
	if err != nil {
		return model.BackupPage{}, err
	}
	if page.Items == nil {
		page.Items = []model.Backup{}
	}
	return page, nil
}
