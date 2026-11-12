// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionPrintsPlainLogoWhenNotTTY(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runWithIO([]string{"version"}, strings.NewReader(""), &stdout, &stderr, false)
	if code != 0 {
		t.Fatalf("code = %d; stderr = %s", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "\x1b[") {
		t.Error("version output should not contain ANSI escapes without a TTY")
	}
	if !strings.Contains(stdout.String(), "siafu") {
		t.Errorf("version output = %q", stdout.String())
	}
}

func TestVersionPrintsColorLogoWhenTTY(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "xterm-256color")
	var stdout, stderr bytes.Buffer
	code := runWithIO([]string{"version"}, strings.NewReader(""), &stdout, &stderr, true)
	if code != 0 {
		t.Fatalf("code = %d; stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "\x1b[38;2;") {
		t.Error("version output should contain truecolor logo on a TTY")
	}
}
