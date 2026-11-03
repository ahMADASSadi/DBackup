package app_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"dbackup/internal/app"
	"dbackup/internal/config"
	"dbackup/internal/security"

	_ "modernc.org/sqlite"
)

func TestAuthBackupAndSecretRedaction(t *testing.T) {
	application := newApp(t)
	server := httptest.NewServer(application.Handler)
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}

	if res := do(t, client, http.MethodGet, server.URL+"/api/v1/dashboard", "", nil); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("dashboard status %d", res.StatusCode)
	}
	for i := 0; i < 8; i++ {
		res := do(t, client, http.MethodPost, server.URL+"/api/v1/auth/login", "", map[string]string{"username": "admin", "password": "wrong-password-value"})
		if res.StatusCode != http.StatusUnauthorized {
			t.Fatalf("attempt %d status %d", i, res.StatusCode)
		}
	}
	limited := do(t, client, http.MethodPost, server.URL+"/api/v1/auth/login", "", map[string]string{"username": "admin", "password": "correct-horse-battery"})
	if limited.StatusCode != http.StatusTooManyRequests {
		body, _ := io.ReadAll(limited.Body)
		t.Fatalf("rate limit status %d %s", limited.StatusCode, body)
	}

	fresh := newApp(t)
	okServer := httptest.NewServer(fresh.Handler)
	defer okServer.Close()
	okJar, _ := cookiejar.New(nil)
	okClient := &http.Client{Jar: okJar, Timeout: 10 * time.Second}
	login := do(t, okClient, http.MethodPost, okServer.URL+"/api/v1/auth/login", "", map[string]string{"username": "admin", "password": "correct-horse-battery"})
	if login.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(login.Body)
		t.Fatalf("login %d %s", login.StatusCode, body)
	}
	var session struct {
		CSRF string `json:"csrf_token"`
	}
	decode(t, login, &session)
	if session.CSRF == "" {
		t.Fatal("missing csrf")
	}
	denied := do(t, okClient, http.MethodPost, okServer.URL+"/api/v1/sources", "", map[string]any{"name": "x"})
	if denied.StatusCode != http.StatusForbidden {
		t.Fatalf("csrf status %d", denied.StatusCode)
	}

	dbPath := filepath.Join(freshRoot(t, fresh), "unused")
	_ = dbPath
	sourceDir := sqliteRoot(t)
	dbFile := filepath.Join(sourceDir, "app.db")
	writeSQLite(t, dbFile)
	// The app under test has its own allow root. Recreate the file inside that root.
	root := allowRootFromSettings(t, okClient, okServer.URL, session.CSRF)
	dbFile = filepath.Join(root, "app.db")
	writeSQLite(t, dbFile)
	created := do(t, okClient, http.MethodPost, okServer.URL+"/api/v1/sources", session.CSRF, map[string]any{
		"name": "Application", "type": "sqlite", "enabled": true,
		"config":   map[string]any{"path": dbFile},
		"schedule": map[string]any{"enabled": false, "mode": "cron", "cron": "0 2 * * *", "timezone": "UTC", "upload": false},
	})
	if created.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(created.Body)
		t.Fatalf("create source %d %s", created.StatusCode, body)
	}
	var source struct {
		ID string `json:"id"`
	}
	raw, _ := io.ReadAll(created.Body)
	if bytes.Contains(raw, []byte("db-secret")) {
		t.Fatal("secret leaked in source response")
	}
	if err := json.Unmarshal(raw, &source); err != nil {
		t.Fatal(err)
	}
	backup := do(t, okClient, http.MethodPost, okServer.URL+"/api/v1/sources/"+source.ID+"/backups", session.CSRF, map[string]any{})
	if backup.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(backup.Body)
		t.Fatalf("backup %d %s", backup.StatusCode, body)
	}
	var job struct {
		ID string `json:"id"`
	}
	decode(t, backup, &job)
	deadline := time.Now().Add(15 * time.Second)
	status := ""
	for time.Now().Before(deadline) {
		res := do(t, okClient, http.MethodGet, okServer.URL+"/api/v1/backups/"+job.ID, "", nil)
		var body struct {
			Status string `json:"status"`
			Error  string `json:"error_summary"`
		}
		decode(t, res, &body)
		status = body.Status
		if status == "completed" || status == "failed" {
			if status == "failed" {
				t.Fatal(body.Error)
			}
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if status != "completed" {
		t.Fatalf("backup status %s", status)
	}
	download := do(t, okClient, http.MethodGet, okServer.URL+"/api/v1/backups/"+job.ID+"/download", "", nil)
	if download.StatusCode != http.StatusOK {
		t.Fatalf("download %d", download.StatusCode)
	}
	payload, _ := io.ReadAll(download.Body)
	sealed := filepath.Join(t.TempDir(), "backup.dbk")
	if err := os.WriteFile(sealed, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	restored := filepath.Join(t.TempDir(), "restored.db")
	if err := security.OpenFile(restored, sealed, "correct-horse-battery"); err != nil {
		t.Fatal(err)
	}
	check, err := sql.Open("sqlite", "file:"+restored+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer check.Close()
	var name string
	if err := check.QueryRow(`SELECT name FROM items WHERE id = 1`).Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != "alpha" {
		t.Fatal(name)
	}
	removed := do(t, okClient, http.MethodDelete, okServer.URL+"/api/v1/backups/"+job.ID, session.CSRF, nil)
	if removed.StatusCode != http.StatusConflict {
		t.Fatalf("delete last backup %d", removed.StatusCode)
	}
}

func newApp(t *testing.T) *app.App {
	t.Helper()
	root := t.TempDir()
	t.Setenv("unused", root)
	sources := filepath.Join(root, "sqlite-sources")
	if err := os.MkdirAll(sources, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		AdminUsername:        "admin",
		AdminPassword:        "correct-horse-battery",
		EncryptionKey:        bytes.Repeat([]byte{9}, 32),
		SessionTTL:           time.Hour,
		BackupDir:            filepath.Join(root, "backups"),
		MetadataDBPath:       filepath.Join(root, "meta", "dbackup.db"),
		SQLiteAllowedRoots:   []string{sources},
		HTTPAddr:             ":8080",
		MaxConcurrentBackups: 2,
		LogLevel:             "error",
		BackupTimeout:        time.Minute,
	}
	application, err := app.New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	application.Start(ctx)
	t.Cleanup(func() {
		cancel()
		application.Stop()
		application.Close()
	})
	t.Setenv("DBACKUP_TEST_ROOT_"+filepath.Base(root), sources)
	storeRoot(root, sources)
	return application
}

var roots = map[string]string{}

func storeRoot(appRoot, sources string) { roots[appRoot] = sources }

func freshRoot(t *testing.T, _ *app.App) string { t.Helper(); return "" }

func sqliteRoot(t *testing.T) string { t.Helper(); return t.TempDir() }

func allowRootFromSettings(t *testing.T, client *http.Client, base, csrf string) string {
	t.Helper()
	res := do(t, client, http.MethodGet, base+"/api/v1/settings", csrf, nil)
	var body struct {
		Roots []string `json:"sqlite_allowed_roots"`
	}
	decode(t, res, &body)
	if len(body.Roots) == 0 {
		t.Fatal("missing allow root")
	}
	return body.Roots[0]
}

func writeSQLite(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT); INSERT INTO items(id, name) VALUES (1, 'alpha')`); err != nil {
		t.Fatal(err)
	}
}

func do(t *testing.T, client *http.Client, method, url, csrf string, body any) *http.Response {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
	}
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { res.Body.Close() })
	return res
}

func decode(t *testing.T, res *http.Response, dst any) {
	t.Helper()
	defer res.Body.Close()
	if err := json.NewDecoder(res.Body).Decode(dst); err != nil {
		t.Fatal(err)
	}
}
