package ui

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

// consoleHandler renders slog records as compact, human-friendly lines:
//
//	INFO  downloading Xray Core  version=v26.7.28  target=linux-amd64
type consoleHandler struct {
	w     io.Writer
	level *slog.LevelVar
	group string
	attrs []slog.Attr
}

func newConsoleHandler(w io.Writer, lv *slog.LevelVar) *consoleHandler {
	return &consoleHandler{w: w, level: lv}
}

func (h *consoleHandler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.level.Level()
}

func (h *consoleHandler) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder

	switch {
	case r.Level >= slog.LevelError:
		b.WriteString(cRed.Sprint("ERROR "))
	case r.Level >= slog.LevelWarn:
		b.WriteString(cYellow.Sprint("WARN "))
	case r.Level >= slog.LevelInfo:
		b.WriteString(cCyan.Sprint("INFO "))
	default:
		b.WriteString(cDim.Sprint("DEBUG "))
	}

	b.WriteString("  ")
	b.WriteString(r.Message)

	for _, a := range appendAttrs(h, r) {
		b.WriteString("  ")
		b.WriteString(cDim.Sprintf("%s=%v", a.Key, a.Value.Any()))
	}

	_, err := io.WriteString(h.w, b.String()+"\n")
	return err
}

func (h *consoleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	merged := make([]slog.Attr, 0, len(h.attrs)+len(attrs))
	merged = append(merged, h.attrs...)
	merged = append(merged, attrs...)
	return &consoleHandler{w: h.w, level: h.level, group: h.group, attrs: merged}
}

func (h *consoleHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	if h.group != "" {
		name = h.group + "." + name
	}
	return &consoleHandler{w: h.w, level: h.level, group: name, attrs: h.attrs}
}

// appendAttrs merges handler-level attributes with the record's own,
// applying any group prefix.
func appendAttrs(h *consoleHandler, r slog.Record) []slog.Attr {
	out := make([]slog.Attr, 0, len(h.attrs)+r.NumAttrs())
	for _, a := range h.attrs {
		out = append(out, prefixGroup(h.group, a))
	}
	r.Attrs(func(a slog.Attr) bool {
		out = append(out, prefixGroup(h.group, a))
		return true
	})
	return out
}

func prefixGroup(group string, a slog.Attr) slog.Attr {
	if group == "" || a.Equal(slog.Attr{}) {
		return a
	}
	return slog.Attr{Key: fmt.Sprintf("%s.%s", group, a.Key), Value: a.Value}
}
