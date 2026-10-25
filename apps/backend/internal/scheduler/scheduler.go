// Package scheduler calculates the next run of a backup policy.
package scheduler

import (
	"fmt"
	"strings"
	"time"

	"dbackup/internal/model"

	"github.com/robfig/cron/v3"
)

const (
	minInterval = 60
	maxInterval = 366 * 24 * 3600
)

// NextRun returns the next execution time in UTC.
func NextRun(mode string, intervalSeconds int, cronExpr, timezone string, from time.Time) (time.Time, error) {
	if strings.TrimSpace(timezone) == "" {
		timezone = "UTC"
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, fmt.Errorf("timezone is invalid")
	}
	if from.IsZero() {
		from = time.Now()
	}
	switch mode {
	case model.ModeInterval:
		if intervalSeconds < minInterval || intervalSeconds > maxInterval {
			return time.Time{}, fmt.Errorf("interval must be between 60 seconds and 366 days")
		}
		return from.UTC().Add(time.Duration(intervalSeconds) * time.Second), nil
	case model.ModeCron:
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		schedule, err := parser.Parse(strings.TrimSpace(cronExpr))
		if err != nil {
			return time.Time{}, fmt.Errorf("cron expression is invalid")
		}
		return schedule.Next(from.In(location)).UTC(), nil
	default:
		return time.Time{}, fmt.Errorf("schedule mode must be interval or cron")
	}
}
