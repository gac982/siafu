// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

type card struct {
	title  string
	value  string
	detail string
}

func renderCard(c card, width int) string {
	if width < 6 {
		return truncate(c.title, width)
	}
	inner := width - 2
	top := borderStyle.Render("┌" + strings.Repeat("─", inner) + "┐")
	bottom := borderStyle.Render("└" + strings.Repeat("─", inner) + "┘")

	line := func(styled string) string {
		return borderStyle.Render("│") + padRight(styled, inner) + borderStyle.Render("│")
	}

	return strings.Join([]string{
		top,
		line(titleStyle.Render(truncate(c.title, inner))),
		line(valueStyle.Render(truncate(c.value, inner))),
		line(secondaryStyle.Render(truncate(c.detail, inner))),
		bottom,
	}, "\n")
}

func renderCardGrid(cards []card, width int) string {
	if len(cards) == 0 {
		return ""
	}
	if width < 40 {
		parts := make([]string, 0, len(cards))
		for _, c := range cards {
			parts = append(parts, renderCard(c, width))
		}
		return lipgloss.JoinVertical(lipgloss.Left, parts...)
	}

	columnWidth := (width - 1) / 2
	var rows []string
	for i := 0; i < len(cards); i += 2 {
		left := renderCard(cards[i], columnWidth)
		if i+1 >= len(cards) {
			rows = append(rows, left)
			continue
		}
		right := renderCard(cards[i+1], columnWidth)
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right))
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func progressBar(fraction float64, width int) string {
	if width <= 0 {
		return ""
	}
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}
	filled := int(fraction*float64(width) + 0.5)
	if filled > width {
		filled = width
	}
	return accentStyle.Render(strings.Repeat("█", filled)) + secondaryStyle.Render(strings.Repeat("░", width-filled))
}

func padRight(s string, width int) string {
	gap := width - lipgloss.Width(s)
	if gap <= 0 {
		return truncate(s, width)
	}
	return s + strings.Repeat(" ", gap)
}

func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	if width == 1 {
		return "…"
	}
	var b strings.Builder
	used := 0
	for _, r := range s {
		runeWidth := lipgloss.Width(string(r))
		if used+runeWidth > width-1 {
			break
		}
		b.WriteRune(r)
		used += runeWidth
	}
	return b.String() + "…"
}

func formatDuration(d time.Duration) string {
	total := int64(d / time.Second)
	hours := total / 3600
	minutes := (total % 3600) / 60
	seconds := total % 60
	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

func formatBytes(value uint64) string {
	const unit = 1024
	if value < unit {
		return fmt.Sprintf("%d B", value)
	}
	div, exp := uint64(unit), 0
	for n := value / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	if exp >= len("KMGTPE") {
		exp = len("KMGTPE") - 1
	}
	return fmt.Sprintf("%.1f %cB", float64(value)/float64(div), "KMGTPE"[exp])
}

func formatRate(value float64) string {
	return formatBytes(uint64(value)) + "/s"
}
