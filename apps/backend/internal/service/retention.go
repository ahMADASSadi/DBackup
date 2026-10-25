package service

import (
	"sort"
	"time"

	"dbackup/internal/model"
)

// SelectLocalDeletions chooses completed backups whose files may be removed.
// The newest completed backup is always kept. When both a count and an age are
// set, a backup is removed only when it exceeds both limits.
func SelectLocalDeletions(items []model.Backup, count, days int, now time.Time) []model.Backup {
	completed := make([]model.Backup, 0, len(items))
	for _, item := range items {
		if item.Status == model.StatusCompleted && item.RelativePath != "" {
			completed = append(completed, item)
		}
	}
	sort.SliceStable(completed, func(i, j int) bool {
		return later(completed[i]).After(later(completed[j]))
	})
	if len(completed) <= 1 || (count <= 0 && days <= 0) {
		return nil
	}
	var victims []model.Backup
	for i, item := range completed {
		if i == 0 {
			continue
		}
		if exceeds(i, later(item), count, days, now) {
			victims = append(victims, item)
		}
	}
	return victims
}

// SelectFailedDeletions removes old failed rows once an age limit is configured.
func SelectFailedDeletions(items []model.Backup, days int, now time.Time) []model.Backup {
	if days <= 0 {
		return nil
	}
	var victims []model.Backup
	cutoff := now.Add(-time.Duration(days) * 24 * time.Hour)
	for _, item := range items {
		if item.Status == model.StatusFailed && item.CreatedAt.Before(cutoff) {
			victims = append(victims, item)
		}
	}
	return victims
}

// SelectRemoteDeletions chooses completed uploads that may be removed.
// It never selects the newest completed upload.
func SelectRemoteDeletions(items []model.Backup, count, days int, now time.Time) []model.Backup {
	uploaded := make([]model.Backup, 0, len(items))
	for _, item := range items {
		if item.Upload != nil && item.Upload.Status == model.StatusCompleted && item.Upload.ObjectKey != "" {
			uploaded = append(uploaded, item)
		}
	}
	sort.SliceStable(uploaded, func(i, j int) bool {
		return later(uploaded[i]).After(later(uploaded[j]))
	})
	if len(uploaded) <= 1 || (count <= 0 && days <= 0) {
		return nil
	}
	var victims []model.Backup
	for i, item := range uploaded {
		if i == 0 {
			continue
		}
		if exceeds(i, later(item), count, days, now) {
			victims = append(victims, item)
		}
	}
	return victims
}

func exceeds(index int, at time.Time, count, days int, now time.Time) bool {
	overCount := count > 0 && index >= count
	overAge := days > 0 && at.Before(now.Add(-time.Duration(days)*24*time.Hour))
	switch {
	case count > 0 && days > 0:
		return overCount && overAge
	case count > 0:
		return overCount
	case days > 0:
		return overAge
	default:
		return false
	}
}

func later(item model.Backup) time.Time {
	if item.CompletedAt != nil && !item.CompletedAt.IsZero() {
		return item.CompletedAt.UTC()
	}
	return item.CreatedAt.UTC()
}
