package backup

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"

	"dbackup/internal/security"

	"modernc.org/sqlite"
)

type sqliteBackuper interface {
	NewBackup(dstURI string) (*sqlite.Backup, error)
}

func dumpSQLite(ctx context.Context, sourcePath, destPath string) (string, error) {
	db, err := sql.Open("sqlite", sqliteReadDSN(sourcePath))
	if err != nil {
		return "", fmt.Errorf("open sqlite source: %w", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	var version string
	if err := db.QueryRowContext(ctx, `SELECT sqlite_version()`).Scan(&version); err != nil {
		return "", fmt.Errorf("read sqlite version: %w", err)
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return version, err
	}
	defer conn.Close()
	err = conn.Raw(func(driverConn any) error {
		backuper, ok := driverConn.(sqliteBackuper)
		if !ok {
			return fmt.Errorf("sqlite driver does not support the backup api")
		}
		backup, err := backuper.NewBackup(destPath)
		if err != nil {
			return err
		}
		for {
			if err := ctx.Err(); err != nil {
				_ = backup.Finish()
				return err
			}
			more, err := backup.Step(64)
			if err != nil {
				_ = backup.Finish()
				return err
			}
			if !more {
				return backup.Finish()
			}
		}
	})
	if err != nil {
		return "sqlite " + version, fmt.Errorf("sqlite backup failed: %s", security.Truncate(err.Error(), 300))
	}
	return "sqlite " + version, nil
}

func sqliteReadDSN(path string) string {
	u := url.URL{Scheme: "file", Path: path, RawQuery: "mode=ro"}
	return u.String()
}
