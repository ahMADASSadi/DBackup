package handler

import (
	"database/sql"
	"net"
	"net/http"

	"dbackup/internal/service"
)

// Handler serves the versioned API.
type Handler struct {
	Auth         *service.Auth
	Catalog      *service.Catalog
	Jobs         *service.Jobs
	Settings     *service.Settings
	Overview     *service.Overview
	DB           *sql.DB
	SecureCookie bool
	Trusted      []*net.IPNet
}

// Routes returns the authenticated API handler.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", h.Health)
	mux.HandleFunc("POST /api/v1/auth/login", h.Login)
	mux.HandleFunc("POST /api/v1/auth/logout", h.Logout)
	mux.HandleFunc("GET /api/v1/auth/session", h.Session)
	mux.HandleFunc("POST /api/v1/auth/unlock", h.Unlock)
	mux.HandleFunc("PUT /api/v1/auth/password", h.Password)
	mux.HandleFunc("GET /api/v1/dashboard", h.Dashboard)
	mux.HandleFunc("GET /api/v1/sources", h.ListSources)
	mux.HandleFunc("POST /api/v1/sources", h.CreateSource)
	mux.HandleFunc("GET /api/v1/sources/{id}", h.GetSource)
	mux.HandleFunc("PUT /api/v1/sources/{id}", h.UpdateSource)
	mux.HandleFunc("DELETE /api/v1/sources/{id}", h.DeleteSource)
	mux.HandleFunc("POST /api/v1/sources/{id}/test", h.TestSource)
	mux.HandleFunc("POST /api/v1/sources/{id}/backups", h.CreateBackup)
	mux.HandleFunc("GET /api/v1/backups", h.ListBackups)
	mux.HandleFunc("GET /api/v1/backups/{id}", h.GetBackup)
	mux.HandleFunc("GET /api/v1/backups/{id}/download", h.DownloadBackup)
	mux.HandleFunc("DELETE /api/v1/backups/{id}", h.DeleteBackup)
	mux.HandleFunc("POST /api/v1/backups/{id}/retry", h.RetryBackup)
	mux.HandleFunc("POST /api/v1/backups/{id}/retry-upload", h.RetryUpload)
	mux.HandleFunc("GET /api/v1/settings", h.GetSettings)
	mux.HandleFunc("PUT /api/v1/settings", h.UpdateSettings)
	mux.HandleFunc("POST /api/v1/settings/s3/test", h.TestS3)
	return h.Authenticate(mux)
}
