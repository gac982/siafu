// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/gac982/siafu/internal/results"
)

func TestUIHistorySanitizesNames(t *testing.T) {
	deps := testDeps(t)
	seedHistoryRun(t, deps.resultsDir, "\x1b]0;evil\x07demo", "closed", "PASS")

	m := openMain(t, newApp(deps), "Ver corridas anteriores")
	view := m.View().Content
	if strings.Contains(view, "\x1b]0;") || strings.ContainsRune(view, 0x07) {
		t.Errorf("vista con controles: %q", view)
	}
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	view = m.View().Content
	if strings.Contains(view, "\x1b]0;") || strings.ContainsRune(view, 0x07) {
		t.Errorf("detalle con controles: %q", view)
	}
}

func TestUIRunSanitizesSummary(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "evil.yaml")
	data := `version: 1
name: "evil \u001b[31mred"
target:
  base_url: "http://127.0.0.1:1"
  timeout: 1s
load:
  stages:
    - {duration: 1s, target_vus: 1}
scenarios:
  - name: home
    steps:
      - name: portada
        request: {method: GET, url: /}
`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	deps := testDeps(t)
	deps.candidates = func() []string { return []string{path} }
	m := openMain(t, newApp(deps), "Ejecutar una prueba")
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if view := m.View().Content; strings.Contains(view, "\x1b[31mred") {
		t.Errorf("resumen con ESC: %q", view)
	}
}

func TestPrintRunsListSanitizesName(t *testing.T) {
	var out bytes.Buffer
	printRunsList(&out, []results.IndexEntry{{
		RunID: "20260101-000000-evil", Name: "\x1b]0;evil\x07demo",
		Mode: "closed", Verdict: "PASS",
	}})
	if strings.ContainsRune(out.String(), 0x1b) || strings.ContainsRune(out.String(), 0x07) {
		t.Errorf("lista con controles: %q", out.String())
	}
}

func TestRunRunsShowSanitizesSummary(t *testing.T) {
	dir := t.TempDir()
	store, err := results.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := store.Save(results.RunRecord{Name: "evil", Mode: "closed", Verdict: "PASS"},
		[]results.SeriesPoint{{TMS: 1000, RPS: 1}}, "resumen \x1b]0;evil\x07\n")
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := runRunsShow([]string{rec.RunID, "--results-dir", dir}, &out, &errOut); code != 0 {
		t.Fatalf("code = %d (%s)", code, errOut.String())
	}
	if strings.ContainsRune(out.String(), 0x1b) || strings.ContainsRune(out.String(), 0x07) {
		t.Errorf("summary con controles: %q", out.String())
	}
}
