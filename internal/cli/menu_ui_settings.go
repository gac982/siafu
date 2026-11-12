// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package cli

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/gac982/siafu/internal/tui"

	"github.com/gac982/siafu/internal/safetext"
)

type settingsScreen struct {
	baseScreen
	deps  *menuDeps
	input *textField
}

func newSettingsScreen(deps *menuDeps) *settingsScreen {
	return &settingsScreen{
		deps:  deps,
		input: newTextFieldLimit("Directorio de resultados", deps.resultsDir, 4096),
	}
}

func (s *settingsScreen) Title() string { return "Ajustes y versión" }

func (s *settingsScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "esc":
			return s, func() tea.Msg { return popScreenMsg{} }
		case "enter":
			value := strings.TrimSpace(s.input.Value())
			if value != "" {
				s.deps.resultsDir = value
			}
			return s, func() tea.Msg { return popScreenMsg{} }
		}
	}
	s.input.Update(msg)
	return s, nil
}

func (s *settingsScreen) View(width, height int) string {
	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Foreground(tui.Bone).Bold(true).Render("Ajustes y versión"),
		"",
		lipgloss.NewStyle().Foreground(tui.Ash).Render("Versión: "+safetext.Line(s.deps.version)),
		lipgloss.NewStyle().Foreground(tui.Ash).Render("Directorio de resultados: "+safetext.Line(s.deps.resultsDir)),
		s.input.View(),
		"",
		lipgloss.NewStyle().Foreground(tui.Ash).Render("Enter aplicar · Esc volver"),
	)
}
