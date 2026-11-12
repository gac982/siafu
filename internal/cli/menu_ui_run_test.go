// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func press(code rune) []tea.Msg {
	return []tea.Msg{tea.KeyPressMsg{Code: code}}
}

func TestUIRunFlowExecutes(t *testing.T) {
	path := writeMenuConfig(t, t.TempDir())
	deps := testDeps(t)
	deps.candidates = func() []string { return []string{path} }
	var got []string
	deps.exec = func(args []string) tea.Cmd {
		got = args
		return func() tea.Msg { return runFinishedMsg{} }
	}

	m := updateApp(t, newApp(deps),
		tea.KeyPressMsg{Code: tea.KeyEnter},
		tea.KeyPressMsg{Code: tea.KeyEnter},
		tea.KeyPressMsg{Code: tea.KeyEnter},
	)
	want := buildRunArgs(path, deps.resultsDir, "", "", false, false, false)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("args = %#v; quiero %#v", got, want)
	}
	if view := m.View().Content; !strings.Contains(view, "Código:") && !strings.Contains(view, "Veredicto:") {
		t.Errorf("vista sin resultado: %q", view)
	}
}

func TestUIRunFlowAdvanced(t *testing.T) {
	path := writeMenuConfig(t, t.TempDir())
	deps := testDeps(t)
	deps.candidates = func() []string { return []string{path} }
	var got []string
	deps.exec = func(args []string) tea.Cmd {
		got = args
		return func() tea.Msg { return runFinishedMsg{} }
	}

	m := updateApp(t, newApp(deps),
		tea.KeyPressMsg{Code: tea.KeyEnter},
		tea.KeyPressMsg{Code: tea.KeyEnter},
		tea.KeyPressMsg{Code: tea.KeyDown},
		tea.KeyPressMsg{Code: tea.KeyEnter},
	)
	adv, ok := m.stack[len(m.stack)-1].(*advancedScreen)
	if !ok {
		t.Fatalf("pantalla = %T; quiero *advancedScreen", m.stack[len(m.stack)-1])
	}
	adv.seed.SetValue("42")
	adv.tag.SetValue("smoke")
	adv.dir.SetValue("/custom")
	adv.noSave = true
	adv.noTUI = true
	adv.debug = true

	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	m = updateApp(t, m,
		tea.KeyPressMsg{Code: tea.KeyUp},
		tea.KeyPressMsg{Code: tea.KeyEnter},
	)
	want := buildRunArgs(path, "/custom", "42", "smoke", true, true, true)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("args = %#v; quiero %#v", got, want)
	}
}

func TestUIRunFlowInvalidConfig(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(bad, []byte("{no yaml"), 0o600); err != nil {
		t.Fatal(err)
	}
	deps := testDeps(t)
	deps.candidates = func() []string { return []string{bad} }
	called := false
	deps.exec = func([]string) tea.Cmd {
		called = true
		return nil
	}

	m := updateApp(t, newApp(deps),
		tea.KeyPressMsg{Code: tea.KeyEnter},
		tea.KeyPressMsg{Code: tea.KeyEnter},
	)
	if called {
		t.Error("exec se llamó con una configuración inválida")
	}
	if view := m.View().Content; !strings.Contains(view, "bad.yaml") {
		t.Errorf("vista = %q", view)
	}
}

func TestUIValidateFlow(t *testing.T) {
	path := writeMenuConfig(t, t.TempDir())
	deps := testDeps(t)
	deps.candidates = func() []string { return []string{path} }

	m := openMain(t, newApp(deps), "Validar un YAML")
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if view := m.View().Content; !strings.Contains(view, "OK") {
		t.Errorf("vista = %q", view)
	}
}

func TestUIRunFinishedShowsVerdict(t *testing.T) {
	deps := testDeps(t)
	seedHistoryRun(t, deps.resultsDir, "corrida", "closed", "PASS")
	deps.now = func() time.Time { return time.Now().Add(-time.Hour) }
	path := writeMenuConfig(t, t.TempDir())
	deps.candidates = func() []string { return []string{path} }
	deps.exec = func([]string) tea.Cmd {
		return func() tea.Msg { return runFinishedMsg{} }
	}

	m := updateApp(t, newApp(deps),
		tea.KeyPressMsg{Code: tea.KeyEnter},
		tea.KeyPressMsg{Code: tea.KeyEnter},
		tea.KeyPressMsg{Code: tea.KeyEnter},
	)
	if view := m.View().Content; !strings.Contains(view, "APROBADA") {
		t.Errorf("vista = %q", view)
	}
}

func TestUIRunFinishedNoSaveShowsCode(t *testing.T) {
	path := writeMenuConfig(t, t.TempDir())
	deps := testDeps(t)
	deps.candidates = func() []string { return []string{path} }
	deps.exec = func([]string) tea.Cmd {
		return func() tea.Msg { return runFinishedMsg{code: 1} }
	}

	m := updateApp(t, newApp(deps),
		tea.KeyPressMsg{Code: tea.KeyEnter},
		tea.KeyPressMsg{Code: tea.KeyEnter},
		tea.KeyPressMsg{Code: tea.KeyDown},
		tea.KeyPressMsg{Code: tea.KeyEnter},
	)
	adv := m.stack[len(m.stack)-1].(*advancedScreen)
	adv.noSave = true
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyUp}, tea.KeyPressMsg{Code: tea.KeyEnter})

	if view := m.View().Content; !strings.Contains(view, "Código: 1") {
		t.Errorf("vista = %q", view)
	}
}

func TestUIRunScreenTextInputKeepsQ(t *testing.T) {
	path := writeMenuConfig(t, t.TempDir())
	deps := testDeps(t)
	deps.candidates = func() []string { return []string{path} }

	m := updateApp(t, newApp(deps),
		tea.KeyPressMsg{Code: tea.KeyEnter},
		tea.KeyPressMsg{Code: tea.KeyEnter},
		tea.KeyPressMsg{Code: tea.KeyDown},
		tea.KeyPressMsg{Code: tea.KeyEnter},
	)
	adv := m.stack[len(m.stack)-1].(*advancedScreen)
	m = updateApp(t, m, tea.KeyPressMsg{Code: 'q', Text: "q"})
	if m.quitCode != 0 {
		t.Errorf("quitCode = %d; quiero 0", m.quitCode)
	}
	if adv.seed.Value() != "q" {
		t.Errorf("seed = %q; quiero q", adv.seed.Value())
	}
}

func TestUIRunFinishedUsesCustomResultsDir(t *testing.T) {
	deps := testDeps(t)
	custom := t.TempDir()
	seedHistoryRun(t, custom, "custom", "closed", "PASS")
	deps.now = func() time.Time { return time.Now().Add(-time.Hour) }
	path := writeMenuConfig(t, t.TempDir())

	result := newRunResultScreen(deps, path, runOptions{resultsDir: custom})
	m := newApp(deps)
	m.stack = append(m.stack, result)
	m = updateApp(t, m, runFinishedMsg{})

	if view := m.View().Content; !strings.Contains(view, "APROBADA") {
		t.Errorf("vista = %q", view)
	}
}

func TestUITwoRuns(t *testing.T) {
	path := writeMenuConfig(t, t.TempDir())
	deps := testDeps(t)
	deps.candidates = func() []string { return []string{path} }
	calls := 0
	deps.exec = func([]string) tea.Cmd {
		calls++
		return func() tea.Msg { return runFinishedMsg{} }
	}

	m := newApp(deps)
	m = openMain(t, m, "Ejecutar una prueba")
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEnter}, tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEnter}) // result -> home

	m = openMain(t, m, "Ejecutar una prueba")
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEnter}, tea.KeyPressMsg{Code: tea.KeyEnter})

	if calls != 2 {
		t.Errorf("corridas ejecutadas = %d; quiero 2", calls)
	}
}

func TestUIFilePickerQBack(t *testing.T) {
	deps := testDeps(t)
	m := openMain(t, newApp(deps), "Ejecutar una prueba")
	m = updateApp(t, m, tea.KeyPressMsg{Code: 'q', Text: "q"})
	if len(m.stack) != 1 {
		t.Errorf("pila = %d; quiero 1", len(m.stack))
	}
	if m.quitCode != 0 {
		t.Errorf("quitCode = %d; quiero 0", m.quitCode)
	}
}

func TestRunScreenNormalizesDashPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "--evil.yaml")
	if err := os.WriteFile(path, []byte(`version: 1
name: dash
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
`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	deps := testDeps(t)
	screen := newRunScreen(deps, "--evil.yaml")
	if screen.err != nil {
		t.Fatalf("newRunScreen() = %v", screen.err)
	}
	if screen.path != "./--evil.yaml" {
		t.Errorf("path = %q; quiero ./--evil.yaml", screen.path)
	}
}
