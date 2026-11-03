package handler

import (
	"net/http"
	"time"

	"dbackup/internal/middleware"
	"dbackup/internal/service"
)

const sessionCookie = "dbackup_session"

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type passwordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decode(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	result, err := h.Auth.Login(r.Context(), req.Username, req.Password, middleware.ClientIP(r, h.Trusted))
	if err != nil {
		writeErr(w, err)
		return
	}
	h.setSessionCookie(w, result.Token, result.ExpiresAt)
	if h.Jobs != nil {
		h.Jobs.Poke()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated":       true,
		"username":            result.Username,
		"csrf_token":          result.CSRF,
		"encryption_unlocked": true,
	})
}

func (h *Handler) Unlock(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decode(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if err := h.Auth.Unlock(r.Context(), req.Password); err != nil {
		writeErr(w, err)
		return
	}
	if h.Jobs != nil {
		h.Jobs.Poke()
	}
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "encryption_unlocked": true})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	principal, _ := middleware.PrincipalFrom(r.Context())
	if err := h.Auth.Logout(r.Context(), principal.SessionID, principal.Username); err != nil {
		writeErr(w, err)
		return
	}
	h.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Session(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || cookie.Value == "" {
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
		return
	}
	session, username, err := h.Auth.Session(r.Context(), cookie.Value)
	if err != nil {
		h.clearSessionCookie(w)
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated":       true,
		"username":            username,
		"csrf_token":          session.CSRFSecret,
		"encryption_unlocked": h.Auth.Vault.Unlocked(),
	})
}

func (h *Handler) Password(w http.ResponseWriter, r *http.Request) {
	var req passwordRequest
	if err := decode(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if err := h.Auth.ChangePassword(r.Context(), req.CurrentPassword, req.NewPassword); err != nil {
		writeErr(w, err)
		return
	}
	h.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) setSessionCookie(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.SecureCookie,
		SameSite: http.SameSiteLaxMode,
		Expires:  expires,
		MaxAge:   int(time.Until(expires).Seconds()),
	})
}

func (h *Handler) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.SecureCookie,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

// Authenticate loads the session and enforces CSRF on state-changing requests.
func (h *Handler) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/health" || (r.URL.Path == "/api/v1/auth/login" && r.Method == http.MethodPost) || (r.URL.Path == "/api/v1/auth/session" && r.Method == http.MethodGet) {
			next.ServeHTTP(w, r)
			return
		}
		cookie, err := r.Cookie(sessionCookie)
		if err != nil {
			writeErr(w, service.E(http.StatusUnauthorized, "unauthorized", "authentication required"))
			return
		}
		session, username, err := h.Auth.Session(r.Context(), cookie.Value)
		if err != nil {
			writeErr(w, err)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if !tokenOK(r.Header.Get("X-CSRF-Token"), session.CSRFSecret) {
				writeErr(w, service.E(http.StatusForbidden, "csrf", "csrf token is missing or invalid"))
				return
			}
		}
		ctx := middleware.WithPrincipal(r.Context(), middleware.Principal{Username: username, SessionID: session.ID, CSRF: session.CSRFSecret})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func tokenOK(got, want string) bool {
	if len(got) == 0 || len(got) != len(want) {
		return false
	}
	return subtleConstant(got, want)
}
