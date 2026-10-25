package service

import (
	"testing"
	"time"

	"dbackup/internal/model"
)

func completed(id string, age time.Duration, now time.Time) model.Backup {
	at := now.Add(-age)
	return model.Backup{ID: id, Status: model.StatusCompleted, RelativePath: id + ".dump", CompletedAt: &at, CreatedAt: at}
}

func TestRetentionKeepsNewestAndRequiresBothLimits(t *testing.T) {
	now := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
	only := []model.Backup{completed("only", 90*24*time.Hour, now)}
	if got := SelectLocalDeletions(only, 1, 1, now); len(got) != 0 {
		t.Fatal("deleted the only backup")
	}
	items := []model.Backup{
		completed("new", time.Hour, now),
		completed("old", 40*24*time.Hour, now),
		completed("older", 80*24*time.Hour, now),
	}
	got := SelectLocalDeletions(items, 1, 30, now)
	if len(got) != 2 {
		t.Fatalf("expected two deletions, got %d", len(got))
	}
	recent := SelectLocalDeletions(items, 1, 100, now)
	if len(recent) != 0 {
		t.Fatal("age limit should have kept the older backups")
	}
	failed := []model.Backup{{ID: "bad", Status: model.StatusFailed, CreatedAt: now.Add(-10 * 24 * time.Hour)}}
	if len(SelectFailedDeletions(failed, 7, now)) != 1 {
		t.Fatal("expected old failure to be removed")
	}
	if len(SelectFailedDeletions(failed, 0, now)) != 0 {
		t.Fatal("disabled age policy removed a failure")
	}
}

func TestRemoteRetentionDoesNotSelectOnlyUpload(t *testing.T) {
	now := time.Now()
	at := now.Add(-90 * 24 * time.Hour)
	item := model.Backup{ID: "b", Status: model.StatusCompleted, CompletedAt: &at, Upload: &model.Upload{ID: "u", Status: model.StatusCompleted, ObjectKey: "k"}}
	if len(SelectRemoteDeletions([]model.Backup{item}, 1, 1, now)) != 0 {
		t.Fatal("deleted the only remote object")
	}
}
