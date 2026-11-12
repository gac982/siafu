// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package cli

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/gac982/siafu/internal/tui"
)

func testDeps(t *testing.T) *menuDeps {
	t.Helper()
	return &menuDeps{
		version:     "test",
		resultsDir:  t.TempDir(),
		errOut:      io.Discard,
		candidates:  func() []string { return nil },
		exec:        func([]string) tea.Cmd { return nil },
		openBrowser: func(string) error { return nil },
		startServer: func(string, string) (string, func(), error) {
			return "http://127.0.0.1:1", func() {}, nil
		},
		now: time.Now,
	}
}

func updateApp(t *testing.T, m appModel, msgs ...tea.Msg) appModel {
	t.Helper()
	queue := append([]tea.Msg(nil), msgs...)
	for len(queue) > 0 {
		msg := queue[0]
		queue = queue[1:]
		next, cmd := m.Update(msg)
		var ok bool
		m, ok = next.(appModel)
		if !ok {
			t.Fatalf("Update devolvió %T; quiero appModel", next)
		}
		if cmd != nil {
			if result := cmd(); result != nil {
				queue = append([]tea.Msg{result}, queue...)
			}
		}
	}
	return m
}

func TestMenuUIRendersMainMenu(t *testing.T) {
	m := newApp(testDeps(t))
	m.width, m.height = 100, 30
	view := m.View().Content
	for _, want := range []string{"SIAFU", "Ejecutar una prueba", "Ver corridas anteriores", "Abrir la web de resultados", "Validar un YAML", "Mantenimiento", "Ajustes y versión", "Salir", "↑↓"} {
		if !strings.Contains(view, want) {
			t.Errorf("vista sin %q:\n%s", want, view)
		}
	}
}

func TestMenuUIQuits(t *testing.T) {
	m := updateApp(t, newApp(testDeps(t)), tea.KeyPressMsg{Code: 'q'})
	if m.quitCode != 0 {
		t.Errorf("quitCode = %d; quiero 0", m.quitCode)
	}
}

func TestMenuUICtrlC(t *testing.T) {
	m := updateApp(t, newApp(testDeps(t)), tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if m.quitCode != 130 {
		t.Errorf("quitCode = %d; quiero 130", m.quitCode)
	}
}

func openMain(t *testing.T, m appModel, title string) appModel {
	t.Helper()
	main, ok := m.stack[0].(*mainScreen)
	if !ok {
		t.Fatalf("pantalla base = %T; quiero *mainScreen", m.stack[0])
	}
	for i, item := range main.list.items {
		if item.title == title {
			main.list.cursor = i
			return updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
		}
	}
	t.Fatalf("no existe la opción %q", title)
	return m
}

func TestMainMenuOpensSettings(t *testing.T) {
	m := openMain(t, newApp(testDeps(t)), "Ajustes y versión")
	if len(m.stack) != 2 {
		t.Fatalf("pila = %d; quiero 2", len(m.stack))
	}
	if title := m.stack[1].Title(); title != "Ajustes y versión" {
		t.Errorf("title = %q", title)
	}
}

func TestSettingsScreenChangesResultsDir(t *testing.T) {
	deps := testDeps(t)
	m := openMain(t, newApp(deps), "Ajustes y versión")
	settings, ok := m.stack[1].(*settingsScreen)
	if !ok {
		t.Fatalf("pantalla = %T", m.stack[1])
	}
	newDir := t.TempDir()
	settings.input.SetValue(newDir)
	m.stack[1] = settings

	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if deps.resultsDir != newDir {
		t.Errorf("resultsDir = %q; quiero %q", deps.resultsDir, newDir)
	}
	if len(m.stack) != 1 {
		t.Errorf("pila = %d; quiero 1", len(m.stack))
	}
}

func TestRunMenuUsesUI(t *testing.T) {
	var out, errOut bytes.Buffer
	code := runMenu(strings.NewReader("q\n"), &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d; quiero 0 (stderr: %s)", code, errOut.String())
	}
}

func TestSelectListScrolls(t *testing.T) {
	items := make([]listItem, 12)
	for i := range items {
		items[i] = listItem{title: fmt.Sprintf("f%d", i)}
	}
	list := newSelectList(items)
	for i := 0; i < 10; i++ {
		list.Handle(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	view := list.View(80)
	if !strings.Contains(view, "f10") {
		t.Errorf("la vista no muestra el cursor: %q", view)
	}
	if strings.Contains(view, "f0\n") {
		t.Errorf("la vista no hizo scroll: %q", view)
	}
}

func TestTextFieldLimit(t *testing.T) {
	field := newTextField("x", "")
	field.SetValue(strings.Repeat("a", 300))
	if got := len([]rune(field.Value())); got != 256 {
		t.Errorf("SetValue len = %d; quiero 256", got)
	}
	field = newTextField("x", "")
	for i := 0; i < 300; i++ {
		field.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	}
	if got := len([]rune(field.Value())); got != 256 {
		t.Errorf("Update len = %d; quiero 256", got)
	}
}

func TestUITextFieldPathLimit(t *testing.T) {
	deps := testDeps(t)
	m := openMain(t, newApp(deps), "Ajustes y versión")
	settings := m.stack[1].(*settingsScreen)
	settings.input.SetValue(strings.Repeat("p", 5000))
	if got := len([]rune(settings.input.Value())); got != 4096 {
		t.Errorf("path len = %d; quiero 4096", got)
	}
}

func TestTextFieldViewSanitizes(t *testing.T) {
	field := newTextField("x", "a\x1b]0;evil\x07b")
	view := field.View()
	if strings.Contains(view, "\x1b]0;") || strings.ContainsRune(view, 0x07) {
		t.Errorf("campo con controles: %q", view)
	}
}

func TestRenderFrameFitsBounds(t *testing.T) {
	body := strings.Repeat("línea larga de prueba que debería recortarse\n", 50)
	frame := renderFrame("versión larga de prueba", "SIAFU › Pantalla", body, "pie de prueba", 60, 20)
	if h := lipgloss.Height(frame); h > 20 {
		t.Errorf("alto = %d; quiero <= 20", h)
	}
	if w := lipgloss.Width(frame); w > 60 {
		t.Errorf("ancho = %d; quiero <= 60", w)
	}
}

func TestMenuUIViewColors(t *testing.T) {
	m := newApp(testDeps(t))
	view := m.View()
	if view.BackgroundColor != tui.Ink {
		t.Errorf("background = %v; quiero %v", view.BackgroundColor, tui.Ink)
	}
	if view.ForegroundColor != tui.Bone {
		t.Errorf("foreground = %v; quiero %v", view.ForegroundColor, tui.Bone)
	}
}

func TestFilePickerUsesLastYAML(t *testing.T) {
	deps := testDeps(t)
	deps.candidates = func() []string { return []string{"a.yaml", "b.yaml"} }
	deps.lastYAML = "b.yaml"
	m := openMain(t, newApp(deps), "Ejecutar una prueba")
	picker, ok := m.stack[len(m.stack)-1].(*filePickerScreen)
	if !ok {
		t.Fatalf("pantalla = %T", m.stack[len(m.stack)-1])
	}
	item, _ := picker.list.Selected()
	if item.title != "b.yaml" {
		t.Errorf("cursor en %q; quiero b.yaml", item.title)
	}
}

func TestSelectListRendersAndNavigates(t *testing.T) {
	list := newSelectList([]listItem{{title: "uno"}, {title: "dos"}})
	view := list.View(80)
	if !strings.Contains(view, "uno") || !strings.Contains(view, "dos") {
		t.Fatalf("vista = %q", view)
	}
	list.Handle(tea.KeyPressMsg{Code: tea.KeyDown})
	item, ok := list.Selected()
	if !ok || item.title != "dos" {
		t.Errorf("selección = %q, %v; quiero dos, true", item.title, ok)
	}
}
