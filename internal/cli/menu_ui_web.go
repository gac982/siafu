// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package cli

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/gac982/siafu/internal/results"
	"github.com/gac982/siafu/internal/tui"
	"github.com/gac982/siafu/internal/web"

	"github.com/gac982/siafu/internal/safetext"
)

type webScreen struct {
	baseScreen
	deps       *menuDeps
	port       *textField
	url        string
	stop       func()
	err        error
	browserErr error
}

func newWebScreen(deps *menuDeps) *webScreen {
	return &webScreen{deps: deps, port: newTextField("8080", "8080")}
}

func (s *webScreen) Title() string { return "Web" }

func (s *webScreen) Cleanup() { s.stopServer() }

func (s *webScreen) stopServer() {
	if s.stop != nil {
		s.stop()
		s.stop = nil
		s.url = ""
	}
}

func (s *webScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return s, nil
	}
	if s.stop != nil {
		switch key.String() {
		case "esc", "q":
			s.stopServer()
			return s, func() tea.Msg { return popScreenMsg{} }
		case "s":
			s.stopServer()
			return s, func() tea.Msg { return popScreenMsg{} }
		case "o", "enter":
			s.open()
		}
		return s, nil
	}

	switch key.String() {
	case "esc":
		return s, func() tea.Msg { return popScreenMsg{} }
	case "enter":
		port, err := strconv.Atoi(strings.TrimSpace(s.port.Value()))
		if err != nil || port < 1 || port > 65535 {
			s.err = fmt.Errorf("el puerto debe estar entre 1 y 65535")
			return s, nil
		}
		url, stop, err := s.deps.startServer(fmt.Sprintf("127.0.0.1:%d", port), s.deps.resultsDir)
		if err != nil {
			s.err = err
			return s, nil
		}
		s.url, s.stop, s.err = url, stop, nil
		s.open()
		return s, nil
	}
	s.port.Update(msg)
	return s, nil
}

func (s *webScreen) open() {
	if s.deps.openBrowser == nil {
		return
	}
	if err := s.deps.openBrowser(s.url); err != nil {
		s.browserErr = err
		fmt.Fprintf(s.deps.errOut, "no se pudo abrir el navegador: %v\nAbre %s manualmente.\n", err, s.url)
		return
	}
	s.browserErr = nil
}

func (s *webScreen) View(width, height int) string {
	if s.stop != nil {
		lines := []string{
			lipgloss.NewStyle().Foreground(tui.Bone).Bold(true).Render("Servidor activo"),
			"",
			lipgloss.NewStyle().Foreground(tui.Ember).Render(safetext.Line(s.url)),
			lipgloss.NewStyle().Foreground(tui.Ash).Render("Resultados: " + safetext.Line(s.deps.resultsDir)),
		}
		if s.browserErr != nil {
			lines = append(lines, lipgloss.NewStyle().Foreground(tui.Ember).Render("No se pudo abrir el navegador: "+safetext.Line(s.browserErr.Error())))
		}
		lines = append(lines, "", lipgloss.NewStyle().Foreground(tui.Ash).Render("o abrir navegador · s detener · Esc volver"))
		return lipgloss.JoinVertical(lipgloss.Left, lines...)
	}
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(tui.Bone).Bold(true).Render("Abrir la web de resultados"))
	b.WriteString("\n\n")
	b.WriteString(lipgloss.NewStyle().Foreground(tui.Ash).Render("Puerto en 127.0.0.1:"))
	b.WriteString("\n")
	b.WriteString(s.port.View())
	b.WriteString("\n\n")
	if s.err != nil {
		b.WriteString(lipgloss.NewStyle().Foreground(tui.Ember).Render(safetext.Line(s.err.Error())))
		b.WriteString("\n\n")
	}
	b.WriteString(lipgloss.NewStyle().Foreground(tui.Ash).Render("Enter iniciar · Esc volver"))
	return b.String()
}

func defaultStartServer(addr, dir string) (string, func(), error) {
	store, err := results.OpenStore(dir)
	if err != nil {
		return "", nil, err
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return "", nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	url := "http://" + listener.Addr().String()
	go func() {
		_ = serveListenerWithReady(ctx, listener, web.NewServer(store, web.AssetsFS()).Handler(), io.Discard, store.Dir, nil)
	}()
	return url, cancel, nil
}
