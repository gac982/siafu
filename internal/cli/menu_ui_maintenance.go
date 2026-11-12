// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/gac982/siafu/internal/results"
	"github.com/gac982/siafu/internal/tui"

	"github.com/gac982/siafu/internal/safetext"
)

type maintenanceScreen struct {
	baseScreen
	deps *menuDeps
	list selectList
}

func newMaintenanceScreen(deps *menuDeps) *maintenanceScreen {
	return &maintenanceScreen{
		deps: deps,
		list: newSelectList([]listItem{
			{title: "Conservar solo N corridas"},
			{title: "Reindexar"},
			{title: "Volver"},
		}),
	}
}

func (s *maintenanceScreen) Title() string { return "Mantenimiento" }

func (s *maintenanceScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return s, nil
	}
	switch key.String() {
	case "esc", "q":
		return s, func() tea.Msg { return popScreenMsg{} }
	case "enter":
		item, ok := s.list.Selected()
		if !ok {
			return s, nil
		}
		switch item.title {
		case "Conservar solo N corridas":
			return s, func() tea.Msg { return pushScreenMsg{screen: newPruneScreen(s.deps)} }
		case "Reindexar":
			return s, func() tea.Msg { return pushScreenMsg{screen: newReindexScreen(s.deps)} }
		case "Volver":
			return s, func() tea.Msg { return popScreenMsg{} }
		}
	}
	s.list.Handle(msg)
	return s, nil
}

func (s *maintenanceScreen) View(width, height int) string {
	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Foreground(tui.Bone).Bold(true).Render("Mantenimiento"),
		lipgloss.NewStyle().Foreground(tui.Ash).Render("Resultados: "+safetext.Line(s.deps.resultsDir)),
		"",
		s.list.View(width),
	)
}

type pruneScreen struct {
	baseScreen
	deps       *menuDeps
	keep       *textField
	confirm    *textField
	store      *results.Store
	stage      string
	total      int
	preview    []results.IndexEntry
	previewIDs []string
	result     string
	err        error
}

func newPruneScreen(deps *menuDeps) *pruneScreen {
	s := &pruneScreen{
		deps:    deps,
		keep:    newTextField("20", "20"),
		confirm: newTextField("SI para confirmar", ""),
		stage:   "keep",
	}
	store, err := openRunsStore(deps.resultsDir, deps.errOut)
	if err != nil {
		s.err = err
		return s
	}
	entries, err := store.List()
	if err != nil {
		s.err = err
		return s
	}
	s.store = store
	s.total = len(entries)
	return s
}

func (s *pruneScreen) Title() string { return "Conservar corridas" }

func (s *pruneScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return s, nil
	}
	if s.stage == "done" {
		switch key.String() {
		case "esc", "q", "enter":
			return s, func() tea.Msg { return popScreenMsg{} }
		}
		return s, nil
	}
	if key.String() == "esc" {
		return s, func() tea.Msg { return popScreenMsg{} }
	}
	if key.String() != "enter" {
		if s.stage == "keep" {
			s.keep.Update(msg)
		} else {
			s.confirm.Update(msg)
		}
		return s, nil
	}

	if s.stage == "keep" {
		keep, err := strconv.Atoi(strings.TrimSpace(s.keep.Value()))
		if err != nil || keep < 0 {
			s.err = fmt.Errorf("debe ser un entero mayor o igual que 0")
			return s, nil
		}
		store, err := openRunsStore(s.deps.resultsDir, s.deps.errOut)
		if err != nil {
			s.err = err
			return s, nil
		}
		entries, err := store.List()
		if err != nil {
			s.err = err
			return s, nil
		}
		s.store = store
		s.err = nil
		s.total = len(entries)
		if len(entries) == 0 {
			s.result = "No hay corridas para podar."
			s.stage = "done"
			return s, nil
		}
		if keep >= len(entries) {
			s.result = "No se eliminaría ninguna corrida."
			s.stage = "done"
			return s, nil
		}
		s.preview = entries[keep:]
		s.previewIDs = s.previewIDs[:0]
		for _, entry := range s.preview {
			s.previewIDs = append(s.previewIDs, entry.RunID)
		}
		s.stage = "confirm"
		return s, nil
	}

	if strings.TrimSpace(s.confirm.Value()) != "SI" {
		s.result = "cancelado"
		s.stage = "done"
		return s, nil
	}
	removed, err := s.store.PruneIDs(s.previewIDs)
	if err != nil {
		s.err = err
		s.stage = "done"
		return s, nil
	}
	s.result = fmt.Sprintf("Eliminada(s) %d corrida(s).", removed)
	s.stage = "done"
	return s, nil
}

func (s *pruneScreen) View(width, height int) string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(tui.Bone).Bold(true).Render("Conservar solo N corridas"))
	b.WriteString("\n\n")
	if s.stage == "keep" {
		fmt.Fprintf(&b, "Corridas actuales: %d\n\n", s.total)
		b.WriteString("¿Cuántas conservar? ")
		b.WriteString(s.keep.View())
		b.WriteString("\n\n")
		if s.err != nil {
			b.WriteString(lipgloss.NewStyle().Foreground(tui.Ember).Render(safetext.Line(s.err.Error())))
			b.WriteString("\n\n")
		}
		b.WriteString(lipgloss.NewStyle().Foreground(tui.Ash).Render("Enter continuar · Esc volver"))
		return b.String()
	}
	if s.stage == "confirm" {
		fmt.Fprintf(&b, "Se eliminarán %d corrida(s):\n", len(s.preview))
		for i, entry := range s.preview {
			if i >= 10 {
				fmt.Fprintf(&b, "  ... y %d más\n", len(s.preview)-10)
				break
			}
			fmt.Fprintf(&b, "  %s  %s\n", safetext.Line(entry.RunID), safetext.Line(entry.Name))
		}
		b.WriteString("\nEscribe SI para confirmar: ")
		b.WriteString(s.confirm.View())
		b.WriteString("\n\n")
		b.WriteString(lipgloss.NewStyle().Foreground(tui.Ash).Render("Enter confirmar · Esc cancelar"))
		return b.String()
	}
	if s.err != nil {
		b.WriteString(lipgloss.NewStyle().Foreground(tui.Ember).Render(safetext.Line(s.err.Error())))
	} else {
		b.WriteString(safetext.Line(s.result))
	}
	b.WriteString("\n\n")
	b.WriteString(lipgloss.NewStyle().Foreground(tui.Ash).Render("Enter/Esc volver"))
	return b.String()
}

type reindexScreen struct {
	baseScreen
	result string
}

func newReindexScreen(deps *menuDeps) *reindexScreen {
	s := &reindexScreen{}
	if _, err := os.Stat(deps.resultsDir); os.IsNotExist(err) {
		s.result = fmt.Sprintf("No hay historial en %s.", deps.resultsDir)
		return s
	} else if err != nil {
		s.result = err.Error()
		return s
	}
	store, err := results.NewStore(deps.resultsDir)
	if err != nil {
		s.result = err.Error()
		return s
	}
	if err := store.Reindex(); err != nil {
		s.result = err.Error()
		return s
	}
	s.result = "Índice reconstruido."
	return s
}

func (s *reindexScreen) Title() string { return "Reindexar" }

func (s *reindexScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "esc", "enter", "q":
			return s, func() tea.Msg { return popScreenMsg{} }
		}
	}
	return s, nil
}

func (s *reindexScreen) View(width, height int) string {
	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Foreground(tui.Bone).Bold(true).Render("Reindexar"),
		"",
		s.result,
		"",
		lipgloss.NewStyle().Foreground(tui.Ash).Render("Enter/Esc volver"),
	)
}
