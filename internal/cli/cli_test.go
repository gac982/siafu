// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gac982/siafu/internal/version"
)

func TestVersionCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run([]string{"version"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr.String())
	}
	for _, want := range []string{"siafu", version.Version, runtime.GOOS, runtime.GOARCH} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("version output %q does not contain %q", stdout.String(), want)
		}
	}
}

func TestUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run([]string{"nope"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "uso") {
		t.Errorf("stderr = %q, want usage text", stderr.String())
	}
}

func TestValidateCommandOK(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ok.yaml")
	writeTestConfig(t, path, `
version: 1
name: cli
target: {base_url: "http://localhost:8080"}
load: {stages: [{duration: 1s, target_vus: 1}]}
scenarios: [{name: s, steps: [{name: one, request: {url: /}}]}]
`)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"validate", path}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr.String())
	}
	for _, want := range []string{"cli", "1 escenario", "sin umbrales"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("stdout %q does not contain %q", stdout.String(), want)
		}
	}
}

func TestValidateCommandInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	writeTestConfig(t, path, "version: 2\nname: bad\n")

	var stdout, stderr bytes.Buffer
	code := Run([]string{"validate", path}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "version") {
		t.Errorf("stderr %q does not name the problem", stderr.String())
	}
}

func TestValidateCommandUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"validate"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "uso") {
		t.Errorf("stderr %q does not contain usage", stderr.String())
	}
}

func writeTestConfig(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestHelpCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run([]string{"--help"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	for _, want := range []string{"run", "validate", "version"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("help output %q does not contain %q", stdout.String(), want)
		}
	}
}
