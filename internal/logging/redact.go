package logging

import (
	"context"
	"log/slog"
	"regexp"
	"strings"
)

const masked = "[REDACTED]"

var sensitivePattern = regexp.MustCompile(`(?i)(["']?(?:api[-_ ]?key|secret|signature|x-mbx-apikey)["']?\s*(?:[=:]|%3[dD])\s*["']?)([^"'&,\s;}]+)`)

func Redact(value string) string {
	return sensitivePattern.ReplaceAllString(value, "$1"+masked)
}

type RedactingHandler struct{ next slog.Handler }

func NewRedactingHandler(next slog.Handler) *RedactingHandler { return &RedactingHandler{next: next} }

func (h *RedactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *RedactingHandler) Handle(ctx context.Context, record slog.Record) error {
	clean := slog.NewRecord(record.Time, record.Level, Redact(record.Message), record.PC)
	record.Attrs(func(attr slog.Attr) bool { clean.AddAttrs(redactAttr(attr)); return true })
	return h.next.Handle(ctx, clean)
}

func (h *RedactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clean := make([]slog.Attr, len(attrs))
	for i, attr := range attrs {
		clean[i] = redactAttr(attr)
	}
	return &RedactingHandler{next: h.next.WithAttrs(clean)}
}

func (h *RedactingHandler) WithGroup(name string) slog.Handler {
	return &RedactingHandler{next: h.next.WithGroup(name)}
}

func redactAttr(attr slog.Attr) slog.Attr {
	if attr.Value.Kind() == slog.KindGroup {
		group := attr.Value.Group()
		for i := range group {
			group[i] = redactAttr(group[i])
		}
		return slog.Group(attr.Key, attrsToAny(group)...)
	}
	key := strings.ToLower(attr.Key)
	if strings.Contains(key, "secret") || strings.Contains(key, "signature") || strings.Contains(key, "api_key") || strings.Contains(key, "apikey") {
		return slog.String(attr.Key, masked)
	}
	if attr.Value.Kind() == slog.KindString {
		return slog.String(attr.Key, Redact(attr.Value.String()))
	}
	return attr
}

func attrsToAny(attrs []slog.Attr) []any {
	values := make([]any, len(attrs))
	for i := range attrs {
		values[i] = attrs[i]
	}
	return values
}
