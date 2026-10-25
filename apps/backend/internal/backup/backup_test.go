package backup

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dbackup/internal/model"

	_ "modernc.org/sqlite"
)

func TestSQLiteBackupSeesWAL(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "app.db")
	db, err := sql.Open("sqlite", "file:"+source)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`PRAGMA wal_autocheckpoint=0`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO items(id, name) VALUES (1, 'alpha')`); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "snap.db")
	if _, err := dumpSQLite(context.Background(), source, dest); err != nil {
		t.Fatal(err)
	}
	restored, err := sql.Open("sqlite", "file:"+dest+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	var name string
	if err := restored.QueryRow(`SELECT name FROM items WHERE id = 1`).Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != "alpha" {
		t.Fatalf("restored %q", name)
	}
}

type fakeRunner struct {
	calls []fakeCall
}

type fakeCall struct {
	name string
	args []string
	env  []string
}

func (f *fakeRunner) LookPath(file string) (string, error) { return "/usr/bin/" + file, nil }

func (f *fakeRunner) Run(_ context.Context, name string, args []string, env []string) (Result, error) {
	f.calls = append(f.calls, fakeCall{name: name, args: append([]string(nil), args...), env: append([]string(nil), env...)})
	joined := strings.Join(args, "\n")
	if strings.Contains(joined, "--version") {
		if name == "pg_dump" {
			return Result{Stdout: "pg_dump (PostgreSQL) 17.0\n"}, nil
		}
		return Result{Stdout: "mysqldump  Ver 10.19 Distrib 10.11.6-MariaDB\n"}, nil
	}
	for _, arg := range args {
		if strings.Contains(arg, "db-secret") {
			return Result{}, errLeaked
		}
	}
	for _, item := range env {
		if strings.HasPrefix(item, "PGPASSWORD=") || strings.HasPrefix(item, "MYSQL_PWD=") {
			return Result{}, errLeaked
		}
	}
	for i, arg := range args {
		if arg == "--file" && i+1 < len(args) {
			if err := os.WriteFile(args[i+1], []byte("PGDMP"), 0o600); err != nil {
				return Result{}, err
			}
		}
		if strings.HasPrefix(arg, "--result-file=") {
			if err := os.WriteFile(strings.TrimPrefix(arg, "--result-file="), []byte("-- dump"), 0o600); err != nil {
				return Result{}, err
			}
		}
	}
	return Result{}, nil
}

var errLeaked = leakError("secret leaked into process arguments")

type leakError string

func (e leakError) Error() string { return string(e) }

func TestDumpDoesNotPassSecretsToShell(t *testing.T) {
	runner := &fakeRunner{}
	engine := &Engine{Runner: runner}
	source := model.Source{Type: model.TypePostgres, Config: model.ConnConfig{
		Host: "127.0.0.1", Port: 5432, Database: "app", Username: "backup", TLSMode: "disable",
	}}
	dest := filepath.Join(t.TempDir(), "out.dump")
	if _, err := engine.Dump(context.Background(), source, "db-secret", dest, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) == 0 || runner.calls[0].name != "pg_dump" {
		t.Fatalf("calls %+v", runner.calls)
	}
	for _, call := range runner.calls {
		if call.name == "sh" || call.name == "bash" {
			t.Fatal("used a shell")
		}
	}
	hostile := source
	hostile.Config.Host = "127.0.0.1;touch"
	runner.calls = nil
	if _, err := engine.Dump(context.Background(), hostile, "db-secret", dest, t.TempDir()); err == nil {
		t.Fatal("accepted a shell-like host")
	}
	if len(runner.calls) != 0 {
		t.Fatal("runner was invoked for a rejected host")
	}
}
