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
	"github.com/gac982/siafu/internal/sysmetrics"
)

func sampleFrame() Frame {
	return Frame{
		Name:      "Demo local",
		Target:    "http://127.0.0.1:8080",
		Elapsed:   30 * time.Second,
		Planned:   3 * time.Minute,
		Stage:     2,
		Stages:    3,
		VUsActive: 87,
		VUsTarget: 100,
		VUsPeak:   100,
		Metrics: metrics.Snapshot{
			Elapsed:      30 * time.Second,
			Total:        18420,
			Failed:       148,
			ErrorRate:    0.008,
			RPSRecent:    142.3,
			RPSAvg:       130.5,
			RPSPeak:      150,
			RPSPeakAt:    20 * time.Second,
			Latency:      metrics.Percentiles{P50: 90 * time.Millisecond, P95: 210 * time.Millisecond, P99: 480 * time.Millisecond},
			ChecksTotal:  18420,
			ChecksPassed: 18272,
			Iterations:   3905,
		},
		Host: sysmetrics.Snapshot{
			CPUPercent:      34,
			MemUsed:         6 << 30,
			MemTotal:        16 << 30,
			MemPercent:      39,
			DiskUsedPercent: 61,
			DiskFree:        142 << 30,
			NetRxBps:        12 << 20,
			NetTxBps:        3 << 20,
		},
		Logs: []string{"10:42:01 INFO etapa 2/3", "10:42:03 WARN GET /api → 500"},
	}
}

func setFrame(m *Model, frame Frame) {
	updated, _ := m.Update(FrameMsg{Frame: frame})
	*m = updated.(Model)
}

func setSize(m *Model, width, height int) {
	updated, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	*m = updated.(Model)
}

func TestKeyCtrlDStops(t *testing.T) {
	stops := 0
	m := New(Actions{Stop: func() { stops++ }})
	_, _ = m.Update(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
	if stops != 1 {
		t.Fatalf("stops = %d, want 1", stops)
	}
}

func TestKeyCtrlCQuits(t *testing.T) {
	quits := 0
	m := New(Actions{Quit: func() { quits++ }})
	_, _ = m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if quits != 1 {
		t.Fatalf("quits = %d, want 1", quits)
	}
}

func TestKeyXTogglesLogs(t *testing.T) {
	m := New(Actions{})
	setFrame(&m, sampleFrame())
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	m = updated.(Model)
	if !m.logsFull {
		t.Fatal("x did not enable fullscreen logs")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	m = updated.(Model)
	if m.logsFull {
		t.Fatal("x did not disable fullscreen logs")
	}
}

func TestKeyHelp(t *testing.T) {
	m := New(Actions{})
	updated, _ := m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	m = updated.(Model)
	if !m.showHelp {
		t.Fatal("? did not show help")
	}
	view := m.View().Content
	if !strings.Contains(view, "Ctrl+D") {
		t.Errorf("help view does not contain keys: %q", view)
	}
}

func TestFrameUpdatesCards(t *testing.T) {
	m := New(Actions{})
	setSize(&m, 120, 40)
	setFrame(&m, sampleFrame())
	view := m.View().Content
	for _, want := range []string{"CPU", "RAM", "DISCO", "RED", "RPS", "p95", "Errores", "LOG", "Demo local", "127.0.0.1"} {
		if !strings.Contains(view, want) {
			t.Errorf("view does not contain %q", want)
		}
	}
}

func TestViewModelSmallTerminal(t *testing.T) {
	sizes := [][2]int{{120, 40}, {80, 24}, {60, 20}}
	for _, size := range sizes {
		m := New(Actions{})
		setSize(&m, size[0], size[1])
		setFrame(&m, sampleFrame())
		view := m.View().Content
		lines := strings.Split(view, "\n")
		if len(lines) > size[1] {
			t.Errorf("%dx%d: %d lines exceed height", size[0], size[1], len(lines))
		}
		for i, line := range lines {
			if width := lipgloss.Width(line); width > size[0] {
				t.Errorf("%dx%d: line %d width %d exceeds width: %q", size[0], size[1], i, width, line)
			}
		}
	}
}

func TestLogFullscreenShowsBuffer(t *testing.T) {
	m := New(Actions{})
	setSize(&m, 80, 24)
	setFrame(&m, sampleFrame())
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	m = updated.(Model)
	if view := m.View().Content; !strings.Contains(view, "10:42:03") {
		t.Errorf("fullscreen logs do not contain buffer: %q", view)
	}
}

func TestFrameAppendsSeriesOncePerSecond(t *testing.T) {
	m := New(Actions{})
	for _, elapsed := range []time.Duration{0, time.Second, 1500 * time.Millisecond, 2 * time.Second} {
		frame := sampleFrame()
		frame.Metrics.Elapsed = elapsed
		frame.Elapsed = elapsed
		setFrame(&m, frame)
	}
	if len(m.rpsHistory) != 3 {
		t.Fatalf("rps history = %d points, want 3", len(m.rpsHistory))
	}
}

func TestSparkline(t *testing.T) {
	if got := RenderSparkline(nil, 10); got != "" {
		t.Errorf("empty sparkline = %q", got)
	}
	flat := RenderSparkline([]float64{5, 5, 5}, 3)
	if len([]rune(flat)) != 3 {
		t.Errorf("flat sparkline = %q", flat)
	}
	got := RenderSparkline([]float64{1, 2, 3, 4, 5}, 5)
	if len([]rune(got)) != 5 {
		t.Errorf("sparkline = %q", got)
	}
	if RenderSparkline([]float64{1, 2, 3}, 10) == "" {
		t.Error("sparkline did not pad to requested width")
	}
}

func TestCardTruncatesLongValues(t *testing.T) {
	card := renderCard(card{title: "CPU", value: strings.Repeat("x", 100), detail: "detalle"}, 20)
	for _, line := range strings.Split(card, "\n") {
		if width := lipgloss.Width(line); width > 20 {
			t.Errorf("card line width %d > 20: %q", width, line)
		}
	}
}

func TestCtrlCReturnsQuitCommand(t *testing.T) {
	quits := 0
	m := New(Actions{Quit: func() { quits++ }})
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("Ctrl+C did not return a command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("Ctrl+C returned %T, want tea.QuitMsg", cmd())
	}
	if quits != 1 {
		t.Fatalf("Quit action called %d times, want 1", quits)
	}
}

func TestCtrlDDoesNotQuitProgram(t *testing.T) {
	stops := 0
	m := New(Actions{Stop: func() { stops++ }})
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
	if cmd != nil {
		t.Fatalf("Ctrl+D returned a command (%T); it must only stop the engine", cmd())
	}
	if stops != 1 {
		t.Fatalf("Stop action called %d times, want 1", stops)
	}
}

func TestQReturnsQuitCommandInReport(t *testing.T) {
	quits := 0
	m := New(Actions{Quit: func() { quits++ }})
	setSize(&m, 100, 30)
	setFrame(&m, sampleFrame())
	// q is ignored while running.
	if _, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"}); cmd != nil {
		t.Fatal("q returned a command while the test was still running")
	}
	if quits != 0 {
		t.Fatalf("q quit while running")
	}

	// q quits on the report screen.
	updated, _ := m.Update(ReportMsg{Report: testReport()})
	m = updated.(Model)
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd == nil {
		t.Fatal("q on the report did not return a command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("q returned %T, want tea.QuitMsg", cmd())
	}
	if quits != 1 {
		t.Fatalf("Quit action called %d times, want 1", quits)
	}
}

func TestEscReturnsQuitCommandInReport(t *testing.T) {
	quits := 0
	m := New(Actions{Quit: func() { quits++ }})
	setSize(&m, 100, 30)
	updated, _ := m.Update(ReportMsg{Report: testReport()})
	m = updated.(Model)
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("Esc on the report did not return a command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("Esc returned %T, want tea.QuitMsg", cmd())
	}
	if quits != 1 {
		t.Fatalf("Quit action called %d times, want 1", quits)
	}
}

func TestFrameOpenLoopCards(t *testing.T) {
	m := New(Actions{})
	setSize(&m, 120, 40)
	frame := sampleFrame()
	frame.Mode = "open"
	frame.TargetRate = 5000
	frame.AchievedRate = 4800
	frame.InFlight = 120
	frame.MaxInFlight = 8000
	frame.Dropped = 42
	setFrame(&m, frame)

	view := m.View().Content
	for _, want := range []string{"Target/s", "Achieved/s", "Dropped", "in-flight"} {
		if !strings.Contains(view, want) {
			t.Errorf("open-loop view does not contain %q", want)
		}
	}
}
