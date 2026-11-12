// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package safetext

import (
	"strings"
	"testing"
)

func TestLineStripsControls(t *testing.T) {
	got := Line("a\x1b]52;c;evil\x07b\nc")
	if strings.ContainsRune(got, 0x1b) || strings.ContainsRune(got, 0x07) {
		t.Fatalf("quedaron controles: %q", got)
	}
	for _, want := range []string{"a", "b", "c"} {
		if !strings.Contains(got, want) {
			t.Errorf("falta %q en %q", want, got)
		}
	}
}

func TestMultilineKeepsNewlines(t *testing.T) {
	got := Multiline("a\nb\tc\x00")
	if !strings.Contains(got, "a\nb") {
		t.Errorf("se perdió el salto: %q", got)
	}
	if strings.ContainsRune(got, 0x00) || strings.ContainsRune(got, '\t') {
		t.Errorf("quedaron controles: %q", got)
	}
}

func TestBidiStripped(t *testing.T) {
	if got := Line("a\u202eb"); strings.ContainsRune(got, 0x202e) {
		t.Errorf("bidi no eliminado: %q", got)
	}
}
