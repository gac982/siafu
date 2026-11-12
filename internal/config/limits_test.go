// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package config

import (
	"strings"
	"testing"
)

const limitsDoc = `
version: 1
name: limits
target:
  base_url: http://localhost:8080
load:
  stages:
    - duration: 1s
      target_vus: 1
limits:
  memory_budget_mb: 300
  warn_at_percent: 75
  analysis_budget_mb: 32
  max_log_lines: 500
  max_batch_samples: 32
  max_idle_conns: 256
scenarios:
  - name: s
    steps:
      - name: one
        request:
          url: /
`

func TestDecodeLimits(t *testing.T) {
	cfg, err := Decode(strings.NewReader(limitsDoc))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Limits.MemoryBudgetMB != 300 || cfg.Limits.WarnAtPercent != 75 {
		t.Errorf("limits = %+v", cfg.Limits)
	}
	if cfg.Limits.AnalysisBudgetMB != 32 || cfg.Limits.MaxLogLines != 500 {
		t.Errorf("limits = %+v", cfg.Limits)
	}
	if cfg.Limits.MaxBatchSamples != 32 || cfg.Limits.MaxIdleConns != 256 {
		t.Errorf("limits = %+v", cfg.Limits)
	}
}

func TestLimitsDefaults(t *testing.T) {
	cfg, err := Decode(strings.NewReader(`
version: 1
name: defaults
target: {base_url: http://localhost:8080}
load: {stages: [{duration: 1s, target_vus: 1}]}
scenarios: [{name: s, steps: [{name: one, request: {url: /}}]}]
`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Limits.MemoryBudgetMB != 450 || cfg.Limits.WarnAtPercent != 80 {
		t.Errorf("default limits = %+v", cfg.Limits)
	}
	if cfg.Limits.AnalysisBudgetMB != 64 || cfg.Limits.MaxLogLines != 1000 {
		t.Errorf("default limits = %+v", cfg.Limits)
	}
	if cfg.Limits.MaxBatchSamples != 64 || cfg.Limits.MaxIdleConns != 1024 {
		t.Errorf("default limits = %+v", cfg.Limits)
	}
}

func TestValidateLimits(t *testing.T) {
	cases := []struct {
		name string
		edit func(*Config)
		want string
	}{
		{"budget low", func(c *Config) { c.Limits.MemoryBudgetMB = 0 }, "memory_budget_mb"},
		{"budget high", func(c *Config) { c.Limits.MemoryBudgetMB = 1025 }, "memory_budget_mb"},
		{"warning low", func(c *Config) { c.Limits.WarnAtPercent = 49 }, "warn_at_percent"},
		{"warning high", func(c *Config) { c.Limits.WarnAtPercent = 96 }, "warn_at_percent"},
		{"analysis low", func(c *Config) { c.Limits.AnalysisBudgetMB = 0 }, "analysis_budget_mb"},
		{"analysis high", func(c *Config) { c.Limits.AnalysisBudgetMB = 129 }, "analysis_budget_mb"},
		{"logs low", func(c *Config) { c.Limits.MaxLogLines = 99 }, "max_log_lines"},
		{"logs high", func(c *Config) { c.Limits.MaxLogLines = 10001 }, "max_log_lines"},
		{"batch low", func(c *Config) { c.Limits.MaxBatchSamples = 0 }, "max_batch_samples"},
		{"batch high", func(c *Config) { c.Limits.MaxBatchSamples = 513 }, "max_batch_samples"},
		{"idle low", func(c *Config) { c.Limits.MaxIdleConns = 0 }, "max_idle_conns"},
		{"idle high", func(c *Config) { c.Limits.MaxIdleConns = 4097 }, "max_idle_conns"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := Decode(strings.NewReader(limitsDoc))
			if err != nil {
				t.Fatal(err)
			}
			tc.edit(cfg)
			err = cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}
