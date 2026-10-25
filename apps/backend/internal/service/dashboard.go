package service

import (
	"context"
	"time"

	"dbackup/internal/model"
	"dbackup/internal/repository"
	"dbackup/internal/storage"
)

// SuccessView is the newest completed backup for a source.
type SuccessView struct {
	SourceID    string     `json:"source_id"`
	SourceName  string     `json:"source_name"`
	BackupID    string     `json:"backup_id,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// RunView is an upcoming scheduled backup.
type RunView struct {
	SourceID   string    `json:"source_id"`
	SourceName string    `json:"source_name"`
	NextRunAt  time.Time `json:"next_run_at"`
}

// Dashboard is the operations summary.
type Dashboard struct {
	Sources        int                `json:"sources"`
	Recent         []model.Backup     `json:"recent"`
	LastSuccess    []SuccessView      `json:"last_success"`
	NextRuns       []RunView          `json:"next_runs"`
	UsageBytes     int64              `json:"usage_bytes"`
	FailedJobs     int                `json:"failed_jobs"`
	PendingUploads int                `json:"pending_uploads"`
	FailedUploads  int                `json:"failed_uploads"`
	Audit          []model.AuditEvent `json:"audit"`
}

// Overview loads the dashboard.
type Overview struct {
	Store *repository.Store
	Files *storage.Local
}

func (o *Overview) Load(ctx context.Context) (Dashboard, error) {
	sources, err := o.Store.ListSources(ctx)
	if err != nil {
		return Dashboard{}, err
	}
	recent, err := o.Store.RecentBackups(ctx, 8)
	if err != nil {
		return Dashboard{}, err
	}
	if recent == nil {
		recent = []model.Backup{}
	}
	last, err := o.Store.LastSuccesses(ctx)
	if err != nil {
		return Dashboard{}, err
	}
	next, err := o.Store.NextRuns(ctx)
	if err != nil {
		return Dashboard{}, err
	}
	failed, err := o.Store.CountByStatus(ctx, model.StatusFailed)
	if err != nil {
		return Dashboard{}, err
	}
	pendingUploads, err := o.Store.CountUploadsByStatus(ctx, model.StatusPending, model.StatusRunning)
	if err != nil {
		return Dashboard{}, err
	}
	failedUploads, err := o.Store.CountUploadsByStatus(ctx, model.StatusFailed)
	if err != nil {
		return Dashboard{}, err
	}
	audit, err := o.Store.RecentAudit(ctx, 8)
	if err != nil {
		return Dashboard{}, err
	}
	if audit == nil {
		audit = []model.AuditEvent{}
	}
	usage, err := o.Files.Usage()
	if err != nil {
		usage = 0
	}
	view := Dashboard{
		Sources: len(sources), Recent: recent, UsageBytes: usage,
		FailedJobs: failed, PendingUploads: pendingUploads, FailedUploads: failedUploads,
		Audit: audit, LastSuccess: []SuccessView{}, NextRuns: []RunView{},
	}
	for _, item := range last {
		view.LastSuccess = append(view.LastSuccess, SuccessView{
			SourceID: item.SourceID, SourceName: item.SourceName, BackupID: item.BackupID, CompletedAt: item.At,
		})
	}
	for _, item := range next {
		view.NextRuns = append(view.NextRuns, RunView{SourceID: item.SourceID, SourceName: item.SourceName, NextRunAt: item.At})
	}
	return view, nil
}
