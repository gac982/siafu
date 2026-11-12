// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package cli

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/gac982/siafu/internal/tui"

	"github.com/gac982/siafu/internal/safetext"
)

type listItem struct {
	title string
	desc  string
}

const selectListVisible = 8

type selectList struct {
	items  []listItem
	cursor int
	offset int
}

func newSelectList(items []listItem) selectList {
	return selectList{items: items}
}

func (l *selectList) Handle(msg tea.Msg) bool {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return false
	}
	switch key.String() {
	case "up", "k":
		if l.cursor > 0 {
			l.cursor--
		}
		l.ensureVisible()
		return true
	case "down", "j":
		if l.cursor < len(l.items)-1 {
			l.cursor++
		}
		l.ensureVisible()
		return true
	case "home":
		l.cursor = 0
		l.ensureVisible()
		return true
	case "end":
		l.cursor = len(l.items) - 1
		l.ensureVisible()
		return true
	}
	return false
}

func (l *selectList) ensureVisible() {
	if l.cursor < l.offset {
		l.offset = l.cursor
	}
	if l.cursor >= l.offset+selectListVisible {
		l.offset = l.cursor - selectListVisible + 1
	}
	if l.offset < 0 {
		l.offset = 0
	}
}

func (l selectList) Selected() (listItem, bool) {
	if len(l.items) == 0 || l.cursor < 0 || l.cursor >= len(l.items) {
		return listItem{}, false
	}
	return l.items[l.cursor], true
}

func (l selectList) View(width int) string {
	var b strings.Builder
	start := l.offset
	end := min(len(l.items), start+selectListVisible)
	for i := start; i < end; i++ {
		item := l.items[i]
		marker := "  "
		titleStyle := lipgloss.NewStyle().Foreground(tui.Bone)
		markerStyle := lipgloss.NewStyle().Foreground(tui.Ember).Bold(true)
		if i == l.cursor {
			marker = "▸ "
		} else {
			markerStyle = lipgloss.NewStyle().Foreground(tui.Ash)
		}
		fmt.Fprintf(&b, "%s%s\n", markerStyle.Render(marker), titleStyle.Render(safetext.Line(item.title)))
		if item.desc != "" {
			fmt.Fprintf(&b, "  %s\n", lipgloss.NewStyle().Foreground(tui.Ash).Render(safetext.Line(item.desc)))
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func renderFrame(version, breadcrumb, body, footer string, width, height int) string {
	if width < 20 {
		width = 20
	}
	if height < 8 {
		height = 8
	}
	cardWidth := max(16, min(width-2, 76))
	innerWidth := max(8, cardWidth-4)
	maxBody := max(1, height-10)
	header := lipgloss.JoinHorizontal(lipgloss.Left,
		lipgloss.NewStyle().Foreground(tui.Ember).Bold(true).Render("SIAFU"),
		lipgloss.NewStyle().Foreground(tui.Ash).Render("  "+version),
	)
	header = ansi.Truncate(header, innerWidth, "…")
	crumb := ansi.Truncate(lipgloss.NewStyle().Foreground(tui.Ash).Render(breadcrumb), innerWidth, "…")
	body = clipBody(body, maxBody, innerWidth)
	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(tui.Ash).
		Padding(1, 2).
		Width(cardWidth).
		Render(lipgloss.JoinVertical(lipgloss.Left, header, crumb, "", body))
	hint := ansi.Truncate(lipgloss.NewStyle().Foreground(tui.Ash).Render(footer), max(10, width-2), "…")
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(lipgloss.Center, card, hint))
}

func clipBody(body string, maxLines, width int) string {
	lines := strings.Split(body, "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	for i := range lines {
		lines[i] = ansi.Truncate(lines[i], width, "…")
	}
	return strings.Join(lines, "\n")
}

type textField struct {
	placeholder string
	value       []rune
	cursor      int
	max         int
}

func newTextField(placeholder, value string) *textField {
	return newTextFieldLimit(placeholder, value, 256)
}

func newTextFieldLimit(placeholder, value string, max int) *textField {
	if max < 1 {
		max = 256
	}
	runes := []rune(value)
	if len(runes) > max {
		runes = runes[:max]
	}
	return &textField{placeholder: placeholder, value: runes, cursor: len(runes), max: max}
}

func (f *textField) SetValue(value string) {
	f.value = []rune(value)
	if len(f.value) > f.max {
		f.value = f.value[:f.max]
	}
	f.cursor = len(f.value)
}

func (f *textField) Value() string { return string(f.value) }

func (f *textField) Update(msg tea.Msg) bool {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return false
	}
	switch key.Code {
	case tea.KeyBackspace:
		if f.cursor > 0 {
			f.value = append(f.value[:f.cursor-1], f.value[f.cursor:]...)
			f.cursor--
		}
		return true
	case tea.KeyDelete:
		if f.cursor < len(f.value) {
			f.value = append(f.value[:f.cursor], f.value[f.cursor+1:]...)
		}
		return true
	case tea.KeyLeft:
		if f.cursor > 0 {
			f.cursor--
		}
		return true
	case tea.KeyRight:
		if f.cursor < len(f.value) {
			f.cursor++
		}
		return true
	case tea.KeyHome:
		f.cursor = 0
		return true
	case tea.KeyEnd:
		f.cursor = len(f.value)
		return true
	}
	if key.Mod == tea.ModCtrl && key.Code == 'u' {
		f.value = nil
		f.cursor = 0
		return true
	}
	if key.Text == "" {
		return false
	}
	for _, r := range []rune(key.Text) {
		if len(f.value) >= f.max {
			break
		}
		if r < 0x20 || r == 0x7f {
			continue
		}
		f.value = append(f.value, 0)
		copy(f.value[f.cursor+1:], f.value[f.cursor:])
		f.value[f.cursor] = r
		f.cursor++
	}
	return true
}

func (f *textField) View() string {
	if len(f.value) == 0 {
		return lipgloss.NewStyle().Foreground(tui.Ash).Render(f.placeholder) + "▏"
	}
	before := safetext.Line(string(f.value[:f.cursor]))
	after := safetext.Line(string(f.value[f.cursor:]))
	return before + lipgloss.NewStyle().Foreground(tui.Ember).Render("▏") + after
}
