// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package cli

import (
	"strings"
	"testing"
)

func TestMainScreenShowsMarkWhenWide(t *testing.T) {
	screen := newMainScreen(nil)

	wide := screen.View(80, 24)
	if !strings.Contains(wide, "▀") {
		t.Error("wide main screen should include the ANSI mark")
	}

	narrow := screen.View(40, 24)
	if strings.Contains(narrow, "▀") {
		t.Error("narrow main screen should skip the ANSI mark")
	}
}
