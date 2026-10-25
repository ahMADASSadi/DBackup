package service

import (
	"context"
	"time"

	"dbackup/internal/model"
	"dbackup/internal/security"
	"dbackup/internal/storage"
)

func (j *Jobs) loop(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		j.dispatch(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-j.wake:
		}
	}
}

func (j *Jobs) scheduleLoop(ctx context.Context) {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	j.scheduleTick(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			j.scheduleTick(ctx)
		}
	}
}

func (j *Jobs) scheduleTick(ctx context.Context) {
	due, err := j.Store.ListDueSchedules(ctx, time.Now().UTC())
	if err != nil {
		j.Log.Error("list schedules", "error", err.Error())
		return
	}
	for _, src := range due {
		if _, err := j.Enqueue(ctx, src.ID, model.TriggerSchedule, nil, "scheduler"); err != nil {
			if api, ok := As(err); ok && (api.Code == "busy" || api.Code == "disabled") {
				j.Log.Info("skipped scheduled backup", "source_id", src.ID, "reason", api.Code)
				continue
			}
			j.Log.Error("schedule backup", "source_id", src.ID, "error", err.Error())
		}
	}
	if err := j.ApplyRetention(ctx); err != nil {
		j.Log.Error("retention", "error", err.Error())
	}
}

func (j *Jobs) dispatch(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	limit := j.limit(ctx)
	runningBackups, err := j.Store.CountRunningBackups(ctx)
	if err != nil {
		j.Log.Error("count backups", "error", err.Error())
		return
	}
	runningUploads, err := j.Store.CountActiveUploads(ctx)
	if err != nil {
		j.Log.Error("count uploads", "error", err.Error())
		return
	}
	active := runningBackups + runningUploads
	if active < limit {
		pending, err := j.Store.ListPendingBackups(ctx, 20)
		if err != nil {
			j.Log.Error("list pending backups", "error", err.Error())
			return
		}
		seen := map[string]bool{}
		for _, item := range pending {
			if active >= limit {
				break
			}
			if seen[item.SourceID] {
				continue
			}
			runningForSource, err := j.sourceRunning(ctx, item.SourceID)
			if err != nil {
				j.Log.Error("source running", "error", err.Error())
				return
			}
			if runningForSource {
				seen[item.SourceID] = true
				continue
			}
			ok, err := j.Store.ClaimBackup(ctx, item.ID)
			if err != nil || !ok {
				continue
			}
			seen[item.SourceID] = true
			active++
			j.wg.Add(1)
			go j.executeBackup(item.ID)
		}
	}
	if active >= limit {
		return
	}
	uploads, err := j.Store.ListPendingUploads(ctx, 20)
	if err != nil {
		j.Log.Error("list uploads", "error", err.Error())
		return
	}
	for _, upload := range uploads {
		if active >= limit {
			return
		}
		ok, err := j.Store.ClaimUpload(ctx, upload.ID)
		if err != nil || !ok {
			continue
		}
		active++
		j.wg.Add(1)
		go j.executeUpload(upload.ID, upload.BackupID)
	}
}

func (j *Jobs) sourceRunning(ctx context.Context, sourceID string) (bool, error) {
	n, err := j.Store.CountRunningBackups(ctx)
	if err != nil {
		return false, err
	}
	if n == 0 {
		return false, nil
	}
	items, err := j.Store.ListBackupsBySource(ctx, sourceID)
	if err != nil {
		return false, err
	}
	for _, item := range items {
		if item.Status == model.StatusRunning {
			return true, nil
		}
	}
	return false, nil
}

func (j *Jobs) limit(ctx context.Context) int {
	limit := j.HardLimit
	settings, err := j.Store.GetSettings(ctx)
	if err == nil && settings.MaxConcurrentBackups > 0 && settings.MaxConcurrentBackups < limit {
		limit = settings.MaxConcurrentBackups
	}
	if limit < 1 {
		return 1
	}
	return limit
}

func (j *Jobs) executeBackup(id string) {
	defer j.wg.Done()
	ctx, cancel := context.WithTimeout(j.root, j.Timeout)
	defer cancel()
	defer func() {
		if recovered := recover(); recovered != nil {
			j.failBackup(id, "backup failed")
		}
	}()
	item, err := j.Store.GetBackup(ctx, id)
	if err != nil {
		j.Log.Error("load backup", "backup_id", id, "error", err.Error())
		return
	}
	src, err := j.Store.GetSource(ctx, item.SourceID)
	if err != nil {
		j.failBackup(id, "source is no longer available")
		return
	}
	password, err := j.decryptSource(src)
	if err != nil {
		j.failBackup(id, "stored credential could not be read")
		return
	}
	ext := model.Extension(src.Type)
	temp, err := j.Files.CreateTemp(id, ext)
	if err != nil {
		j.failBackup(id, "temporary backup file could not be created")
		return
	}
	rel := src.ID + "/" + id + ext
	version, err := j.Engine.Dump(ctx, src, password, temp, j.Files.Root())
	if err != nil {
		_ = j.Files.RemoveTemp(temp)
		j.failBackup(id, security.Truncate(security.Redact(err.Error(), password), 400))
		return
	}
	size, sum, err := j.Files.Publish(temp, rel)
	if err != nil {
		_ = j.Files.RemoveTemp(temp)
		_ = j.Files.Remove(rel)
		j.failBackup(id, "backup file could not be published")
		return
	}
	if err := j.Store.CompleteBackup(ctx, id, rel, sum, version, size); err != nil {
		_ = j.Files.Remove(rel)
		j.failBackup(id, "backup metadata could not be saved")
		return
	}
	if item.UploadRequested {
		j.queueUpload(ctx, id)
	}
	if err := j.ApplyRetention(ctx); err != nil {
		j.Log.Error("retention", "error", err.Error())
	}
	j.Poke()
}

func (j *Jobs) queueUpload(ctx context.Context, backupID string) {
	_, configured, err := loadS3(ctx, j.Store, j.Cipher, j.AllowInsecure)
	if err != nil {
		_, _ = j.Store.InsertUpload(ctx, backupID, model.StatusFailed, "object storage credentials could not be read")
		return
	}
	if !configured {
		_, _ = j.Store.InsertUpload(ctx, backupID, model.StatusFailed, "object storage is not configured")
		return
	}
	if _, err := j.Store.InsertUpload(ctx, backupID, model.StatusPending, ""); err != nil {
		j.Log.Error("queue upload", "backup_id", backupID, "error", err.Error())
	}
}

func (j *Jobs) executeUpload(uploadID, backupID string) {
	defer j.wg.Done()
	ctx, cancel := context.WithTimeout(j.root, j.Timeout)
	defer cancel()
	item, err := j.Store.GetBackup(ctx, backupID)
	if err != nil || item.Status != model.StatusCompleted || item.RelativePath == "" {
		_ = j.Store.FailUpload(ctx, uploadID, "backup file is not available")
		return
	}
	cfg, configured, err := loadS3(ctx, j.Store, j.Cipher, j.AllowInsecure)
	if err != nil || !configured {
		_ = j.Store.FailUpload(ctx, uploadID, "object storage is not configured")
		return
	}
	path, err := j.Files.Path(item.RelativePath)
	if err != nil {
		_ = j.Store.FailUpload(ctx, uploadID, "backup file is not available")
		return
	}
	key := storage.ObjectKey(cfg.Prefix, item.RelativePath)
	if err := j.Objects.Upload(ctx, cfg, key, path, item.ChecksumSHA256); err != nil {
		_ = j.Store.FailUpload(ctx, uploadID, security.Truncate(security.Redact(err.Error(), cfg.AccessKey, cfg.SecretKey), 400))
		j.Log.Error("upload", "backup_id", backupID, "error", security.Redact(err.Error(), cfg.AccessKey, cfg.SecretKey))
		return
	}
	if err := j.Store.CompleteUpload(ctx, uploadID, key); err != nil {
		j.Log.Error("record upload", "backup_id", backupID, "error", err.Error())
	}
}

func (j *Jobs) decryptSource(src model.Source) (string, error) {
	if src.PasswordCipher == "" {
		return "", nil
	}
	return j.Cipher.Decrypt("source:"+src.ID, src.PasswordCipher)
}

func (j *Jobs) failBackup(id, summary string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := j.Store.FailBackup(ctx, id, summary); err != nil {
		j.Log.Error("record backup failure", "backup_id", id, "error", err.Error())
	}
	j.Log.Error("backup failed", "backup_id", id, "error", summary)
}

// ApplyRetention deletes expired local files and remote objects independently.
func (j *Jobs) ApplyRetention(ctx context.Context) error {
	settings, err := j.Store.GetSettings(ctx)
	if err != nil {
		return err
	}
	sources, err := j.Store.ListSources(ctx)
	if err != nil {
		return err
	}
	cfg, configured, err := loadS3(ctx, j.Store, j.Cipher, j.AllowInsecure)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, src := range sources {
		policy := effectiveRetention(settings.Retention, src.Schedule.Retention)
		items, err := j.Store.ListBackupsBySource(ctx, src.ID)
		if err != nil {
			return err
		}
		for _, victim := range SelectLocalDeletions(items, policy.LocalCount, policy.LocalDays, now) {
			if err := j.Files.Remove(victim.RelativePath); err != nil {
				j.Log.Error("delete expired backup", "backup_id", victim.ID, "error", err.Error())
				continue
			}
			if err := j.Store.MarkBackupExpired(ctx, victim.ID); err != nil {
				j.Log.Error("mark backup expired", "backup_id", victim.ID, "error", err.Error())
			}
		}
		for _, victim := range SelectFailedDeletions(items, policy.LocalDays, now) {
			if err := j.Store.DeleteFailedBackup(ctx, victim.ID); err != nil {
				j.Log.Error("delete failed backup", "backup_id", victim.ID, "error", err.Error())
			}
		}
		if !configured {
			continue
		}
		for _, victim := range SelectRemoteDeletions(items, policy.RemoteCount, policy.RemoteDays, now) {
			if err := j.Objects.Delete(ctx, cfg, victim.Upload.ObjectKey); err != nil {
				j.Log.Error("delete expired object", "backup_id", victim.ID, "error", security.Redact(err.Error(), cfg.AccessKey, cfg.SecretKey))
				continue
			}
			if err := j.Store.ExpireUpload(ctx, victim.Upload.ID); err != nil {
				j.Log.Error("mark upload expired", "backup_id", victim.ID, "error", err.Error())
			}
		}
	}
	return nil
}

func effectiveRetention(global model.Retention, override model.RetentionOverride) model.Retention {
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
	return out
}
