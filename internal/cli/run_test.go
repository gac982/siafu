// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package cli

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gac982/siafu/internal/results"
)

func TestRunRunReportKeepsExitCode(t *testing.T) {
	rep, code := runRunReport([]string{"run"}, io.Discard, io.Discard)
	if code != 2 {
		t.Fatalf("code = %d; quiero 2", code)
	}
	if rep.Verdict != "" {
		t.Errorf("verdict = %q; quiero vacío", rep.Verdict)
	}
}

func runConfigYAML(base string, minRPS float64) string {
	return fmt.Sprintf(`
version: 1
name: cli-run
target:
  base_url: %q
  timeout: 1s
load:
  stages:
    - duration: 200ms
      target_vus: 1
    - duration: 300ms
      target_vus: 1
scenarios:
  - name: s
    steps:
      - name: one
        request:
          method: GET
          url: /
        expect:
          status: 200
thresholds:
  latency_p95: 1s
  error_rate: 0.05
  min_rps: %v
`, base, minRPS)
}

func writeRunConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "run.yaml")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunCommandSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer srv.Close()
	path := writeRunConfig(t, runConfigYAML(srv.URL, 1))

	var stdout, stderr bytes.Buffer
	code := Run([]string{"run", "--no-tui", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d (stderr: %s)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "APROBADA") {
		t.Errorf("stdout does not contain APROBADA: %q", stdout.String())
	}
}

func TestRunCommandThresholdFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer srv.Close()
	path := writeRunConfig(t, runConfigYAML(srv.URL, 100000))

	var stdout, stderr bytes.Buffer
	code := Run([]string{"run", "--no-tui", path}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (stderr: %s)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "FALLIDA") {
		t.Errorf("stdout does not contain FALLIDA")
	}
}

func TestRunCommandFlagsAfterPositional(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer srv.Close()
	path := writeRunConfig(t, runConfigYAML(srv.URL, 1))

	var stdout, stderr bytes.Buffer
	code := Run([]string{"run", path, "--no-tui"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr.String())
	}
}

func TestRunCommandUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"run"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "uso") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestRunCommandConfigError(t *testing.T) {
	path := writeRunConfig(t, "version: 2\nname: bad\n")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"run", path}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "version") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

const admissionRejectedYAML = `
version: 1
name: admission
target:
  base_url: "http://127.0.0.1:1"
load:
  max_vus: 5000
  stages:
    - duration: 1s
      target_vus: 5000
limits:
  memory_budget_mb: 100
scenarios:
  - name: s
    steps:
      - name: one
        request: {method: GET, url: /}
`

func TestRunCommandAdmissionRejected(t *testing.T) {
	path := writeRunConfig(t, admissionRejectedYAML)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"run", "--no-tui", path}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	for _, want := range []string{"5 000", "1 392", "No se ejecutó ninguna petición"} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("stderr = %q, want %q", stderr.String(), want)
		}
	}
}

func TestValidateCommandAdmissionRejected(t *testing.T) {
	path := writeRunConfig(t, admissionRejectedYAML)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"validate", path}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "No se ejecutó ninguna petición") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

const openLoopRejectedYAML = `
version: 1
name: open-admission
target:
  base_url: "http://127.0.0.1:1"
load:
  mode: open
  open_loop:
    stages:
      - duration: 1s
        target_rate: 1000
    max_in_flight: 15000
limits:
  memory_budget_mb: 300
scenarios:
  - name: s
    steps:
      - name: one
        request: {method: GET, url: /}
`

func TestRunCommandOpenLoopAdmissionRejected(t *testing.T) {
	path := writeRunConfig(t, openLoopRejectedYAML)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"run", "--no-tui", path}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	for _, want := range []string{"open_loop.max_in_flight", "15 000", "4 989", "No se ejecutó ninguna petición"} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("stderr = %q, want %q", stderr.String(), want)
		}
	}
}

func TestRunCommandSavesResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer srv.Close()

	path := writeRunConfig(t, runConfigYAML(srv.URL, 1))
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := Run([]string{"run", "--no-tui", "--results-dir", dir, "--tag", "smoke", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d (stderr: %s)", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "resultados guardados en") {
		t.Errorf("stderr = %q", stderr.String())
	}
	store, err := results.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	list, err := store.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %+v, err = %v", list, err)
	}
	if list[0].Tag != "smoke" || list[0].Mode != "closed" {
		t.Errorf("entry = %+v", list[0])
	}
}

func TestRunCommandNoSave(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer srv.Close()

	path := writeRunConfig(t, runConfigYAML(srv.URL, 1))
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := Run([]string{"run", "--no-tui", "--no-save", "--results-dir", dir, path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d (stderr: %s)", code, stderr.String())
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("results dir is not empty: %+v", entries)
	}
}

func TestRunCommandFlagsAfterPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer srv.Close()

	path := writeRunConfig(t, runConfigYAML(srv.URL, 1))
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := Run([]string{"run", path, "--no-tui", "--results-dir", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d (stderr: %s)", code, stderr.String())
	}
	store, err := results.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	list, err := store.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %+v, err = %v", list, err)
	}
}
