package handler

import (
	"net/http"
	"strconv"

	"dbackup/internal/middleware"
	"dbackup/internal/model"
	"dbackup/internal/service"
)

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	if err := h.DB.PingContext(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	view, err := h.Overview.Load(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (h *Handler) ListSources(w http.ResponseWriter, r *http.Request) {
	items, err := h.Catalog.List(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) CreateSource(w http.ResponseWriter, r *http.Request) {
	var req service.SourceInput
	if err := decode(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	src, err := h.Catalog.Create(r.Context(), req, actor(r))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, src)
}

func (h *Handler) GetSource(w http.ResponseWriter, r *http.Request) {
	src, err := h.Catalog.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, src)
}

func (h *Handler) UpdateSource(w http.ResponseWriter, r *http.Request) {
	var req service.SourceInput
	if err := decode(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	src, err := h.Catalog.Update(r.Context(), r.PathValue("id"), req, actor(r))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, src)
}

func (h *Handler) DeleteSource(w http.ResponseWriter, r *http.Request) {
	deleteBackups := r.URL.Query().Get("delete_backups") == "true"
	if err := h.Catalog.Delete(r.Context(), r.PathValue("id"), deleteBackups, actor(r)); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) TestSource(w http.ResponseWriter, r *http.Request) {
	if err := h.Catalog.Test(r.Context(), r.PathValue("id")); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

type backupRequest struct {
	Upload *bool `json:"upload"`
}

func (h *Handler) CreateBackup(w http.ResponseWriter, r *http.Request) {
	var req backupRequest
	if err := decodeOptional(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	item, err := h.Jobs.Enqueue(r.Context(), r.PathValue("id"), model.TriggerManual, req.Upload, actor(r))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, item)
}

func (h *Handler) ListBackups(w http.ResponseWriter, r *http.Request) {
	from, err := parseTimeQuery(r.URL.Query().Get("from"))
	if err != nil {
		writeErr(w, service.E(http.StatusBadRequest, "validation_error", "from must be RFC3339"))
		return
	}
	to, err := parseTimeQuery(r.URL.Query().Get("to"))
	if err != nil {
		writeErr(w, service.E(http.StatusBadRequest, "validation_error", "to must be RFC3339"))
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	page, err := h.Jobs.List(r.Context(), model.BackupFilter{
		SourceID: r.URL.Query().Get("source_id"),
		Status:   r.URL.Query().Get("status"),
		From:     from,
		To:       to,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (h *Handler) GetBackup(w http.ResponseWriter, r *http.Request) {
	item, err := h.Jobs.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) DownloadBackup(w http.ResponseWriter, r *http.Request) {
	file, err := h.Jobs.Open(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	defer file.File.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+file.Name+`"`)
	http.ServeContent(w, r, file.Name, file.Info.ModTime(), file.File)
}

func (h *Handler) DeleteBackup(w http.ResponseWriter, r *http.Request) {
	if err := h.Jobs.Delete(r.Context(), r.PathValue("id"), r.URL.Query().Get("force") == "true", actor(r)); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) RetryBackup(w http.ResponseWriter, r *http.Request) {
	item, err := h.Jobs.Retry(r.Context(), r.PathValue("id"), actor(r))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, item)
}

func (h *Handler) RetryUpload(w http.ResponseWriter, r *http.Request) {
	item, err := h.Jobs.RetryUpload(r.Context(), r.PathValue("id"), actor(r))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, item)
}

func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
	view, err := h.Settings.View(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (h *Handler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req service.SettingsUpdate
	if err := decode(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	view, err := h.Settings.Update(r.Context(), req, actor(r))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (h *Handler) TestS3(w http.ResponseWriter, r *http.Request) {
	var req service.S3Update
	if err := decode(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if err := h.Settings.TestS3(r.Context(), req); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func actor(r *http.Request) string {
	if principal, ok := middleware.PrincipalFrom(r.Context()); ok && principal.Username != "" {
		return principal.Username
	}
	return "admin"
}
