// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

// Package safetext removes terminal control and bidi characters from strings
// before they are rendered to a terminal.
package safetext

import "strings"

// Line sanitizes a single-line string. Newlines, carriage returns and tabs are
// replaced by spaces; other control and bidi characters are removed.
func Line(s string) string {
	return strings.Map(safeRune, s)
}

// Multiline sanitizes a multi-line string. Newlines are preserved, tabs become
// four spaces and other control/bidi characters are removed.
func Multiline(s string) string {
	s = strings.ReplaceAll(s, "\t", "    ")
	return strings.Map(func(r rune) rune {
		if r == '\n' {
			return r
		}
		if r == '\r' {
			return ' '
		}
		return safeRune(r)
	}, s)
}

func safeRune(r rune) rune {
	switch r {
	case '\n', '\r', '\t':
		return ' '
	}
	if r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f) {
		return -1
	}
	if (r >= 0x202a && r <= 0x202e) || (r >= 0x2066 && r <= 0x2069) || (r >= 0x200e && r <= 0x200f) {
		return -1
	}
	return r
}
