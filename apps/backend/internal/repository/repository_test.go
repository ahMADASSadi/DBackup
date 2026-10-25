package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"dbackup/internal/database"
	"dbackup/internal/model"
	"dbackup/internal/security"
)

func TestMigrationAndInterruptedBackup(t *testing.T) {
	dir := t.TempDir()
	db, err := database.Open(filepath.Join(dir, "meta.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db2, err := database.Open(filepath.Join(dir, "meta.db"))
	if err != nil {
		t.Fatal(err)
	}
	db2.Close()
	store := New(db)
	id, err := security.NewID()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	src := model.Source{
		ID: id, Name: "app", Type: model.TypeSQLite, Enabled: true,
		Config:   model.ConnConfig{Path: "/sqlite-sources/app.db"},
		Schedule: model.Schedule{Mode: model.ModeCron, Cron: "0 2 * * *", Timezone: "UTC"},
	}
	if err := store.CreateSource(context.Background(), src); err != nil {
		t.Fatal(err)
	}
	backupID, err := security.NewID()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.InsertBackup(context.Background(), model.Backup{
		ID: backupID, SourceID: id, DBType: model.TypeSQLite, Status: model.StatusRunning,
		Format: model.FormatSQLite, Trigger: model.TriggerManual, CreatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	pendingID, err := security.NewID()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.InsertBackup(context.Background(), model.Backup{
		ID: pendingID, SourceID: id, DBType: model.TypeSQLite, Status: model.StatusPending,
		Format: model.FormatSQLite, Trigger: model.TriggerManual, CreatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.ReconcileInterrupted(context.Background()); err != nil {
		t.Fatal(err)
	}
	running, err := store.GetBackup(context.Background(), backupID)
	if err != nil {
		t.Fatal(err)
	}
	if running.Status != model.StatusFailed || running.ErrorSummary == "" {
		t.Fatalf("running backup was not failed: %+v", running)
	}
	pending, err := store.GetBackup(context.Background(), pendingID)
	if err != nil {
		t.Fatal(err)
	}
	if pending.Status != model.StatusPending {
		t.Fatalf("pending backup changed: %s", pending.Status)
	}
}
