// Package ui provides the terminal presentation layer for the builder:
// structured logging and progress rendering to a single output stream.
package ui

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"bgscan-builder/internal/downloader"

	"github.com/fatih/color"
)

// UI is the single presentation surface for the whole process. It owns the
// logger, the progress renderer, and every pretty state transition.
type UI struct {
	w     io.Writer
	fancy bool

	logger *slog.Logger
	level  *slog.LevelVar
}

// New creates a UI bound to w (usually os.Stderr). When w is a real terminal
// the UI enables progress rendering and ANSI colour; otherwise it degrades to
// clean, log-friendly plain text (ideal for CI and pipes).
func New(w io.Writer) *UI {
	if w == nil {
		w = os.Stderr
	}

	u := &UI{w: w}
	u.fancy = isTerminal(w)
	color.NoColor = !u.fancy

	u.level = new(slog.LevelVar)
	u.level.Set(slog.LevelInfo)
	u.logger = slog.New(newConsoleHandler(u.w, u.level))
	return u
}

// Fancy reports whether the live UI (progress + colour) is enabled.
func (u *UI) Fancy() bool { return u.fancy }

// Logger returns the process-wide structured logger.
func (u *UI) Logger() *slog.Logger { return u.logger }

// SetLevel adjusts the minimum log level that gets emitted (e.g. Debug).
func (u *UI) SetLevel(l slog.Level) { u.level.Set(l) }

// ProgressSink exposes the progress renderer to consumers such as the
// downloader. It is nil when the UI runs in plain (non-terminal) mode.
func (u *UI) ProgressSink() downloader.ProgressSink {
	if !u.fancy {
		return nil
	}
	return &progressSink{w: u.w}
}

// brandArt is the compact terminal wordmark shown at process start.
var brandArt = []string{
	" ██████╗  ██████╗    ███████╗ ██████╗ █████╗ ███╗   ██╗",
	" ██╔══██╗██╔════╝    ██╔════╝██╔════╝██╔══██╗████╗  ██║",
	" ██████╔╝██║  ███╗   ███████╗██║     ███████║██╔██╗ ██║",
	" ██╔══██╗██║   ██║   ╚════██║██║     ██╔══██║██║╚██╗██║",
	" ██████╔╝╚██████╔╝   ███████║╚██████╗██║  ██║██║ ╚████║",
	" ╚═════╝  ╚═════╝    ╚══════╝ ╚═════╝╚═╝  ╚═╝╚═╝  ╚═══╝",
}

// Brand renders the identity block and a quiet tagline.
func (u *UI) Brand(version, subtitle string) {
	u.writeLine("")
	for _, line := range brandArt {
		u.writeLine(cCyan.Sprint(line))
	}
	u.Muted(fmt.Sprintf("  bgscan-builder · %s · %s", subtitle, version))
	u.writeLine("")
}

// Section opens a new stage of the flow.
func (u *UI) Section(title string) {
	u.writeLine("")
	u.writeLine(fmt.Sprintf("%s %s", cCyan.Sprint("→"), cBold.Sprint(title)))
}

// Row renders a dim label with a bold value in a fixed-width column.
func (u *UI) Row(label, value string) {
	u.writeLine(fmt.Sprintf("   %s%s", cDim.Sprintf("%-16s", label), cBold.Sprint(value)))
}

// Muted renders a dim, secondary information line.
func (u *UI) Muted(msg string) { u.writeLine(cDim.Sprint(msg)) }

// Summary renders a terminal status table.
func (u *UI) Summary(title string, rows []Row) {
	if len(rows) == 0 {
		return
	}

	width := 0
	for _, r := range rows {
		if len(r.Target) > width {
			width = len(r.Target)
		}
	}
	if width < len(title)+2 {
		width = len(title) + 2
	}

	u.writeLine("")
	u.writeLine(cDim.Sprintf("── %s %s", strings.ToUpper(title), strings.Repeat("─", 4)))
	for _, r := range rows {
		mark := "●"
		status := cDim.Sprint(r.Status)
		if r.OK {
			mark = cGreen.Sprint("✓")
			status = cGreen.Sprint(r.Status)
		}
		u.writeLine(fmt.Sprintf("  %-*s  %s  %s", width, r.Target, mark, status))
	}
	u.writeLine(cDim.Sprintf("── %s", strings.Repeat("─", 4)))
}

// Row describes one summary entry.
type Row struct {
	Target string
	Status string
	OK     bool
}

// Info logs a structured informational message.
func (u *UI) Info(msg string, args ...any) { u.logger.Info(msg, args...) }

// Debug logs a structured debug message (hidden unless verbose).
func (u *UI) Debug(msg string, args ...any) { u.logger.Debug(msg, args...) }

// Warn logs a structured warning message.
func (u *UI) Warn(msg string, args ...any) { u.logger.Warn(msg, args...) }

// Error logs a structured error message.
func (u *UI) Error(msg string, args ...any) { u.logger.Error(msg, args...) }

// Success renders a completed-stage marker.
func (u *UI) Success(msg string) {
	u.writeLine(fmt.Sprintf("%s %s", cGreen.Sprint("✓"), msg))
}

// Fail renders a generic fatal error marker and message.
func (u *UI) Fail(msg string) {
	u.writeLine(fmt.Sprintf("%s %s", cRed.Sprint("✗"), msg))
}

// writeLine writes a single, complete log line.
func (u *UI) writeLine(l string) {
	_, _ = io.WriteString(u.w, l+"\n")
}

func (u *UI) write(l string) {
	_, _ = io.WriteString(u.w, l)
}

// isTerminal reports whether w is a character device (a real terminal).
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	st, err := f.Stat()
	if err != nil {
		return false
	}
	return st.Mode()&os.ModeCharDevice != 0
}
