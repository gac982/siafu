// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/gac982/siafu/internal/metrics"
	"github.com/gac982/siafu/internal/report"
	"github.com/gac982/siafu/internal/sysmetrics"
)

func testReport() report.Report {
	codes := map[int]int64{200: 990, 429: 5, 500: 5}
	return report.Report{
		Name:      "Demo local",
		Target:    "http://127.0.0.1:8080",
		StartedAt: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
		Planned:   time.Minute,
		Actual:    time.Minute,
		PeakVUs:   20,
		Seed:      7,
		Verdict:   report.VerdictPass,
		Global: metrics.Snapshot{
			Total:        1000,
			Failed:       5,
			ErrorRate:    0.005,
			RPSAvg:       100,
			RPSPeak:      120,
			Latency:      metrics.Percentiles{P50: 50 * time.Millisecond, P95: 200 * time.Millisecond, P99: 500 * time.Millisecond},
			ChecksTotal:  1000,
			ChecksPassed: 995,
			Iterations:   300,
			StatusCodes:  codes,
		},
		Host: sysmetrics.Summary{CPUAvg: 20, CPUMax: 40, MemMaxPercent: 50},
		Thresholds: []report.ThresholdResult{
			{Name: "latencia p95", Expected: "≤ 300ms", Observed: "200ms", Pass: true, Evaluated: true},
		},
		Diagnosis: []report.Finding{{Severity: report.SeverityInfo, Message: "Todos los umbrales se cumplen."}},
	}
}

func TestReportViewShowsVerdictAndSections(t *testing.T) {
	m := New(Actions{})
	setSize(&m, 120, 60)
	updated, _ := m.Update(ReportMsg{Report: testReport()})
	m = updated.(Model)
	view := m.View().Content
	for _, want := range []string{"APROBADA", "RESUMEN GLOBAL", "UMBRALES", "DIAGNÓSTICO", "REPORTE"} {
		if !strings.Contains(view, want) {
			t.Errorf("report view does not contain %q", want)
		}
	}
}

func TestReportScroll(t *testing.T) {
	m := New(Actions{})
	setSize(&m, 80, 24)
	rep := testReport()
	codes := map[int]int64{}
	for code := 100; code < 200; code++ {
		codes[code] = int64(code)
	}
	rep.Global.StatusCodes = codes
	updated, _ := m.Update(ReportMsg{Report: rep})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = updated.(Model)
	if m.reportViewport.YOffset() == 0 {
		t.Errorf("report did not scroll (content height %d, viewport %d)", len(strings.Split(m.reportViewport.View(), "\n")), m.reportViewport.Height())
	}
}

func TestReportKeysQuit(t *testing.T) {
	quits := 0
	m := New(Actions{Quit: func() { quits++ }})
	setSize(&m, 100, 30)
	updated, _ := m.Update(ReportMsg{Report: testReport()})
	m = updated.(Model)
	_, _ = m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if quits != 1 {
		t.Fatalf("quits = %d, want 1", quits)
	}
}

func TestReportEscQuits(t *testing.T) {
	quits := 0
	m := New(Actions{Quit: func() { quits++ }})
	setSize(&m, 100, 30)
	updated, _ := m.Update(ReportMsg{Report: testReport()})
	m = updated.(Model)
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if quits != 1 {
		t.Fatalf("quits = %d, want 1", quits)
	}
}

func TestReportViewSmallTerminal(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {60, 20}} {
		m := New(Actions{})
		setSize(&m, size[0], size[1])
		updated, _ := m.Update(ReportMsg{Report: testReport()})
		m = updated.(Model)
		view := m.View().Content
		lines := strings.Split(view, "\n")
		if len(lines) > size[1] {
			t.Errorf("%dx%d: %d lines exceed height", size[0], size[1], len(lines))
		}
		for i, line := range lines {
			if width := lipgloss.Width(line); width > size[0] {
				t.Errorf("%dx%d: line %d width %d exceeds width", size[0], size[1], i, width)
			}
		}
	}
}
