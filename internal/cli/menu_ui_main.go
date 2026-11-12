// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package cli

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/gac982/siafu/internal/tui"
)

type mainScreen struct {
	baseScreen
	deps *menuDeps
	list selectList
}

func newMainScreen(deps *menuDeps) *mainScreen {
	return &mainScreen{
		deps: deps,
		list: newSelectList([]listItem{
			{title: "Ejecutar una prueba"},
			{title: "Ver corridas anteriores"},
			{title: "Abrir la web de resultados"},
			{title: "Validar un YAML"},
			{title: "Mantenimiento"},
			{title: "Ajustes y versión"},
			{title: "Salir"},
		}),
	}
}

func (s *mainScreen) Title() string { return "Inicio" }

func (s *mainScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return s, nil
	}
	switch key.String() {
	case "q", "esc":
		return s, func() tea.Msg { return quitAppMsg{code: 0} }
	case "enter":
		item, ok := s.list.Selected()
		if !ok {
			return s, nil
		}
		switch item.title {
		case "Ejecutar una prueba":
			onPick := func(path string) tea.Cmd {
				s.deps.lastYAML = path
				return func() tea.Msg { return pushScreenMsg{screen: newRunScreen(s.deps, path)} }
			}
			return s, func() tea.Msg { return pushScreenMsg{screen: newFilePickerScreen(s.deps, onPick)} }
		case "Ver corridas anteriores":
			return s, func() tea.Msg { return pushScreenMsg{screen: newHistoryScreen(s.deps)} }
		case "Abrir la web de resultados":
			return s, func() tea.Msg { return pushScreenMsg{screen: newWebScreen(s.deps)} }
		case "Validar un YAML":
			onPick := func(path string) tea.Cmd {
				s.deps.lastYAML = path
				return func() tea.Msg { return pushScreenMsg{screen: newValidateResultScreen(path)} }
			}
			return s, func() tea.Msg { return pushScreenMsg{screen: newFilePickerScreen(s.deps, onPick)} }
		case "Mantenimiento":
			return s, func() tea.Msg { return pushScreenMsg{screen: newMaintenanceScreen(s.deps)} }
		case "Ajustes y versión":
			return s, func() tea.Msg { return pushScreenMsg{screen: newSettingsScreen(s.deps)} }
		case "Salir":
			return s, func() tea.Msg { return quitAppMsg{code: 0} }
		}
	}
	s.list.Handle(msg)
	return s, nil
}

func (s *mainScreen) View(width, height int) string {
	body := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Foreground(tui.Bone).Bold(true).Render("Inicio"),
		"",
		s.list.View(width),
	)
	if width >= 64 && height >= 24 {
		return lipgloss.JoinHorizontal(lipgloss.Center, tui.LogoMark(true), "  ", body)
	}
	return body
}
