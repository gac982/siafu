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
)

func TestUIHistoryListShowsRuns(t *testing.T) {
	deps := testDeps(t)
	seedHistoryRun(t, deps.resultsDir, "aprobada", "closed", "PASS")
	seedHistoryRun(t, deps.resultsDir, "fallida", "open", "FAIL")

	m := openMain(t, newApp(deps), "Ver corridas anteriores")
	view := m.View().Content
	for _, want := range []string{"APROBADA", "FALLIDA", "aprobada", "fallida", "Página 1/1"} {
		if !strings.Contains(view, want) {
			t.Errorf("vista sin %q:\n%s", want, view)
		}
	}
}

func TestUIHistoryDetail(t *testing.T) {
	deps := testDeps(t)
	entry := seedHistoryRun(t, deps.resultsDir, "detalle", "closed", "PASS")

	m := openMain(t, newApp(deps), "Ver corridas anteriores")
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if view := m.View().Content; !strings.Contains(view, "summary "+entry.Name) {
		t.Errorf("vista = %q", view)
	}
}

func TestUIHistoryFilter(t *testing.T) {
	deps := testDeps(t)
	seedHistoryRun(t, deps.resultsDir, "aprobada", "closed", "PASS")
	seedHistoryRun(t, deps.resultsDir, "fallida", "open", "FAIL")

	m := openMain(t, newApp(deps), "Ver corridas anteriores")
	m = updateApp(t, m, tea.KeyPressMsg{Code: 'f'})
	filter, ok := m.stack[len(m.stack)-1].(*historyFilterScreen)
	if !ok {
		t.Fatalf("pantalla = %T; quiero *historyFilterScreen", m.stack[len(m.stack)-1])
	}
	filter.mode.cursor = 2    // abierto
	filter.verdict.cursor = 2 // FALLIDA
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})

	view := m.View().Content
	if !strings.Contains(view, "fallida") || strings.Contains(view, "aprobada") {
		t.Errorf("vista filtrada = %q", view)
	}
}

func TestUIHistoryFilterNoMatches(t *testing.T) {
	deps := testDeps(t)
	seedHistoryRun(t, deps.resultsDir, "solo-cerrada", "closed", "PASS")

	m := openMain(t, newApp(deps), "Ver corridas anteriores")
	m = updateApp(t, m, tea.KeyPressMsg{Code: 'f'})
	filter := m.stack[len(m.stack)-1].(*historyFilterScreen)
	filter.mode.cursor = 2
	filter.verdict.cursor = 2
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})

	if view := m.View().Content; !strings.Contains(view, "No hay corridas que coincidan") {
		t.Errorf("vista = %q", view)
	}
	m = updateApp(t, m, tea.KeyPressMsg{Code: 'f'})
	filter = m.stack[len(m.stack)-1].(*historyFilterScreen)
	filter.mode.cursor = 0
	filter.verdict.cursor = 0
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if view := m.View().Content; !strings.Contains(view, "solo-cerrada") {
		t.Errorf("vista tras quitar filtro = %q", view)
	}
}

func TestUIHistoryEmpty(t *testing.T) {
	deps := testDeps(t)
	m := openMain(t, newApp(deps), "Ver corridas anteriores")
	if view := m.View().Content; !strings.Contains(view, "No hay corridas") {
		t.Errorf("vista = %q", view)
	}
}

func TestUIHistoryCorruptRun(t *testing.T) {
	deps := testDeps(t)
	seedHistoryRun(t, deps.resultsDir, "buena", "closed", "PASS")
	bad := filepath.Join(deps.resultsDir, "bad")
	if err := os.Mkdir(bad, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bad, "run.json"), []byte("{no-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	var errOut bytes.Buffer
	deps.errOut = &errOut

	m := openMain(t, newApp(deps), "Ver corridas anteriores")
	if !strings.Contains(errOut.String(), "aviso:") {
		t.Errorf("stderr = %q", errOut.String())
	}
	if view := m.View().Content; !strings.Contains(view, "buena") {
		t.Errorf("vista = %q", view)
	}
}

func TestUIHistoryCompare(t *testing.T) {
	deps := testDeps(t)
	seedHistoryRun(t, deps.resultsDir, "primera", "closed", "PASS")
	seedHistoryRun(t, deps.resultsDir, "segunda", "open", "FAIL")

	m := openMain(t, newApp(deps), "Ver corridas anteriores")
	m = updateApp(t, m, tea.KeyPressMsg{Code: 'c'})
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})

	if view := m.View().Content; !strings.Contains(view, "A:") || !strings.Contains(view, "B:") {
		t.Errorf("vista = %q", view)
	}
}

func TestUIHistoryPagination(t *testing.T) {
	deps := testDeps(t)
	for i := 0; i < 12; i++ {
		seedHistoryRun(t, deps.resultsDir, "corrida", "closed", "PASS")
	}
	m := openMain(t, newApp(deps), "Ver corridas anteriores")
	if view := m.View().Content; !strings.Contains(view, "Página 1/2") {
		t.Errorf("vista página 1 = %q", view)
	}
	m = updateApp(t, m, tea.KeyPressMsg{Code: 'n', Text: "n"})
	if view := m.View().Content; !strings.Contains(view, "Página 2/2") {
		t.Errorf("vista página 2 = %q", view)
	}
}
