// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package tui

import (
	_ "embed"
	"os"
	"regexp"
	"strings"
)

//go:embed assets/logo.ans
var logoArt string

//go:embed assets/mark.ans
var markArt string

var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// Logo returns the full Siafu wordmark as ANSI block art. With color=false the
// escape sequences are stripped, which is what non-TTY output needs.
func Logo(color bool) string { return renderArt(logoArt, color) }

// LogoMark returns the compact ant mark used by the interactive menu.
func LogoMark(color bool) string { return renderArt(markArt, color) }

func renderArt(art string, color bool) string {
	art = strings.TrimRight(art, "\n")
	if color {
		return art
	}
	return ansiEscape.ReplaceAllString(art, "")
}

// ColorEnabled reports whether the environment can render the colored logo.
// The TUI itself requires a terminal; this also honors NO_COLOR and TERM=dumb
// for commands such as `siafu version`.
func ColorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	return !strings.EqualFold(os.Getenv("TERM"), "dumb")
}
