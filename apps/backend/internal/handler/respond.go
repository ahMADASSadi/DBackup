// Package handler is the HTTP API.
package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"dbackup/internal/service"
)

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeErr(w http.ResponseWriter, err error) {
	if api, ok := service.As(err); ok {
		writeJSON(w, api.Status, map[string]any{"error": map[string]string{"code": api.Code, "message": api.Message}})
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]string{"code": "internal", "message": "internal error"}})
}

func decode(r *http.Request, dst any) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return service.E(http.StatusBadRequest, "bad_request", "invalid request body")
	}
	if len(body) == 0 {
		return service.E(http.StatusBadRequest, "bad_request", "request body is required")
	}
	dec := json.NewDecoder(bytesReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return service.E(http.StatusBadRequest, "bad_request", "invalid JSON")
	}
	return nil
}

func decodeOptional(r *http.Request, dst any) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return service.E(http.StatusBadRequest, "bad_request", "invalid request body")
	}
	if len(body) == 0 {
		return nil
	}
	dec := json.NewDecoder(bytesReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return service.E(http.StatusBadRequest, "bad_request", "invalid JSON")
	}
	return nil
}

func parseTimeQuery(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, errors.New("invalid time")
	}
	parsed = parsed.UTC()
	return &parsed, nil
}
