// Package app wires the control plane.
package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"dbackup/internal/backup"
	"dbackup/internal/config"
	"dbackup/internal/database"
	"dbackup/internal/handler"
	"dbackup/internal/middleware"
	"dbackup/internal/repository"
	"dbackup/internal/security"
	"dbackup/internal/service"
	"dbackup/internal/storage"
)

// Version is the control plane release.
const Version = "0.1.0"

// App is a running control plane.
type App struct {
	Handler http.Handler
	Jobs    *service.Jobs
	DB      *sql.DB
	Log     *slog.Logger
}

// New opens storage, migrates metadata, and builds the HTTP handler.
func New(cfg config.Config, log *slog.Logger) (*App, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if log == nil {
		log = slog.Default()
	}
	backupDir, err := filepath.Abs(cfg.BackupDir)
	if err != nil {
		return nil, err
	}
	metaPath, err := filepath.Abs(cfg.MetadataDBPath)
	if err != nil {
		return nil, err
	}
	if err := security.ValidateDataPaths(backupDir, metaPath); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(metaPath), 0o700); err != nil {
		return nil, fmt.Errorf("create metadata directory: %w", err)
	}
	for _, root := range cfg.SQLiteAllowedRoots {
		if _, err := os.Stat(root); err != nil {
			return nil, fmt.Errorf("sqlite allow root %s is not accessible", root)
		}
	}
	db, err := database.Open(metaPath)
	if err != nil {
		return nil, err
	}
	files, err := storage.NewLocal(backupDir)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := files.CleanupTemp(); err != nil {
		_ = db.Close()
		return nil, err
	}
	cipher, err := security.NewCipher(cfg.EncryptionKey)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	store := repository.New(db)
	if err := store.ReconcileInterrupted(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	vault := security.NewVault()
	auth, err := service.NewAuth(store, cfg.SessionTTL, vault)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := auth.Bootstrap(context.Background(), cfg); err != nil {
		_ = db.Close()
		return nil, err
	}
	trusted, err := middleware.ParseCIDRs(cfg.TrustedProxies)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("TRUSTED_PROXIES: %w", err)
	}
	engine := &backup.Engine{Roots: cfg.SQLiteAllowedRoots, Allowlist: cfg.DBHostAllowlist, MetaPath: metaPath}
	tools := engine.CheckTools(context.Background())
	if tools.PgError != "" {
		log.Error("backup tool", "tool", "pg_dump", "error", tools.PgError)
	} else {
		log.Info("backup tool", "tool", "pg_dump", "version", tools.PgDump)
	}
	if tools.MyError != "" {
		log.Error("backup tool", "tool", "mysqldump", "error", tools.MyError)
	} else {
		log.Info("backup tool", "tool", "mysqldump", "version", tools.MySQLDump)
	}
	objects := storage.S3{Resolver: security.SystemResolver{}}
	jobs := service.NewJobs(store, engine, files, objects, cipher, log, cfg.MaxConcurrentBackups, cfg.BackupTimeout, cfg.S3AllowInsecure, vault)
	api := &handler.Handler{
		Auth: auth,
		Catalog: &service.Catalog{
			Store: store, Engine: engine, Cipher: cipher, Files: files, Objects: objects,
			AllowInsecure: cfg.S3AllowInsecure, BackupDir: files.Root(), MetaPath: metaPath,
		},
		Jobs: jobs,
		Settings: &service.Settings{
			Store: store, Cipher: cipher, Files: files, Objects: objects,
			AllowInsecure: cfg.S3AllowInsecure, HardLimit: cfg.MaxConcurrentBackups,
			BackupDir: files.Root(), MetaPath: metaPath, Roots: cfg.SQLiteAllowedRoots,
			Tools: tools, Version: Version, Resolver: security.SystemResolver{},
		},
		Overview:     &service.Overview{Store: store, Files: files},
		DB:           db,
		SecureCookie: cfg.CookieSecure,
		Trusted:      trusted,
	}
	return &App{Handler: middleware.Secure(log, api.Routes()), Jobs: jobs, DB: db, Log: log}, nil
}

// Start begins background backup work.
func (a *App) Start(ctx context.Context) { a.Jobs.Start(ctx) }

// Stop waits for background work to observe cancellation.
func (a *App) Stop() { a.Jobs.Stop() }

// Close releases the metadata database.
func (a *App) Close() error {
	if a.DB != nil {
		return a.DB.Close()
	}
	return nil
}
