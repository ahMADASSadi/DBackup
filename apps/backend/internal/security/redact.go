package security

import (
	"context"
	"log/slog"
	"regexp"
	"strings"
)

var secretPattern = regexp.MustCompile(`(?i)(password|passwd|pwd|secret|token|authorization|access_key|secret_key)\s*[:=]\s*([^\s&",]+)`)

// Redact removes known secret values and common credential assignments.
func Redact(message string, secrets ...string) string {
	for _, secret := range secrets {
		if len(secret) < 3 {
			continue
		}
		message = strings.ReplaceAll(message, secret, "[redacted]")
	}
	return secretPattern.ReplaceAllString(message, "$1=[redacted]")
}

// Truncate keeps error text short enough for the metadata database and UI.
func Truncate(message string, limit int) string {
	message = strings.TrimSpace(message)
	if limit <= 0 || len(message) <= limit {
		return message
	}
	return message[:limit] + "…"
}

// RedactHandler strips secrets from slog records.
type RedactHandler struct {
	next slog.Handler
}

// NewRedactHandler wraps next.
func NewRedactHandler(next slog.Handler) slog.Handler {
	return RedactHandler{next: next}
}

func (h RedactHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h RedactHandler) Handle(ctx context.Context, record slog.Record) error {
	record.Message = Redact(record.Message)
	attrs := make([]slog.Attr, 0, record.NumAttrs())
	record.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, redactAttr(a))
		return true
	})
	clone := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	clone.AddAttrs(attrs...)
	return h.next.Handle(ctx, clone)
}

func (h RedactHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	cleaned := make([]slog.Attr, len(attrs))
	for i, attr := range attrs {
		cleaned[i] = redactAttr(attr)
	}
	return RedactHandler{next: h.next.WithAttrs(cleaned)}
}

func (h RedactHandler) WithGroup(name string) slog.Handler {
	return RedactHandler{next: h.next.WithGroup(name)}
}

func redactAttr(attr slog.Attr) slog.Attr {
	key := strings.ToLower(attr.Key)
	switch key {
	case "password", "secret", "token", "authorization", "access_key", "secret_key", "cookie", "csrf", "encryption_key":
		attr.Value = slog.StringValue("[redacted]")
		return attr
	}
	if attr.Value.Kind() == slog.KindString {
		attr.Value = slog.StringValue(Redact(attr.Value.String()))
	}
	return attr
}
