//go:build integration

package backup_test

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"dbackup/internal/backup"
	"dbackup/internal/model"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestPostgresDumpRestores(t *testing.T) {
	ctx := context.Background()
	host, port, user, password, database := mustEnv(t, "PGHOST"), mustEnv(t, "PGPORT"), mustEnv(t, "PGUSER"), os.Getenv("PGPASSWORD"), mustEnv(t, "PGDATABASE")
	db := openPG(t, host, port, user, password, database)
	if _, err := db.Exec(`DROP TABLE IF EXISTS items`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE items (id integer primary key, name text not null)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO items(id, name) VALUES (1, 'alpha')`); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "pg.dump")
	engine := &backup.Engine{}
	source := model.Source{Type: model.TypePostgres, Config: model.ConnConfig{Host: host, Port: atoi(t, port), Database: database, Username: user, TLSMode: "disable"}}
	if _, err := engine.Dump(ctx, source, password, dest, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	restoreDB := database + "_restore"
	if _, err := db.Exec(`DROP DATABASE IF EXISTS ` + restoreDB); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE DATABASE ` + restoreDB); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DROP DATABASE IF EXISTS ` + restoreDB) })
	cmd := exec.Command("pg_restore", "--no-owner", "--host", host, "--port", port, "--username", user, "--dbname", restoreDB, dest)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+password)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("pg_restore: %v %s", err, out)
	}
	restored := openPG(t, host, port, user, password, restoreDB)
	var name string
	if err := restored.QueryRow(`SELECT name FROM items WHERE id = 1`).Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != "alpha" {
		t.Fatalf("restored %q", name)
	}
}

func TestMySQLDumpRestores(t *testing.T) {
	ctx := context.Background()
	host, port, user, password, database := mustEnv(t, "MYHOST"), mustEnv(t, "MYPORT"), mustEnv(t, "MYUSER"), os.Getenv("MYPASSWORD"), mustEnv(t, "MYDATABASE")
	db := openMy(t, host, port, user, password, database)
	if _, err := db.Exec(`DROP TABLE IF EXISTS items`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE items (id int primary key, name varchar(64) not null)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO items(id, name) VALUES (1, 'alpha')`); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "mysql.sql")
	engine := &backup.Engine{}
	source := model.Source{Type: model.TypeMySQL, Config: model.ConnConfig{Host: host, Port: atoi(t, port), Database: database, Username: user, TLSMode: "disable"}}
	if _, err := engine.Dump(ctx, source, password, dest, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DROP TABLE items`); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	cnf := filepath.Join(t.TempDir(), "client.cnf")
	content := fmt.Sprintf("[client]\nhost=%s\nport=%s\nuser=%s\npassword=%s\nskip-ssl\n", host, port, user, password)
	if err := os.WriteFile(cnf, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("mysql", "--defaults-extra-file="+cnf, database)
	cmd.Stdin = file
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("mysql restore: %v %s", err, out)
	}
	var name string
	if err := db.QueryRow(`SELECT name FROM items WHERE id = 1`).Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != "alpha" {
		t.Fatalf("restored %q", name)
	}
}

func openPG(t *testing.T, host, port, user, password, database string) *sql.DB {
	t.Helper()
	u := &url.URL{Scheme: "postgres", User: url.UserPassword(user, password), Host: net.JoinHostPort(host, port), Path: "/" + database, RawQuery: "sslmode=disable"}
	db, err := sql.Open("pgx", u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Ping(); err != nil {
		t.Fatal(err)
	}
	return db
}

func openMy(t *testing.T, host, port, user, password, database string) *sql.DB {
	t.Helper()
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?parseTime=true&tls=false", user, password, net.JoinHostPort(host, port), database)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Ping(); err != nil {
		t.Fatal(err)
	}
	return db
}

func mustEnv(t *testing.T, key string) string {
	t.Helper()
	value := os.Getenv(key)
	if value == "" {
		t.Fatalf("%s is required", key)
	}
	return value
}

func atoi(t *testing.T, value string) int {
	t.Helper()
	port, err := strconv.Atoi(value)
	if err != nil {
		t.Fatal(err)
	}
	return port
}
