// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gac982/siafu/internal/vars"
)

// Load reads, decodes, resolves relative TLS paths and validates a file.
func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("abrir configuración %s: %w", path, err)
	}
	defer f.Close()

	cfg, err := Decode(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	cfg.sourceDir = filepath.Dir(path)
	if ca := cfg.Target.TLS.CAFile; ca != "" && !filepath.IsAbs(ca) {
		cfg.Target.TLS.CAFile = filepath.Join(cfg.sourceDir, ca)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}

// ValidationError collects every problem found in a configuration.
type ValidationError struct {
	Problems []string
}

// Error renders all problems as a bullet list.
func (e *ValidationError) Error() string {
	if len(e.Problems) == 1 {
		return "configuración inválida: " + e.Problems[0]
	}
	return "configuración inválida:\n- " + strings.Join(e.Problems, "\n- ")
}

// EnvRefs returns the sorted, unique environment variable names referenced
// by any template in the configuration.
func (c *Config) EnvRefs() []string {
	seen := map[string]bool{}
	add := func(raw string) {
		tpl, err := vars.Compile(raw)
		if err != nil {
			return
		}
		for _, ref := range tpl.Refs() {
			if ref.Kind == vars.KindEnv {
				seen[ref.Name] = true
			}
		}
	}

	for _, value := range c.Target.Headers {
		add(value)
	}
	for _, value := range c.Vars {
		add(value)
	}
	for _, scenario := range c.Scenarios {
		for _, step := range scenario.Steps {
			add(step.Request.URL)
			add(step.Request.Body)
			for _, value := range step.Request.Headers {
				add(value)
			}
		}
	}

	refs := make([]string, 0, len(seen))
	for name := range seen {
		refs = append(refs, name)
	}
	sort.Strings(refs)
	return refs
}

// Summary is a short human-readable description used by `siafu validate`.
func (c *Config) Summary() string {
	var total time.Duration
	peak := 0
	steps := 0
	for _, stage := range c.Load.Stages {
		total += stage.Duration.Value()
		if stage.TargetVUs > peak {
			peak = stage.TargetVUs
		}
	}
	for _, scenario := range c.Scenarios {
		steps += len(scenario.Steps)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", c.Name)
	fmt.Fprintf(&b, "  %d escenario(s), %d paso(s)\n", len(c.Scenarios), steps)
	if c.Load.Mode == ModeOpen {
		var rateTotal time.Duration
		peakRate := 0.0
		for _, stage := range c.Load.OpenLoop.Stages {
			rateTotal += stage.Duration.Value()
			if stage.TargetRate > peakRate {
				peakRate = stage.TargetRate
			}
		}
		fmt.Fprintf(&b, "  modo open · duración %s · in-flight %d · pico %g iteraciones/s\n",
			rateTotal, c.Load.OpenLoop.MaxInFlight, peakRate)
	} else {
		fmt.Fprintf(&b, "  duración %s, pico %d VUs\n", total, peak)
	}
	version := c.Target.HTTPVersion
	if version == "" {
		version = HTTPVersionAuto
	}
	fmt.Fprintf(&b, "  HTTP: %s\n", version)
	fmt.Fprintf(&b, "  umbrales: %s\n", c.thresholdSummary())
	return b.String()
}

func (c *Config) thresholdSummary() string {
	var parts []string
	t := c.Thresholds
	if t.LatencyP95 != nil {
		parts = append(parts, "p95≤"+t.LatencyP95.Value().String())
	}
	if t.LatencyP99 != nil {
		parts = append(parts, "p99≤"+t.LatencyP99.Value().String())
	}
	if t.ErrorRate != nil {
		parts = append(parts, fmt.Sprintf("errores≤%.2f%%", *t.ErrorRate*100))
	}
	if t.MinRPS != nil {
		parts = append(parts, fmt.Sprintf("rps≥%.1f", *t.MinRPS))
	}
	if len(parts) == 0 {
		return "sin umbrales"
	}
	return strings.Join(parts, " · ")
}
