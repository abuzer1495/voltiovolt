// SPDX-License-Identifier: AGPL-3.0-or-later

package redact

import (
	"context"
	"fmt"
	"log/slog"
)

// Handler is a log/slog handler. It replaces each attribute of the Secret
// type with "[REDACTED]" and removes the Scrub patterns from the message and
// from all text values. Then it gives the record to the next handler.
type Handler struct {
	next slog.Handler
}

// NewHandler returns a Handler that writes to next.
func NewHandler(next slog.Handler) *Handler {
	return &Handler{next: next}
}

// Enabled implements slog.Handler.
func (h *Handler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.next.Enabled(ctx, l)
}

// Handle implements slog.Handler.
func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	out := slog.NewRecord(r.Time, r.Level, Scrub(r.Message), r.PC)
	r.Attrs(func(a slog.Attr) bool {
		out.AddAttrs(cleanAttr(a))
		return true
	})
	return h.next.Handle(ctx, out)
}

// WithAttrs implements slog.Handler.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	c := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		c[i] = cleanAttr(a)
	}
	return &Handler{next: h.next.WithAttrs(c)}
}

// WithGroup implements slog.Handler.
func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{next: h.next.WithGroup(name)}
}

func cleanAttr(a slog.Attr) slog.Attr {
	a.Key = Scrub(a.Key)
	a.Value = cleanValue(a.Value, 0)
	return a
}

func cleanValue(v slog.Value, depth int) slog.Value {
	// Check the Secret type before LogValuer resolution.
	if v.Kind() == slog.KindAny {
		switch v.Any().(type) {
		case Secret, *Secret:
			return slog.StringValue(Redacted)
		}
	}
	if v.Kind() == slog.KindLogValuer {
		if _, ok := v.Any().(Secret); ok {
			return slog.StringValue(Redacted)
		}
		if depth > 8 {
			return slog.StringValue(Redacted)
		}
		return cleanValue(v.Resolve(), depth+1)
	}
	switch v.Kind() {
	case slog.KindString:
		return slog.StringValue(Scrub(v.String()))
	case slog.KindGroup:
		g := v.Group()
		c := make([]slog.Attr, len(g))
		for i, a := range g {
			c[i] = slog.Attr{Key: Scrub(a.Key), Value: cleanValue(a.Value, depth+1)}
		}
		return slog.GroupValue(c...)
	case slog.KindAny:
		switch x := v.Any().(type) {
		case error:
			return slog.StringValue(Scrub(x.Error()))
		case fmt.Stringer:
			return slog.StringValue(Scrub(x.String()))
		default:
			// The text form goes through fmt, which calls the Secret
			// methods for nested Secret values.
			return slog.StringValue(Scrub(fmt.Sprintf("%+v", x)))
		}
	default:
		return v
	}
}
