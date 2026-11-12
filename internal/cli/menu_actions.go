// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package cli

import (
	"errors"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gac982/siafu/internal/report"
)

type historyFilters struct {
	mode    string
	verdict string
	query   string
}

func buildRunArgs(path, resultsDir, seed, tag string, noSave, noTUI, debug bool) []string {
	args := []string{path, "--results-dir", resultsDir}
	if tag != "" {
		args = append(args, "--tag", tag)
	}
	if seed != "" {
		args = append(args, "--seed", seed)
	}
	if noSave {
		args = append(args, "--no-save")
	}
	if noTUI {
		args = append(args, "--no-tui")
	}
	if debug {
		args = append(args, "--debug")
	}
	return args
}

func configCandidates() []string {
	patterns := []string{"*.yaml", "*.yml", "examples/*.yaml", "examples/*.yml"}
	seen := make(map[string]bool)
	var candidates []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, match := range matches {
			if strings.HasPrefix(match, "-") {
				match = "./" + match
			}
			if seen[match] {
				continue
			}
			seen[match] = true
			candidates = append(candidates, match)
		}
	}
	sort.Strings(candidates)
	return candidates
}

func verdictLabel(value string) string {
	if value == "" {
		return "—"
	}
	return report.Verdict(value).Label()
}

func modeLabel(value string) string {
	switch strings.ToLower(value) {
	case "closed":
		return "cerrado"
	case "open":
		return "abierto"
	case "":
		return "—"
	default:
		return value
	}
}

func openInBrowser(url string) error {
	var lastErr error
	for _, command := range []string{"xdg-open", "open"} {
		cmd := exec.Command(command, url)
		if err := cmd.Start(); err == nil {
			_ = cmd.Process.Release()
			return nil
		} else {
			lastErr = err
		}
	}
	if lastErr == nil {
		lastErr = errors.New("no se encontró un comando para abrir el navegador")
	}
	return lastErr
}
