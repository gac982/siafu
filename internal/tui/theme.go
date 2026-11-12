// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

// Package tui renders the live dashboard and the final report.
package tui

import "charm.land/lipgloss/v2"

// The palette is limited to exactly four colors.
var (
	Ink   = lipgloss.Color("#0E1116")
	Bone  = lipgloss.Color("#E6E1D6")
	Ember = lipgloss.Color("#FC4C02")
	Ash   = lipgloss.Color("#7A8394")
)

var (
	titleStyle     = lipgloss.NewStyle().Foreground(Bone).Bold(true)
	valueStyle     = lipgloss.NewStyle().Foreground(Ember).Bold(true)
	secondaryStyle = lipgloss.NewStyle().Foreground(Ash)
	borderStyle    = lipgloss.NewStyle().Foreground(Ash)
	accentStyle    = lipgloss.NewStyle().Foreground(Ember)
	pageStyle      = lipgloss.NewStyle().Foreground(Bone).Background(Ink)
)
