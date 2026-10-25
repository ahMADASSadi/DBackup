package scheduler

import (
	"testing"
	"time"
)

func TestNextRun(t *testing.T) {
	from := time.Date(2026, 10, 25, 1, 30, 0, 0, time.UTC)
	next, err := NextRun("interval", 3600, "", "UTC", from)
	if err != nil || !next.Equal(from.Add(time.Hour)) {
		t.Fatalf("interval: %v %s", err, next)
	}
	next, err = NextRun("cron", 0, "0 2 * * *", "UTC", from)
	if err != nil {
		t.Fatal(err)
	}
	if next.Hour() != 2 || next.Before(from) {
		t.Fatal(next)
	}
	if _, err := NextRun("cron", 0, "not a cron", "UTC", from); err == nil {
		t.Fatal("accepted invalid cron")
	}
	if _, err := NextRun("interval", 10, "", "UTC", from); err == nil {
		t.Fatal("accepted short interval")
	}
}
