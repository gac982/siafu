// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package config

import (
	"math"
	"strings"
	"testing"
	"time"
)

const openLoopYAML = `
version: 1
name: open-loop
target:
  base_url: http://localhost:8080
load:
  mode: open
  open_loop:
    stages:
      - {duration: 1s, target_rate: 1000}
      - {duration: 2s, target_rate: 5000.5}
      - {duration: 1s, target_rate: 0}
    max_in_flight: 8000
    on_overload: drop
    on_analysis_exhausted: skip
scenarios:
  - name: s
    steps:
      - name: one
        request: {url: /}
`

func TestDecodeOpenLoop(t *testing.T) {
	cfg, err := Decode(strings.NewReader(openLoopYAML))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Load.Mode != ModeOpen {
		t.Fatalf("mode = %q, want %q", cfg.Load.Mode, ModeOpen)
	}
	if len(cfg.Load.OpenLoop.Stages) != 3 {
		t.Fatalf("stages = %+v", cfg.Load.OpenLoop.Stages)
	}
	if cfg.Load.OpenLoop.Stages[0].TargetRate != 1000 {
		t.Errorf("first rate = %v", cfg.Load.OpenLoop.Stages[0].TargetRate)
	}
	if cfg.Load.OpenLoop.Stages[1].TargetRate != 5000.5 {
		t.Errorf("second rate = %v", cfg.Load.OpenLoop.Stages[1].TargetRate)
	}
	if cfg.Load.OpenLoop.MaxInFlight != 8000 {
		t.Errorf("max_in_flight = %d", cfg.Load.OpenLoop.MaxInFlight)
	}
	if cfg.Load.OpenLoop.OnOverload != OverloadDrop {
		t.Errorf("on_overload = %q", cfg.Load.OpenLoop.OnOverload)
	}
	if cfg.Load.OpenLoop.OnAnalysisExhausted != AnalysisSkip {
		t.Errorf("on_analysis_exhausted = %q", cfg.Load.OpenLoop.OnAnalysisExhausted)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
}

func TestOpenLoopDefaults(t *testing.T) {
	cfg, err := Decode(strings.NewReader(validYAML))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Load.Mode != ModeClosed {
		t.Errorf("mode = %q, want %q", cfg.Load.Mode, ModeClosed)
	}
	if cfg.Load.OpenLoop.OnOverload != OverloadDrop {
		t.Errorf("on_overload = %q", cfg.Load.OpenLoop.OnOverload)
	}
	if cfg.Load.OpenLoop.OnAnalysisExhausted != AnalysisSkip {
		t.Errorf("on_analysis_exhausted = %q", cfg.Load.OpenLoop.OnAnalysisExhausted)
	}
}

func TestValidateOpenLoopRules(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{"unknown mode", func(c *Config) { c.Load.Mode = "turbo" }, "mode"},
		{"no open stages", func(c *Config) { c.Load.OpenLoop.Stages = nil }, "open_loop.stages"},
		{"zero duration", func(c *Config) { c.Load.OpenLoop.Stages[0].Duration = Duration{} }, "duration"},
		{"negative rate", func(c *Config) { c.Load.OpenLoop.Stages[0].TargetRate = -1 }, "target_rate"},
		{"nan rate", func(c *Config) { c.Load.OpenLoop.Stages[0].TargetRate = math.NaN() }, "target_rate"},
		{"infinite rate", func(c *Config) { c.Load.OpenLoop.Stages[0].TargetRate = math.Inf(1) }, "target_rate"},
		{"huge rate", func(c *Config) { c.Load.OpenLoop.Stages[0].TargetRate = 1_000_001 }, "target_rate"},
		{"all zero rates", func(c *Config) {
			for i := range c.Load.OpenLoop.Stages {
				c.Load.OpenLoop.Stages[i].TargetRate = 0
			}
		}, "target_rate"},
		{"max in flight low", func(c *Config) { c.Load.OpenLoop.MaxInFlight = 0 }, "max_in_flight"},
		{"max in flight high", func(c *Config) { c.Load.OpenLoop.MaxInFlight = 20001 }, "max_in_flight"},
		{"overload queue", func(c *Config) { c.Load.OpenLoop.OnOverload = "queue" }, "on_overload"},
		{"analysis ignore", func(c *Config) { c.Load.OpenLoop.OnAnalysisExhausted = "ignore" }, "on_analysis_exhausted"},
		{"too long", func(c *Config) { c.Load.OpenLoop.Stages[0].Duration = Duration{Duration: 7 * time.Hour} }, "6h"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := Decode(strings.NewReader(openLoopYAML))
			if err != nil {
				t.Fatal(err)
			}
			tc.mutate(cfg)
			err = cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate() = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestSummaryOpenLoop(t *testing.T) {
	cfg, err := Decode(strings.NewReader(openLoopYAML))
	if err != nil {
		t.Fatal(err)
	}
	summary := cfg.Summary()
	for _, want := range []string{"open", "8000", "5000"} {
		if !strings.Contains(summary, want) {
			t.Errorf("summary = %q, want %q", summary, want)
		}
	}
}
