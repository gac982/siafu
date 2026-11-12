// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestUIWebStartsAndShowsURL(t *testing.T) {
	deps := testDeps(t)
	stopped := false
	deps.startServer = func(addr, dir string) (string, func(), error) {
		return "http://127.0.0.1:9999", func() { stopped = true }, nil
	}
	m := openMain(t, newApp(deps), "Abrir la web de resultados")
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})

	view := m.View().Content
	if !strings.Contains(view, "http://127.0.0.1:9999") || !strings.Contains(view, "activo") {
		t.Errorf("vista = %q", view)
	}
	m = updateApp(t, m, tea.KeyPressMsg{Code: 's', Text: "s"})
	if !stopped {
		t.Error("stop no se llamó")
	}
	if len(m.stack) != 1 {
		t.Errorf("pila = %d; quiero 1 tras detener", len(m.stack))
	}
}

func TestUIWebOpensBrowser(t *testing.T) {
	deps := testDeps(t)
	opened := ""
	deps.startServer = func(addr, dir string) (string, func(), error) {
		return "http://127.0.0.1:9999", func() {}, nil
	}
	deps.openBrowser = func(url string) error {
		opened = url
		return nil
	}
	m := openMain(t, newApp(deps), "Abrir la web de resultados")
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	_ = m
	if opened != "http://127.0.0.1:9999" {
		t.Errorf("url abierta = %q", opened)
	}
}

func TestUIWebBindError(t *testing.T) {
	deps := testDeps(t)
	deps.startServer = func(addr, dir string) (string, func(), error) {
		return "", nil, errors.New("address already in use")
	}
	m := openMain(t, newApp(deps), "Abrir la web de resultados")
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if view := m.View().Content; !strings.Contains(view, "address already in use") {
		t.Errorf("vista = %q", view)
	}
}

func TestUIWebBrowserFailure(t *testing.T) {
	deps := testDeps(t)
	var errOut bytes.Buffer
	deps.errOut = &errOut
	deps.startServer = func(addr, dir string) (string, func(), error) {
		return "http://127.0.0.1:9999", func() {}, nil
	}
	deps.openBrowser = func(string) error { return errors.New("no hay navegador") }

	m := openMain(t, newApp(deps), "Abrir la web de resultados")
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if view := m.View().Content; !strings.Contains(view, "http://127.0.0.1:9999") {
		t.Errorf("vista = %q", view)
	}
	if !strings.Contains(errOut.String(), "no se pudo abrir el navegador") {
		t.Errorf("stderr = %q", errOut.String())
	}
}

func TestUIWebCleanupOnQuit(t *testing.T) {
	deps := testDeps(t)
	stopped := false
	deps.startServer = func(addr, dir string) (string, func(), error) {
		return "http://127.0.0.1:9999", func() { stopped = true }, nil
	}
	m := openMain(t, newApp(deps), "Abrir la web de resultados")
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updateApp(t, m, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if !stopped {
		t.Error("stop no se llamó al salir")
	}
	if m.quitCode != 130 {
		t.Errorf("quitCode = %d; quiero 130", m.quitCode)
	}
}

func TestUIWebInvalidPort(t *testing.T) {
	deps := testDeps(t)
	called := false
	deps.startServer = func(addr, dir string) (string, func(), error) {
		called = true
		return "", nil, nil
	}
	m := openMain(t, newApp(deps), "Abrir la web de resultados")
	screen := m.stack[len(m.stack)-1].(*webScreen)
	screen.port.SetValue("99999")
	m = updateApp(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if called {
		t.Error("startServer se llamó con un puerto inválido")
	}
	if view := m.View().Content; !strings.Contains(view, "puerto") {
		t.Errorf("vista = %q", view)
	}
}

func TestUIWebPortKeepsQ(t *testing.T) {
	deps := testDeps(t)
	m := openMain(t, newApp(deps), "Abrir la web de resultados")
	m = updateApp(t, m, tea.KeyPressMsg{Code: 'q', Text: "q"})
	if len(m.stack) != 2 {
		t.Errorf("pila = %d; quiero 2", len(m.stack))
	}
	screen := m.stack[len(m.stack)-1].(*webScreen)
	if screen.port.Value() != "8080q" {
		t.Errorf("puerto = %q; quiero 8080q", screen.port.Value())
	}
}
