package ui

import (
	"bytes"
	"strings"
	"testing"
)

func TestNew_PlainMode(t *testing.T) {
	var buf bytes.Buffer
	u := New(&buf)

	if u.Fancy() {
		t.Fatal("expected plain mode for a non-terminal writer")
	}
	if u.ProgressSink() != nil {
		t.Fatal("expected nil progress sink in plain mode")
	}

	u.Info("downloading xray", "version", "v1.0")
	u.Success("xray staged")
	u.Warn("retry", "attempt", 2)
	u.Summary("summary", []Row{{Target: "linux-amd64", Status: "done", OK: true}})
	u.Fail("boom")

	out := buf.String()
	for _, want := range []string{
		"INFO   downloading xray",
		"version=v1.0",
		"✓ xray staged",
		"WARN   retry",
		"attempt=2",
		"SUMMARY",
		"linux-amd64",
		"✓",
		"✗ boom",
	} {
		if !strings.Contains(out, want) {
			got := strings.ReplaceAll(out, "\n", " | ")
			t.Errorf("plain output missing %q; got: %s", want, got)
		}
	}
}

func TestNew_BrandAndSection(t *testing.T) {
	var buf bytes.Buffer
	u := New(&buf)

	u.Brand("v1.0.0", "installer")
	u.Section("System")
	u.Row("OS", "linux")
	u.Success("System ready")

	out := buf.String()
	for _, want := range []string{
		"██████╗",
		"bgscan-builder · installer · v1.0.0",
		"→ System",
		"OS",
		"linux",
		"✓ System ready",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q: %q", want, out)
		}
	}
}

func TestSetLevel_DebugHiddenByDefault(t *testing.T) {
	var buf bytes.Buffer
	u := New(&buf)

	u.Debug("verbose detail")
	if strings.Contains(buf.String(), "verbose detail") {
		t.Fatal("debug output should be hidden by default")
	}

	u.SetLevel(-4)
	u.Debug("verbose detail")
	if !strings.Contains(buf.String(), "verbose detail") {
		t.Fatal("debug output should appear after SetLevel(Debug)")
	}
}
