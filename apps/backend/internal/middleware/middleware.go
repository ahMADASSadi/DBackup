// Package middleware applies the HTTP controls shared by every route.
package middleware

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"
)

type principalKey struct{}

// Principal is the authenticated administrator.
type Principal struct {
	Username  string
	SessionID string
	CSRF      string
}

// WithPrincipal stores the administrator on the request context.
func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, principal)
}

// PrincipalFrom returns the administrator, if the request was authenticated.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalKey{}).(Principal)
	return principal, ok
}

// Secure wraps a handler with recovery, logging, request limits, and baseline headers.
func Secure(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Error("panic", "path", r.URL.Path)
				writeError(rec, http.StatusInternalServerError, "internal", "internal error")
			}
		}()
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		start := time.Now()
		next.ServeHTTP(rec, r)
		log.Info("request", "method", r.Method, "path", r.URL.Path, "status", rec.status, "duration_ms", time.Since(start).Milliseconds())
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(status int) {
	s.status = status
	s.ResponseWriter.WriteHeader(status)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": message}})
}

// ClientIP returns the caller address, using the proxy-appended address when the peer is trusted.
func ClientIP(r *http.Request, trusted []*net.IPNet) string {
	host := r.RemoteAddr
	if parsedHost, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		host = parsedHost
	}
	peer := net.ParseIP(host)
	if peer != nil && containsIP(trusted, peer) {
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			parts := strings.Split(forwarded, ",")
			candidate := strings.TrimSpace(parts[len(parts)-1])
			if net.ParseIP(candidate) != nil {
				return candidate
			}
		}
	}
	return host
}

func containsIP(nets []*net.IPNet, ip net.IP) bool {
	for _, network := range nets {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// ParseCIDRs parses CIDRs and single IP addresses.
func ParseCIDRs(values []string) ([]*net.IPNet, error) {
	var out []*net.IPNet
	for _, value := range values {
		if strings.Contains(value, "/") {
			_, network, err := net.ParseCIDR(value)
			if err != nil {
				return nil, err
			}
			out = append(out, network)
			continue
		}
		addr, err := netip.ParseAddr(value)
		if err != nil {
			return nil, err
		}
		bits := 32
		if addr.Is6() {
			bits = 128
		}
		out = append(out, &net.IPNet{IP: net.ParseIP(value), Mask: net.CIDRMask(bits, bits)})
	}
	return out, nil
}
