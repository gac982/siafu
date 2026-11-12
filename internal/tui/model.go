// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package tui

import (
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/gac982/siafu/internal/metrics"
	"github.com/gac982/siafu/internal/report"
	"github.com/gac982/siafu/internal/sysmetrics"

	"github.com/gac982/siafu/internal/safetext"
)

// State is the lifecycle phase shown by the dashboard.
type State int

const (
	// StateRunning means the load is active.
	StateRunning State = iota
	// StateFinishing means the engine is stopping.
	StateFinishing
)

// Frame is the immutable data the orchestrator sends to the UI.
type Frame struct {
	Name         string
	Target       string
	Elapsed      time.Duration
	Planned      time.Duration
	Stage        int
	Stages       int
	VUsActive    int
	VUsTarget    int
	VUsPeak      int
	Mode         string
	TargetRate   float64
	AchievedRate float64
	InFlight     int
	MaxInFlight  int
	Dropped      int64
	Metrics      metrics.Snapshot
	Host         sysmetrics.Snapshot
	Logs         []string
	State        State
}

// Actions are the callbacks the model invokes on key presses.
// Quit must only cancel/stop external work; the model returns tea.Quit
// from Update to terminate Bubble Tea. Calling Program.Quit from inside
// Update would deadlock on the program's unbuffered message channel.
type Actions struct {
	Stop func()
	Quit func()
}

// FrameMsg updates the dashboard.
type FrameMsg struct {
	Frame Frame
}

// ReportMsg switches the UI to the final report.
type ReportMsg struct {
	Report report.Report
}

// Model is the Bubble Tea model of the live dashboard.
type Model struct {
	actions Actions
	frame   Frame

	width, height  int
	logsFull       bool
	showHelp       bool
	logFollow      bool
	logViewport    viewport.Model
	reportViewport viewport.Model
	report         *report.Report

	rpsHistory   []float64
	p95History   []float64
	lastSeriesAt time.Duration
}

// New creates an empty dashboard.
func New(actions Actions) Model {
	vp := viewport.New(viewport.WithWidth(80), viewport.WithHeight(8))
	return Model{
		actions:        actions,
		width:          100,
		height:         30,
		logFollow:      true,
		logViewport:    vp,
		reportViewport: viewport.New(viewport.WithWidth(80), viewport.WithHeight(8)),
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeViewport()
		return m, nil
	case FrameMsg:
		m.applyFrame(msg.Frame)
		return m, nil
	case ReportMsg:
		m.applyReport(msg.Report)
		return m, nil
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+d":
		if m.actions.Stop != nil {
			m.actions.Stop()
		}
	case "ctrl+c":
		if m.actions.Quit != nil {
			m.actions.Quit()
		}
		return m, tea.Quit
	case "x":
		m.logsFull = !m.logsFull
		m.resizeViewport()
	case "?":
		m.showHelp = !m.showHelp
	case "esc":
		if m.showHelp {
			m.showHelp = false
		} else if m.logsFull {
			m.logsFull = false
			m.resizeViewport()
		} else if m.report != nil {
			if m.actions.Quit != nil {
				m.actions.Quit()
			}
			return m, tea.Quit
		}
	case "q":
		if m.report != nil {
			if m.actions.Quit != nil {
				m.actions.Quit()
			}
			return m, tea.Quit
		}
	case "up", "down", "pgup", "pgdown", "home", "end":
		if m.report != nil && !m.logsFull {
			m.reportViewport, _ = m.reportViewport.Update(msg)
			return m, nil
		}
		m.logFollow = false
		m.logViewport, _ = m.logViewport.Update(msg)
	}
	return m, nil
}

func (m *Model) applyFrame(frame Frame) {
	m.frame = frame

	elapsed := frame.Metrics.Elapsed
	if elapsed == 0 {
		elapsed = frame.Elapsed
	}
	if len(m.rpsHistory) == 0 || elapsed-m.lastSeriesAt >= time.Second {
		m.rpsHistory = append(m.rpsHistory, frame.Metrics.RPSRecent)
		m.p95History = append(m.p95History, float64(frame.Metrics.Latency.P95)/float64(time.Millisecond))
		if len(m.rpsHistory) > 120 {
			m.rpsHistory = m.rpsHistory[len(m.rpsHistory)-120:]
			m.p95History = m.p95History[len(m.p95History)-120:]
		}
		m.lastSeriesAt = elapsed
	}

	logs := make([]string, 0, len(frame.Logs))
	for _, line := range frame.Logs {
		logs = append(logs, safetext.Line(line))
	}
	content := strings.Join(logs, "\n")
	if content == "" {
		content = "sin eventos todavía"
	}
	m.logViewport.SetContent(content)
	m.resizeViewport()
	if m.logFollow {
		m.logViewport.GotoBottom()
	}
}

func (m *Model) resizeViewport() {
	width, height := m.size()
	m.reportViewport.SetWidth(max(10, width-4))
	m.reportViewport.SetHeight(max(1, height-5))
	if m.logsFull {
		m.logViewport.SetWidth(max(10, width-4))
		m.logViewport.SetHeight(max(1, height-5))
		return
	}
	_, gridHeight, trend, logPanel := dashboardHeights(width, height)
	_ = gridHeight
	_ = trend
	m.logViewport.SetWidth(max(10, width-4))
	m.logViewport.SetHeight(max(1, logPanel-3))
}

// View implements tea.Model.
func (m Model) View() tea.View {
	width, height := m.size()
	var content string
	switch {
	case m.showHelp:
		content = fitView(helpView(width), width, height)
	case m.logsFull:
		content = fitView(m.logsView(width, height), width, height)
	case m.report != nil:
		content = fitView(m.reportView(width, height), width, height)
	default:
		content = fitView(m.dashboard(width, height), width, height)
	}
	view := tea.NewView(content)
	view.AltScreen = true
	return view
}

func (m Model) size() (int, int) {
	width, height := m.width, m.height
	if width <= 0 {
		width = 100
	}
	if height <= 0 {
		height = 30
	}
	return width, height
}
