// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package cli

import (
	"fmt"
	"io"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/gac982/siafu/internal/orchestrator"
	"github.com/gac982/siafu/internal/tui"
	"github.com/gac982/siafu/internal/version"
)

type menuDeps struct {
	version     string
	resultsDir  string
	errOut      io.Writer
	lastYAML    string
	candidates  func() []string
	exec        func([]string) tea.Cmd
	openBrowser func(string) error
	startServer func(string, string) (string, func(), error)
	now         func() time.Time
}

type screen interface {
	Title() string
	Init() tea.Cmd
	Update(tea.Msg) (screen, tea.Cmd)
	View(width, height int) string
	Cleanup()
}

type baseScreen struct{}

func (baseScreen) Init() tea.Cmd { return nil }
func (baseScreen) Cleanup()      {}

type pushScreenMsg struct{ screen screen }
type popScreenMsg struct{}
type quitAppMsg struct{ code int }
type homeMsg struct{}

type appModel struct {
	deps     *menuDeps
	stack    []screen
	width    int
	height   int
	quitCode int
}

func newApp(deps *menuDeps) appModel {
	return appModel{
		deps:   deps,
		width:  80,
		height: 24,
		stack:  []screen{newMainScreen(deps)},
	}
}

func (m appModel) Init() tea.Cmd {
	if len(m.stack) == 0 {
		return nil
	}
	return m.stack[len(m.stack)-1].Init()
}

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case pushScreenMsg:
		m.stack = append(m.stack, msg.screen)
		return m, m.stack[len(m.stack)-1].Init()
	case popScreenMsg:
		if len(m.stack) > 0 {
			m.stack[len(m.stack)-1].Cleanup()
			m.stack = m.stack[:len(m.stack)-1]
		}
		if len(m.stack) == 0 {
			return m, tea.Quit
		}
		return m, nil
	case homeMsg:
		for len(m.stack) > 1 {
			m.stack[len(m.stack)-1].Cleanup()
			m.stack = m.stack[:len(m.stack)-1]
		}
		return m, nil
	case quitAppMsg:
		m.quitCode = msg.code
		m.cleanupAll()
		return m, tea.Quit
	case tea.KeyPressMsg:
		if msg.Mod == tea.ModCtrl && msg.Code == 'c' {
			m.quitCode = 130
			m.cleanupAll()
			return m, tea.Quit
		}
	}

	if len(m.stack) == 0 {
		return m, nil
	}
	next, cmd := m.stack[len(m.stack)-1].Update(msg)
	if next != nil {
		m.stack[len(m.stack)-1] = next
	}
	return m, cmd
}

func (m *appModel) cleanupAll() {
	for _, s := range m.stack {
		s.Cleanup()
	}
}

func (m appModel) View() tea.View {
	if len(m.stack) == 0 {
		return tea.NewView("")
	}
	top := m.stack[len(m.stack)-1]
	breadcrumb := "SIAFU"
	if len(m.stack) > 1 {
		breadcrumb += " › " + top.Title()
	}
	body := top.View(m.width, m.height)
	footer := "↑↓ mover · Enter elegir · Esc/q volver · Ctrl+C salir"
	view := tea.NewView(renderFrame(version.Version, breadcrumb, body, footer, m.width, m.height))
	view.AltScreen = true
	view.BackgroundColor = tui.Ink
	view.ForegroundColor = tui.Bone
	return view
}

func runMenuUI(in io.Reader, out, errOut io.Writer) int {
	deps := defaultMenuDeps()
	deps.errOut = errOut
	program := tea.NewProgram(newApp(&deps), tea.WithInput(in), tea.WithOutput(out))
	final, err := program.Run()
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	if model, ok := final.(appModel); ok {
		return model.quitCode
	}
	return 0
}

func defaultMenuDeps() menuDeps {
	return menuDeps{
		version:     version.String(),
		resultsDir:  orchestrator.ResolveResultsDir(orchestrator.Options{}),
		errOut:      os.Stderr,
		candidates:  configCandidates,
		exec:        defaultExec,
		openBrowser: openInBrowser,
		startServer: defaultStartServer,
		now:         time.Now,
	}
}
