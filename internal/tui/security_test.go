// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package tui

import (
	"strings"
	"testing"
)

func TestTUISanitizesFrameAndLogs(t *testing.T) {
	m := New(Actions{})
	updated, _ := m.Update(FrameMsg{Frame: Frame{
		Name:   "\x1b]0;evil\x07demo",
		Target: "http://127.0.0.1\x1b[31m",
		Logs:   []string{"\x1b[31mlog line"},
	}})
	view := updated.(Model).View().Content
	if strings.Contains(view, "\x1b]0;") || strings.Contains(view, "\x1b[31mlog") || strings.ContainsRune(view, 0x07) {
		t.Errorf("TUI con controles: %q", view)
	}
}
