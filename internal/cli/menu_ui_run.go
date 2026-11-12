// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/gac982/siafu/internal/results"
	"github.com/gac982/siafu/internal/tui"

	"github.com/gac982/siafu/internal/safetext"
)

type runFinishedMsg struct {
	err  error
	code int
}

type runOptions struct {
	seed       string
	tag        string
	resultsDir string
	noSave     bool
	noTUI      bool
	debug      bool
}

func defaultExec(args []string) tea.Cmd {
	binary, err := os.Executable()
	if err != nil {
		return func() tea.Msg { return runFinishedMsg{err: err, code: 2} }
	}
	cmd := exec.Command(binary, append([]string{"run"}, args...)...)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return runFinishedMsg{err: err}
	})
}

type filePickerScreen struct {
	baseScreen
	deps   *menuDeps
	onPick func(string) tea.Cmd
	list   selectList
}

func newFilePickerScreen(deps *menuDeps, onPick func(string) tea.Cmd) *filePickerScreen {
	var items []listItem
	for _, path := range deps.candidates() {
		items = append(items, listItem{title: path})
	}
	items = append(items,
		listItem{title: "Escribir otra ruta…"},
		listItem{title: "Volver"},
	)
	list := newSelectList(items)
	if deps.lastYAML != "" {
		for i, item := range items {
			if item.title == deps.lastYAML {
				list.cursor = i
				break
			}
		}
	}
	return &filePickerScreen{deps: deps, onPick: onPick, list: list}
}

func (s *filePickerScreen) Title() string { return "Elegir YAML" }

func (s *filePickerScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return s, nil
	}
	switch key.String() {
	case "esc", "q":
		return s, func() tea.Msg { return popScreenMsg{} }
	case "enter":
		item, ok := s.list.Selected()
		if !ok {
			return s, nil
		}
		switch item.title {
		case "Volver":
			return s, func() tea.Msg { return popScreenMsg{} }
		case "Escribir otra ruta…":
			return s, func() tea.Msg {
				return pushScreenMsg{screen: newPathInputScreen(s.onPick)}
			}
		default:
			return s, s.onPick(item.title)
		}
	}
	s.list.Handle(msg)
	return s, nil
}

func (s *filePickerScreen) View(width, height int) string {
	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Foreground(tui.Bone).Bold(true).Render("Elegir archivo YAML"),
		"",
		s.list.View(width),
	)
}

type pathInputScreen struct {
	baseScreen
	input  *textField
	onPick func(string) tea.Cmd
}

func newPathInputScreen(onPick func(string) tea.Cmd) *pathInputScreen {
	return &pathInputScreen{input: newTextFieldLimit("ruta/al/archivo.yaml", "", 4096), onPick: onPick}
}

func (s *pathInputScreen) Title() string { return "Ruta del YAML" }

func (s *pathInputScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "esc":
			return s, func() tea.Msg { return popScreenMsg{} }
		case "enter":
			path := strings.TrimSpace(s.input.Value())
			if path == "" {
				return s, nil
			}
			return s, s.onPick(path)
		}
	}
	s.input.Update(msg)
	return s, nil
}

func (s *pathInputScreen) View(width, height int) string {
	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Foreground(tui.Bone).Bold(true).Render("Ruta del YAML"),
		"",
		s.input.View(),
		"",
		lipgloss.NewStyle().Foreground(tui.Ash).Render("Enter abrir · Esc volver"),
	)
}

type runScreen struct {
	baseScreen
	deps    *menuDeps
	path    string
	err     error
	summary string
	list    selectList
	options runOptions
}

func newRunScreen(deps *menuDeps, path string) *runScreen {
	if strings.HasPrefix(path, "-") {
		path = "./" + path
	}
	s := &runScreen{
		deps:    deps,
		path:    path,
		options: runOptions{resultsDir: deps.resultsDir},
	}
	cfg, err := validateFile(path)
	s.err = err
	items := []listItem{
		{title: "Ejecutar"},
		{title: "Opciones avanzadas"},
		{title: "Solo validar"},
		{title: "Volver"},
	}
	if s.err == nil {
		s.summary = cfg.Summary()
	} else {
		items = []listItem{{title: "Volver"}}
	}
	s.list = newSelectList(items)
	return s
}

func (s *runScreen) Title() string { return "Ejecutar prueba" }

func (s *runScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return s, nil
	}
	switch key.String() {
	case "esc", "q":
		return s, func() tea.Msg { return popScreenMsg{} }
	case "enter":
		item, ok := s.list.Selected()
		if !ok {
			return s, nil
		}
		if s.err != nil {
			return s, func() tea.Msg { return popScreenMsg{} }
		}
		switch item.title {
		case "Ejecutar":
			result := newRunResultScreen(s.deps, s.path, s.options)
			return s, func() tea.Msg { return pushScreenMsg{screen: result} }
		case "Opciones avanzadas":
			return s, func() tea.Msg {
				return pushScreenMsg{screen: newAdvancedScreen(s)}
			}
		case "Solo validar":
			return s, func() tea.Msg {
				return pushScreenMsg{screen: newTextResultScreen("Validación", "OK  "+s.summary)}
			}
		case "Volver":
			return s, func() tea.Msg { return popScreenMsg{} }
		}
	}
	s.list.Handle(msg)
	return s, nil
}

func (s *runScreen) View(width, height int) string {
	var body strings.Builder
	fmt.Fprintf(&body, "%s\n\n", lipgloss.NewStyle().Foreground(tui.Ash).Render(safetext.Line(s.path)))
	if s.err != nil {
		body.WriteString(lipgloss.NewStyle().Foreground(tui.Ember).Render(safetext.Line(s.err.Error())))
		body.WriteString("\n\n")
	} else {
		body.WriteString(safetext.Multiline(s.summary))
		body.WriteString("\n\n")
	}
	body.WriteString(s.list.View(width))
	return body.String()
}

type advancedScreen struct {
	baseScreen
	run                  *runScreen
	seed, tag, dir       *textField
	noSave, noTUI, debug bool
	focus                int
}

func newAdvancedScreen(run *runScreen) *advancedScreen {
	return &advancedScreen{
		run:    run,
		seed:   newTextField("vacío = YAML", run.options.seed),
		tag:    newTextField("vacía = ninguna", run.options.tag),
		dir:    newTextFieldLimit("directorio", run.options.resultsDir, 4096),
		noSave: run.options.noSave,
		noTUI:  run.options.noTUI,
		debug:  run.options.debug,
	}
}

func (s *advancedScreen) Title() string { return "Opciones avanzadas" }

func (s *advancedScreen) apply() {
	s.run.options.seed = strings.TrimSpace(s.seed.Value())
	s.run.options.tag = strings.TrimSpace(s.tag.Value())
	dir := strings.TrimSpace(s.dir.Value())
	if dir == "" {
		dir = s.run.deps.resultsDir
	}
	s.run.options.resultsDir = dir
	s.run.options.noSave = s.noSave
	s.run.options.noTUI = s.noTUI
	s.run.options.debug = s.debug
}

func (s *advancedScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return s, nil
	}
	switch key.String() {
	case "esc":
		s.apply()
		return s, func() tea.Msg { return popScreenMsg{} }
	case "up":
		if s.focus > 0 {
			s.focus--
		}
		return s, nil
	case "down", "tab":
		if s.focus < 5 {
			s.focus++
		}
		return s, nil
	case "enter":
		switch s.focus {
		case 0, 1, 2:
			if s.focus < 5 {
				s.focus++
			}
		case 3:
			s.noSave = !s.noSave
		case 4:
			s.noTUI = !s.noTUI
		case 5:
			s.debug = !s.debug
		}
		s.apply()
		return s, nil
	}
	switch s.focus {
	case 0:
		s.seed.Update(msg)
	case 1:
		s.tag.Update(msg)
	case 2:
		s.dir.Update(msg)
	}
	s.apply()
	return s, nil
}

func (s *advancedScreen) View(width, height int) string {
	style := lipgloss.NewStyle().Foreground(tui.Bone)
	selected := lipgloss.NewStyle().Foreground(tui.Ember).Bold(true)
	mark := func(index int) string {
		if s.focus == index {
			return selected.Render("▸ ")
		}
		return "  "
	}
	toggle := func(value bool) string {
		if value {
			return "sí"
		}
		return "no"
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		selected.Render("Opciones avanzadas"),
		"",
		mark(0)+style.Render("Semilla: ")+s.seed.View(),
		mark(1)+style.Render("Etiqueta: ")+s.tag.View(),
		mark(2)+style.Render("Directorio: ")+s.dir.View(),
		mark(3)+style.Render("No guardar: "+toggle(s.noSave)),
		mark(4)+style.Render("Sin TUI: "+toggle(s.noTUI)),
		mark(5)+style.Render("Debug: "+toggle(s.debug)),
		"",
		lipgloss.NewStyle().Foreground(tui.Ash).Render("↑↓ mover · Enter editar/alternar · Esc volver"),
	)
}

type textResultScreen struct {
	baseScreen
	title string
	text  string
}

func newTextResultScreen(title, text string) *textResultScreen {
	return &textResultScreen{title: title, text: safetext.Multiline(text)}
}

func (s *textResultScreen) Title() string { return s.title }

func (s *textResultScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "esc", "enter", "q":
			return s, func() tea.Msg { return popScreenMsg{} }
		}
	}
	return s, nil
}

func (s *textResultScreen) View(width, height int) string {
	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Foreground(tui.Bone).Bold(true).Render(s.title),
		"",
		s.text,
		"",
		lipgloss.NewStyle().Foreground(tui.Ash).Render("Enter/Esc volver"),
	)
}

type runResultScreen struct {
	baseScreen
	deps       *menuDeps
	args       []string
	noSave     bool
	resultsDir string
	started    time.Time
	finished   bool
	err        error
	storeErr   error
	code       int
	verdict    string
}

func newRunResultScreen(deps *menuDeps, path string, options runOptions) *runResultScreen {
	resultsDir := options.resultsDir
	if resultsDir == "" {
		resultsDir = deps.resultsDir
	}
	return &runResultScreen{
		deps:       deps,
		args:       buildRunArgs(path, resultsDir, options.seed, options.tag, options.noSave, options.noTUI, options.debug),
		noSave:     options.noSave,
		resultsDir: resultsDir,
		started:    deps.now(),
	}
}

func (s *runResultScreen) Title() string { return "Resultado" }

func (s *runResultScreen) Init() tea.Cmd {
	if s.deps.exec == nil {
		return nil
	}
	return s.deps.exec(s.args)
}

func (s *runResultScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case runFinishedMsg:
		s.finished = true
		s.err = msg.err
		s.code = msg.code
		if s.err != nil && s.code == 0 {
			if exitErr, ok := s.err.(*exec.ExitError); ok {
				s.code = exitErr.ExitCode()
			}
		}
		if !s.noSave {
			store, err := results.OpenStore(s.resultsDir)
			if err != nil {
				s.storeErr = err
			} else if entries, err := store.List(); err != nil {
				s.storeErr = err
			} else {
				for _, entry := range entries {
					if !entry.CreatedAt.Before(s.started) {
						s.verdict = verdictLabel(entry.Verdict)
						break
					}
				}
			}
		}
		return s, nil
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc", "enter", "q":
			return s, func() tea.Msg { return homeMsg{} }
		}
	}
	return s, nil
}

func (s *runResultScreen) View(width, height int) string {
	if !s.finished {
		return lipgloss.NewStyle().Foreground(tui.Bone).Render("Ejecutando corrida…")
	}
	var lines []string
	if s.verdict != "" {
		lines = append(lines, "Veredicto: "+s.verdict)
	}
	if s.err != nil {
		lines = append(lines, "Estado: "+safetext.Line(s.err.Error()))
	}
	if s.storeErr != nil {
		lines = append(lines, "No se pudo leer el historial: "+safetext.Line(s.storeErr.Error()))
	}
	lines = append(lines, fmt.Sprintf("Código: %d", s.code))
	if s.verdict == "" && s.err == nil {
		lines = append(lines, "sin corrida guardada")
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Foreground(tui.Bone).Bold(true).Render("Resultado"),
		"",
		strings.Join(lines, "\n"),
		"",
		lipgloss.NewStyle().Foreground(tui.Ash).Render("Enter/Esc volver al inicio"),
	)
}

func newValidateResultScreen(path string) screen {
	cfg, err := validateFile(path)
	if err != nil {
		return newTextResultScreen("Validación", err.Error())
	}
	return newTextResultScreen("Validación", "OK  "+cfg.Summary())
}
