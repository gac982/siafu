// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package report

import (
	"strings"
	"testing"

	"github.com/gac982/siafu/internal/metrics"
)

func TestTextSanitizesTerminalControls(t *testing.T) {
	text := Text(Report{
		Name:   "\x1b]0;evil\x07demo",
		Target: "http://127.0.0.1\x1b[31m",
	})
	if strings.ContainsRune(text, 0x1b) || strings.ContainsRune(text, 0x07) {
		t.Errorf("reporte con controles: %q", text)
	}
}

func TestTextShowsProtocols(t *testing.T) {
	text := Text(Report{Global: metrics.Snapshot{Protocols: map[string]int64{"HTTP/1.1": 2, "HTTP/2.0": 3}}})
	if !strings.Contains(text, "Protocolos:") || !strings.Contains(text, "HTTP/2.0: 3") {
		t.Errorf("reporte sin protocolos: %q", text)
	}
}
