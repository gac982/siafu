// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestLogoRendersColorAndPlain(t *testing.T) {
	colored := Logo(true)
	if !strings.Contains(colored, "\x1b[38;2;") {
		t.Fatal("Logo(true) should contain truecolor escapes")
	}
	plain := Logo(false)
	if strings.Contains(plain, "\x1b[") {
		t.Fatal("Logo(false) should not contain ANSI escapes")
	}

	lines := strings.Split(plain, "\n")
	if len(lines) < 10 || len(lines) > 32 {
		t.Errorf("logo lines = %d; want 10..32", len(lines))
	}
	for i, line := range lines {
		if width := lipgloss.Width(line); width > 44 {
			t.Errorf("line %d width = %d; want <= 44", i, width)
		}
	}
}

func TestLogoMarkFitsMenu(t *testing.T) {
	colored := LogoMark(true)
	if !strings.Contains(colored, "\x1b[38;2;") {
		t.Fatal("LogoMark(true) should contain truecolor escapes")
	}
	plain := LogoMark(false)
	if strings.Contains(plain, "\x1b[") {
		t.Fatal("LogoMark(false) should not contain ANSI escapes")
	}

	lines := strings.Split(plain, "\n")
	if len(lines) > 12 {
		t.Errorf("mark lines = %d; want <= 12", len(lines))
	}
	for i, line := range lines {
		if width := lipgloss.Width(line); width > 24 {
			t.Errorf("line %d width = %d; want <= 16", i, width)
		}
	}
}

func TestColorEnabledHonorsEnvironment(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("TERM", "xterm-256color")
	if ColorEnabled() {
		t.Error("ColorEnabled() = true with NO_COLOR set")
	}

	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "dumb")
	if ColorEnabled() {
		t.Error("ColorEnabled() = true with TERM=dumb")
	}

	t.Setenv("TERM", "xterm-256color")
	if !ColorEnabled() {
		t.Error("ColorEnabled() = false in a color terminal")
	}
}
