// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package logbuf

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gac982/siafu/internal/redact"
)

func TestRingCapacity(t *testing.T) {
	b := New(3)
	for _, msg := range []string{"one", "two", "three", "four", "five"} {
		b.Info(msg)
	}
	lines := b.Lines()
	if len(lines) != 3 {
		t.Fatalf("len = %d, want 3", len(lines))
	}
	if !strings.Contains(lines[2], "five") {
		t.Errorf("last line = %q", lines[2])
	}
}

func TestErrorRateLimit(t *testing.T) {
	b := New(100)
	for i := 0; i < 10; i++ {
		b.Error("boom")
	}
	lines := b.Lines()
	if len(lines) != 5 {
		t.Fatalf("error lines = %d, want 5 per second", len(lines))
	}
}

func TestRedactsSecrets(t *testing.T) {
	var out bytes.Buffer
	b := New(10)
	b.SetPlain(&out, redact.New([]string{"supersecret"}))
	b.Error("token=supersecret")

	if strings.Contains(out.String(), "supersecret") {
		t.Errorf("plain output leaks secret: %q", out.String())
	}
	if !strings.Contains(out.String(), "***") {
		t.Errorf("plain output does not contain mask: %q", out.String())
	}
	lines := b.Lines()
	if len(lines) != 1 || strings.Contains(lines[0], "supersecret") {
		t.Errorf("buffer leaks secret: %+v", lines)
	}
}

func TestLinesReturnsCopy(t *testing.T) {
	b := New(10)
	b.Info("one")
	lines := b.Lines()
	lines[0] = "mutated"
	if b.Lines()[0] == "mutated" {
		t.Error("Lines() exposed internal storage")
	}
}

func TestRedactsWithoutPlainWriter(t *testing.T) {
	b := New(10)
	b.SetRedactor(redact.New([]string{"supersecret"}))
	b.Error("token=supersecret")
	lines := b.Lines()
	if len(lines) != 1 || strings.Contains(lines[0], "supersecret") {
		t.Fatalf("TUI buffer leaks secret: %+v", lines)
	}
}

func TestRingWrapOrder(t *testing.T) {
	b := New(3)
	for _, msg := range []string{"one", "two", "three", "four"} {
		b.Info(msg)
	}
	lines := b.Lines()
	if len(lines) != 3 {
		t.Fatalf("len = %d", len(lines))
	}
	for i, want := range []string{"two", "three", "four"} {
		if !strings.Contains(lines[i], want) {
			t.Errorf("line %d = %q, want %q", i, lines[i], want)
		}
	}
}

func TestPlainSanitizesTerminalControls(t *testing.T) {
	var out bytes.Buffer
	b := New(10)
	b.SetPlain(&out, nil)
	b.Info("\x1b]0;evil\x07line")
	if strings.ContainsRune(out.String(), 0x1b) || strings.ContainsRune(out.String(), 0x07) {
		t.Errorf("plain con controles: %q", out.String())
	}
}
