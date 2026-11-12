// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/gac982/siafu/internal/results"
)

func TestUIPrunePreviewAndConfirm(t *testing.T) {
	deps := testDeps(t)
	seedHistoryRun(t, deps.resultsDir, "uno", "closed", "PASS")
	seedHistoryRun(t, deps.resultsDir, "dos", "closed", "PASS")
	seedHistoryRun(t, deps.resultsDir, "tres", "closed", "PASS")

	m := openMain(t, newApp(deps), "Mantenimiento")
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEnter}) // prune
	prune := m.stack[len(m.stack)-1].(*pruneScreen)
	prune.keep.SetValue("1")
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if view := m.View().Content; !strings.Contains(view, "Se eliminarán 2") {
		t.Errorf("vista = %q", view)
	}
	prune = m.stack[len(m.stack)-1].(*pruneScreen)
	prune.confirm.SetValue("SI")
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})

	store, err := results.OpenStore(deps.resultsDir)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("quedan %d corridas; quiero 1", len(entries))
	}
}

func TestUIPruneCancel(t *testing.T) {
	deps := testDeps(t)
	seedHistoryRun(t, deps.resultsDir, "uno", "closed", "PASS")
	seedHistoryRun(t, deps.resultsDir, "dos", "closed", "PASS")
	seedHistoryRun(t, deps.resultsDir, "tres", "closed", "PASS")

	m := openMain(t, newApp(deps), "Mantenimiento")
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	prune := m.stack[len(m.stack)-1].(*pruneScreen)
	prune.keep.SetValue("1")
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	prune = m.stack[len(m.stack)-1].(*pruneScreen)
	prune.confirm.SetValue("NO")
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})

	store, _ := results.OpenStore(deps.resultsDir)
	entries, _ := store.List()
	if len(entries) != 3 {
		t.Errorf("quedan %d corridas; quiero 3", len(entries))
	}
}

func TestUIPruneKeepZero(t *testing.T) {
	deps := testDeps(t)
	seedHistoryRun(t, deps.resultsDir, "uno", "closed", "PASS")
	seedHistoryRun(t, deps.resultsDir, "dos", "closed", "PASS")

	m := openMain(t, newApp(deps), "Mantenimiento")
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	prune := m.stack[len(m.stack)-1].(*pruneScreen)
	prune.keep.SetValue("0")
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	prune = m.stack[len(m.stack)-1].(*pruneScreen)
	prune.confirm.SetValue("SI")
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})

	store, _ := results.OpenStore(deps.resultsDir)
	entries, _ := store.List()
	if len(entries) != 0 {
		t.Errorf("quedan %d corridas; quiero 0", len(entries))
	}
}

func TestUIPruneKeepAll(t *testing.T) {
	deps := testDeps(t)
	seedHistoryRun(t, deps.resultsDir, "uno", "closed", "PASS")
	seedHistoryRun(t, deps.resultsDir, "dos", "closed", "PASS")

	m := openMain(t, newApp(deps), "Mantenimiento")
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	prune := m.stack[len(m.stack)-1].(*pruneScreen)
	prune.keep.SetValue("99")
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})

	if view := m.View().Content; !strings.Contains(view, "No se eliminaría ninguna") {
		t.Errorf("vista = %q", view)
	}
	store, _ := results.OpenStore(deps.resultsDir)
	entries, _ := store.List()
	if len(entries) != 2 {
		t.Errorf("quedan %d corridas; quiero 2", len(entries))
	}
}

func TestUIPruneNonNumeric(t *testing.T) {
	deps := testDeps(t)
	seedHistoryRun(t, deps.resultsDir, "uno", "closed", "PASS")

	m := openMain(t, newApp(deps), "Mantenimiento")
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	prune := m.stack[len(m.stack)-1].(*pruneScreen)
	prune.keep.SetValue("abc")
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if view := m.View().Content; !strings.Contains(view, "entero") {
		t.Errorf("vista = %q", view)
	}
	store, _ := results.OpenStore(deps.resultsDir)
	entries, _ := store.List()
	if len(entries) != 1 {
		t.Errorf("quedan %d corridas; quiero 1", len(entries))
	}
}

func TestUIReindexMissingDir(t *testing.T) {
	deps := testDeps(t)
	deps.resultsDir = filepath.Join(t.TempDir(), "missing")
	m := openMain(t, newApp(deps), "Mantenimiento")
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyDown}, tea.KeyPressMsg{Code: tea.KeyEnter})
	if view := m.View().Content; !strings.Contains(view, "No hay historial") {
		t.Errorf("vista = %q", view)
	}
	if _, err := os.Stat(deps.resultsDir); !os.IsNotExist(err) {
		t.Errorf("el directorio se creó: %v", err)
	}
}

func TestUIReindexExisting(t *testing.T) {
	deps := testDeps(t)
	seedHistoryRun(t, deps.resultsDir, "uno", "closed", "PASS")
	if err := os.WriteFile(filepath.Join(deps.resultsDir, "index.jsonl"), []byte("{ corrupto\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := openMain(t, newApp(deps), "Mantenimiento")
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyDown}, tea.KeyPressMsg{Code: tea.KeyEnter})
	if view := m.View().Content; !strings.Contains(view, "Índice reconstruido") {
		t.Errorf("vista = %q", view)
	}
	store, _ := results.OpenStore(deps.resultsDir)
	entries, _ := store.List()
	if len(entries) != 1 {
		t.Errorf("entradas = %d; quiero 1", len(entries))
	}
}

func TestUIPruneFailureShowsError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root puede borrar en un directorio de solo lectura")
	}
	deps := testDeps(t)
	seedHistoryRun(t, deps.resultsDir, "uno", "closed", "PASS")
	if err := os.Chmod(deps.resultsDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(deps.resultsDir, 0o700) })

	m := openMain(t, newApp(deps), "Mantenimiento")
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	prune := m.stack[len(m.stack)-1].(*pruneScreen)
	prune.keep.SetValue("0")
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	prune = m.stack[len(m.stack)-1].(*pruneScreen)
	prune.confirm.SetValue("SI")
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})

	view := strings.ToLower(m.View().Content)
	if !strings.Contains(view, "unlinkat") && !strings.Contains(view, "permission") && !strings.Contains(view, "denied") && !strings.Contains(view, "read-only") {
		t.Errorf("vista sin error de prune: %q", view)
	}
}

func TestUIPruneFieldKeepsQ(t *testing.T) {
	deps := testDeps(t)
	seedHistoryRun(t, deps.resultsDir, "uno", "closed", "PASS")
	m := openMain(t, newApp(deps), "Mantenimiento")
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	prune := m.stack[len(m.stack)-1].(*pruneScreen)
	prune.keep.SetValue("1")
	m = updateApp(t, m, tea.KeyPressMsg{Code: 'q', Text: "q"})
	if len(m.stack) != 3 {
		t.Errorf("pila = %d; quiero 3", len(m.stack))
	}
	if prune.keep.Value() != "1q" {
		t.Errorf("keep = %q; quiero 1q", prune.keep.Value())
	}
}

func TestUIPruneUsesExactPreview(t *testing.T) {
	deps := testDeps(t)
	uno := seedHistoryRun(t, deps.resultsDir, "uno", "closed", "PASS")
	dos := seedHistoryRun(t, deps.resultsDir, "dos", "closed", "PASS")
	tres := seedHistoryRun(t, deps.resultsDir, "tres", "closed", "PASS")
	_ = uno
	_ = dos
	_ = tres

	m := openMain(t, newApp(deps), "Mantenimiento")
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	prune := m.stack[len(m.stack)-1].(*pruneScreen)
	prune.keep.SetValue("1")
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})

	// Una corrida nueva aparece después del preview.
	nueva := seedHistoryRun(t, deps.resultsDir, "cuarta", "closed", "PASS")
	prune = m.stack[len(m.stack)-1].(*pruneScreen)
	prune.confirm.SetValue("SI")
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})

	store, _ := results.OpenStore(deps.resultsDir)
	entries, _ := store.List()
	if len(entries) != 2 {
		t.Fatalf("quedan %d corridas; quiero 2", len(entries))
	}
	found := false
	for _, entry := range entries {
		if entry.RunID == nueva.RunID {
			found = true
		}
	}
	if !found {
		t.Error("la corrida nueva no debería borrarse")
	}
}

func TestUIPruneShowsTotalBeforeEnter(t *testing.T) {
	deps := testDeps(t)
	seedHistoryRun(t, deps.resultsDir, "uno", "closed", "PASS")
	seedHistoryRun(t, deps.resultsDir, "dos", "closed", "PASS")
	m := openMain(t, newApp(deps), "Mantenimiento")
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if view := m.View().Content; !strings.Contains(view, "Corridas actuales: 2") {
		t.Errorf("vista = %q", view)
	}
}
