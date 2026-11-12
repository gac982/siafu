// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package cli

import (
	"bytes"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/gac982/siafu/internal/results"
	"github.com/gac982/siafu/internal/tui"

	"github.com/gac982/siafu/internal/safetext"
)

const historyUIPageSize = 6

type historyScreen struct {
	baseScreen
	deps    *menuDeps
	filters historyFilters
	page    int
	cursor  int
	all     int
	entries []results.IndexEntry
	err     error
	status  string
}

func newHistoryScreen(deps *menuDeps) *historyScreen {
	s := &historyScreen{deps: deps}
	s.reload()
	return s
}

func (s *historyScreen) Title() string { return "Historial" }

func (s *historyScreen) reload() {
	s.status = ""
	store, err := openRunsStore(s.deps.resultsDir, s.deps.errOut)
	if err != nil {
		s.err = err
		s.entries = nil
		return
	}
	entries, err := store.List()
	if err != nil {
		s.err = err
		s.entries = nil
		return
	}
	s.err = nil
	s.all = len(entries)
	s.entries = filterEntries(entries, s.filters.mode, s.filters.verdict, s.filters.query, 0)
	if s.page >= s.pages() {
		s.page = s.pages() - 1
	}
	if s.page < 0 {
		s.page = 0
	}
	if s.cursor >= len(s.pageEntries()) {
		s.cursor = len(s.pageEntries()) - 1
	}
	if s.cursor < 0 {
		s.cursor = 0
	}
}

func (s *historyScreen) pages() int {
	pages := (len(s.entries) + historyUIPageSize - 1) / historyUIPageSize
	if pages < 1 {
		pages = 1
	}
	return pages
}

func (s *historyScreen) pageEntries() []results.IndexEntry {
	start := s.page * historyUIPageSize
	end := start + historyUIPageSize
	if start > len(s.entries) {
		start = len(s.entries)
	}
	if end > len(s.entries) {
		end = len(s.entries)
	}
	return s.entries[start:end]
}

func (s *historyScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return s, nil
	}
	switch key.String() {
	case "esc", "q":
		return s, func() tea.Msg { return popScreenMsg{} }
	case "up", "k":
		if s.cursor > 0 {
			s.cursor--
		}
		return s, nil
	case "down", "j":
		if s.cursor < len(s.pageEntries())-1 {
			s.cursor++
		}
		return s, nil
	case "n":
		if s.page < s.pages()-1 {
			s.page++
			s.cursor = 0
		}
		return s, nil
	case "p":
		if s.page > 0 {
			s.page--
			s.cursor = 0
		}
		return s, nil
	case "r":
		s.reload()
		return s, nil
	case "f":
		return s, func() tea.Msg {
			return pushScreenMsg{screen: newHistoryFilterScreen(s.filters, func(filters historyFilters) {
				s.filters = filters
				s.page = 0
				s.cursor = 0
				s.reload()
			})}
		}
	case "c":
		entries := s.pageEntries()
		if len(entries) < 2 {
			s.status = "Necesitas al menos dos corridas en esta página para comparar."
			return s, nil
		}
		return s, func() tea.Msg {
			return pushScreenMsg{screen: newHistoryCompareScreen(s.deps, entries)}
		}
	case "enter":
		entries := s.pageEntries()
		if len(entries) == 0 {
			return s, nil
		}
		entry := entries[s.cursor]
		return s, func() tea.Msg {
			return pushScreenMsg{screen: newHistoryDetailScreen(s.deps, entry.RunID)}
		}
	}
	return s, nil
}

func (s *historyScreen) View(width, height int) string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(tui.Bone).Bold(true).Render("Corridas guardadas"))
	b.WriteString("\n\n")
	if s.err != nil {
		b.WriteString(lipgloss.NewStyle().Foreground(tui.Ember).Render(safetext.Line(s.err.Error())))
		b.WriteString("\n\n")
		b.WriteString(lipgloss.NewStyle().Foreground(tui.Ash).Render("Esc/q volver"))
		return b.String()
	}
	if s.all == 0 {
		fmt.Fprintf(&b, "No hay corridas en %s. Ejecuta una prueba primero.\n\n", safetext.Line(s.deps.resultsDir))
		b.WriteString(lipgloss.NewStyle().Foreground(tui.Ash).Render("Esc/q volver"))
		return b.String()
	}
	if len(s.entries) == 0 {
		b.WriteString("No hay corridas que coincidan con el filtro.\n\n")
		b.WriteString(lipgloss.NewStyle().Foreground(tui.Ash).Render("f filtrar   Esc/q volver"))
		return b.String()
	}

	header := fmt.Sprintf("%-3s %-16s %-13s %-9s %-9s %s", "#", "FECHA", "VEREDICTO", "MODO", "P95", "NOMBRE")
	b.WriteString(lipgloss.NewStyle().Foreground(tui.Ash).Render(header))
	b.WriteString("\n")
	for i, entry := range s.pageEntries() {
		marker := "  "
		markerStyle := lipgloss.NewStyle().Foreground(tui.Ash)
		style := lipgloss.NewStyle().Foreground(tui.Ash)
		if i == s.cursor {
			marker = "▸ "
			markerStyle = lipgloss.NewStyle().Foreground(tui.Ember).Bold(true)
			style = lipgloss.NewStyle().Foreground(tui.Bone).Bold(true)
		}
		line := fmt.Sprintf("%-3d %-16s %-13s %-9s %-9s %s",
			s.page*historyUIPageSize+i+1,
			entry.CreatedAt.Local().Format("2006-01-02 15:04"),
			verdictLabel(entry.Verdict),
			modeLabel(entry.Mode),
			formatMS(entry.P95MS),
			safetext.Line(entry.Name),
		)
		b.WriteString(markerStyle.Render(marker) + style.Render(line))
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "\nPágina %d/%d · %d corrida(s)\n", s.page+1, s.pages(), len(s.entries))
	if s.status != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(tui.Ember).Render(s.status))
		b.WriteString("\n")
	}
	b.WriteString(lipgloss.NewStyle().Foreground(tui.Ash).Render("↑↓ mover · Enter detalle · f filtrar · n/p página · c comparar · r recargar · Esc/q volver"))
	return b.String()
}

type historyFilterScreen struct {
	baseScreen
	apply   func(historyFilters)
	mode    selectList
	verdict selectList
	query   *textField
	focus   int
}

func newHistoryFilterScreen(current historyFilters, apply func(historyFilters)) *historyFilterScreen {
	mode := newSelectList([]listItem{{title: "Todos"}, {title: "Cerrado"}, {title: "Abierto"}})
	switch current.mode {
	case "closed":
		mode.cursor = 1
	case "open":
		mode.cursor = 2
	}
	verdict := newSelectList([]listItem{
		{title: "Todos"}, {title: "APROBADA"}, {title: "FALLIDA"},
		{title: "SIN UMBRALES"}, {title: "ABORTADA"}, {title: "ERROR"},
	})
	switch current.verdict {
	case "PASS":
		verdict.cursor = 1
	case "FAIL":
		verdict.cursor = 2
	case "NOT_EVALUATED":
		verdict.cursor = 3
	case "ABORTED":
		verdict.cursor = 4
	case "ERROR":
		verdict.cursor = 5
	}
	return &historyFilterScreen{
		apply:   apply,
		mode:    mode,
		verdict: verdict,
		query:   newTextField("nombre, tag o id", current.query),
	}
}

func (s *historyFilterScreen) Title() string { return "Filtros" }

func (s *historyFilterScreen) filters() historyFilters {
	mode := ""
	switch s.mode.cursor {
	case 1:
		mode = "closed"
	case 2:
		mode = "open"
	}
	verdict := ""
	switch s.verdict.cursor {
	case 1:
		verdict = "PASS"
	case 2:
		verdict = "FAIL"
	case 3:
		verdict = "NOT_EVALUATED"
	case 4:
		verdict = "ABORTED"
	case 5:
		verdict = "ERROR"
	}
	return historyFilters{mode: mode, verdict: verdict, query: strings.TrimSpace(s.query.Value())}
}

func (s *historyFilterScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return s, nil
	}
	switch key.String() {
	case "esc":
		return s, func() tea.Msg { return popScreenMsg{} }
	case "enter":
		if s.apply != nil {
			s.apply(s.filters())
		}
		return s, func() tea.Msg { return popScreenMsg{} }
	case "up":
		if s.focus > 0 {
			s.focus--
		}
		return s, nil
	case "down", "tab":
		if s.focus < 2 {
			s.focus++
		}
		return s, nil
	case "left":
		if s.focus == 0 {
			if s.mode.cursor > 0 {
				s.mode.cursor--
			}
			return s, nil
		}
		if s.focus == 1 {
			if s.verdict.cursor > 0 {
				s.verdict.cursor--
			}
			return s, nil
		}
	case "right":
		if s.focus == 0 {
			if s.mode.cursor < len(s.mode.items)-1 {
				s.mode.cursor++
			}
			return s, nil
		}
		if s.focus == 1 {
			if s.verdict.cursor < len(s.verdict.items)-1 {
				s.verdict.cursor++
			}
			return s, nil
		}
	}
	if s.focus == 2 {
		s.query.Update(msg)
	}
	return s, nil
}

func (s *historyFilterScreen) View(width, height int) string {
	focus := lipgloss.NewStyle().Foreground(tui.Ember).Bold(true)
	normal := lipgloss.NewStyle().Foreground(tui.Bone)
	mark := func(index int) string {
		if s.focus == index {
			return focus.Render("▸ ")
		}
		return "  "
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		focus.Render("Filtros"),
		"",
		mark(0)+normal.Render("Modo: ")+normal.Render(s.mode.items[s.mode.cursor].title),
		mark(1)+normal.Render("Veredicto: ")+normal.Render(s.verdict.items[s.verdict.cursor].title),
		mark(2)+normal.Render("Buscar: ")+s.query.View(),
		"",
		lipgloss.NewStyle().Foreground(tui.Ash).Render("↑↓ mover · ←→ cambiar · Enter aplicar · Esc volver"),
	)
}

type historyDetailScreen struct {
	baseScreen
	title  string
	text   string
	offset int
}

func newHistoryDetailScreen(deps *menuDeps, id string) *historyDetailScreen {
	text := ""
	store, err := openRunsStore(deps.resultsDir, deps.errOut)
	if err != nil {
		text = err.Error()
	} else if summary, err := store.Summary(id); err == nil {
		text = summary
	} else {
		var buf bytes.Buffer
		runRunsShow([]string{id, "--results-dir", deps.resultsDir}, &buf, &buf)
		text = buf.String()
	}
	return &historyDetailScreen{title: "Detalle", text: safetext.Multiline(text)}
}

func (s *historyDetailScreen) Title() string { return "Detalle" }

func (s *historyDetailScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return s, nil
	}
	lines := strings.Split(s.text, "\n")
	switch key.String() {
	case "esc", "enter", "q":
		return s, func() tea.Msg { return popScreenMsg{} }
	case "up":
		if s.offset > 0 {
			s.offset--
		}
	case "down":
		if s.offset < len(lines)-1 {
			s.offset++
		}
	case "pgup":
		s.offset = max(0, s.offset-10)
	case "pgdown":
		s.offset = min(len(lines)-1, s.offset+10)
	}
	return s, nil
}

func (s *historyDetailScreen) View(width, height int) string {
	lines := strings.Split(s.text, "\n")
	if s.offset >= len(lines) {
		s.offset = max(0, len(lines)-1)
	}
	limit := max(1, height-14)
	end := min(len(lines), s.offset+limit)
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(tui.Bone).Bold(true).Render(s.title))
	b.WriteString("\n\n")
	b.WriteString(strings.Join(lines[s.offset:end], "\n"))
	b.WriteString("\n\n")
	b.WriteString(lipgloss.NewStyle().Foreground(tui.Ash).Render("↑↓/PgUp/PgDn desplazar · Esc/q volver"))
	return b.String()
}

type historyCompareScreen struct {
	baseScreen
	deps      *menuDeps
	runs      []results.IndexEntry
	cursor    int
	choosingB bool
	a, b      results.IndexEntry
	done      bool
	text      string
}

func newHistoryCompareScreen(deps *menuDeps, runs []results.IndexEntry) *historyCompareScreen {
	return &historyCompareScreen{deps: deps, runs: runs}
}

func (s *historyCompareScreen) Title() string { return "Comparar" }

func (s *historyCompareScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return s, nil
	}
	switch key.String() {
	case "esc", "q":
		return s, func() tea.Msg { return popScreenMsg{} }
	case "up":
		if s.cursor > 0 {
			s.cursor--
		}
		return s, nil
	case "down":
		if s.cursor < len(s.runs)-1 {
			s.cursor++
		}
		return s, nil
	case "enter":
		if s.done {
			return s, func() tea.Msg { return popScreenMsg{} }
		}
		if !s.choosingB {
			s.a = s.runs[s.cursor]
			s.choosingB = true
			return s, nil
		}
		s.b = s.runs[s.cursor]
		var buf bytes.Buffer
		runRunsCompare([]string{s.a.RunID, s.b.RunID, "--results-dir", s.deps.resultsDir}, &buf, &buf)
		s.text = safetext.Multiline(buf.String())
		s.done = true
		return s, nil
	}
	return s, nil
}

func (s *historyCompareScreen) View(width, height int) string {
	if s.done {
		return lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Foreground(tui.Bone).Bold(true).Render("Comparación"),
			"",
			s.text,
			"",
			lipgloss.NewStyle().Foreground(tui.Ash).Render("Enter/Esc volver"),
		)
	}
	var b strings.Builder
	prompt := "Selecciona la corrida A:"
	if s.choosingB {
		prompt = "Selecciona la corrida B:"
	}
	b.WriteString(lipgloss.NewStyle().Foreground(tui.Bone).Bold(true).Render(prompt))
	b.WriteString("\n\n")
	for i, run := range s.runs {
		marker := "  "
		markerStyle := lipgloss.NewStyle().Foreground(tui.Ash)
		style := lipgloss.NewStyle().Foreground(tui.Ash)
		if i == s.cursor {
			marker = "▸ "
			markerStyle = lipgloss.NewStyle().Foreground(tui.Ember).Bold(true)
			style = lipgloss.NewStyle().Foreground(tui.Bone).Bold(true)
		}
		line := fmt.Sprintf("%-16s %-12s %s", run.CreatedAt.Local().Format("2006-01-02 15:04"), verdictLabel(run.Verdict), safetext.Line(run.Name))
		b.WriteString(markerStyle.Render(marker) + style.Render(line))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(tui.Ash).Render("↑↓ mover · Enter elegir · Esc volver"))
	return b.String()
}
