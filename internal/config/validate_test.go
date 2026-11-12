// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const validYAML = `
version: 1
name: prueba
target:
  base_url: http://localhost:8080
  headers:
    Accept: application/json
load:
  stages:
    - duration: 10s
      target_vus: 5
  think_time: 100ms
vars:
  who: "vu-${VU}"
scenarios:
  - name: s
    steps:
      - name: one
        request:
          url: /items/${who}
        expect:
          status: 200
thresholds:
  latency_p95: 300ms
  error_rate: 0.01
  min_rps: 1
`

func validConfig(t *testing.T) *Config {
	t.Helper()
	cfg, err := Decode(strings.NewReader(validYAML))
	if err != nil {
		t.Fatalf("Decode(validYAML) error = %v", err)
	}
	return cfg
}

func TestValidateValid(t *testing.T) {
	if err := validConfig(t).Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateMaxVUsCapacity(t *testing.T) {
	tests := []struct {
		name      string
		maxVUs    int
		targetVUs int
		wantErr   string
	}{
		{"15000", 15000, 15000, ""},
		{"20000", 20000, 20000, ""},
		{"20001", 20001, 20000, "max_vus"},
		{"target above max", 15000, 15001, "target_vus"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig(t)
			cfg.Load.MaxVUs = tc.maxVUs
			cfg.Load.Stages[0].TargetVUs = tc.targetVUs
			err := cfg.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Validate() = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestValidateRules(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{"version", func(c *Config) { c.Version = 2 }, "version"},
		{"empty name", func(c *Config) { c.Name = "" }, "name"},
		{"long name", func(c *Config) { c.Name = strings.Repeat("a", 81) }, "name"},
		{"base url missing", func(c *Config) { c.Target.BaseURL = "" }, "base_url"},
		{"base url scheme", func(c *Config) { c.Target.BaseURL = "ftp://x" }, "base_url"},
		{"base url query", func(c *Config) { c.Target.BaseURL = "http://x?a=1" }, "base_url"},
		{"base url template", func(c *Config) { c.Target.BaseURL = "http://${ENV:HOST}" }, "base_url"},
		{"timeout low", func(c *Config) { c.Target.Timeout.Duration = 500 * time.Millisecond }, "timeout"},
		{"timeout high", func(c *Config) { c.Target.Timeout.Duration = 301 * time.Second }, "timeout"},
		{"header CRLF", func(c *Config) { c.Target.Headers["X"] = "a\r\nb" }, "header"},
		{"no stages", func(c *Config) { c.Load.Stages = nil }, "stages"},
		{"zero stage", func(c *Config) { c.Load.Stages[0].Duration = Duration{} }, "stages[0].duration"},
		{"no load", func(c *Config) { c.Load.Stages[0].TargetVUs = 0 }, "target_vus"},
		{"max vus", func(c *Config) { c.Load.Stages[0].TargetVUs = 3000 }, "max_vus"},
		{"think inverted", func(c *Config) {
			c.Load.ThinkTime.Min.Duration = 2 * time.Second
			c.Load.ThinkTime.Max.Duration = time.Second
		}, "think_time"},
		{"no scenarios", func(c *Config) { c.Scenarios = nil }, "scenarios"},
		{"duplicate scenario", func(c *Config) { c.Scenarios = append(c.Scenarios, c.Scenarios[0]) }, "duplicad"},
		{"scenario name", func(c *Config) { c.Scenarios[0].Name = "bad name" }, "scenarios[0].name"},
		{"no steps", func(c *Config) { c.Scenarios[0].Steps = nil }, "steps"},
		{"duplicate step", func(c *Config) { c.Scenarios[0].Steps = append(c.Scenarios[0].Steps, c.Scenarios[0].Steps[0]) }, "duplicad"},
		{"bad method", func(c *Config) { c.Scenarios[0].Steps[0].Request.Method = "TRACE" }, "method"},
		{"missing url", func(c *Config) { c.Scenarios[0].Steps[0].Request.URL = "" }, "url"},
		{"unknown template", func(c *Config) { c.Scenarios[0].Steps[0].Request.URL = "/${nope}" }, "nope"},
		{"bad status", func(c *Config) { c.Scenarios[0].Steps[0].Expect.Status = StatusCodes{99} }, "status"},
		{"bad extract", func(c *Config) { c.Scenarios[0].Steps[0].Extract = map[string]string{"id": "yaml:x"} }, "extract"},
		{"extract collision", func(c *Config) {
			c.Scenarios[0].Steps[0].Extract = map[string]string{"who": "json:id"}
		}, "who"},
		{"global ref global", func(c *Config) {
			c.Vars["a"] = "${b}"
			c.Vars["b"] = "x"
		}, "vars"},
		{"var name", func(c *Config) { c.Vars["bad name"] = "x" }, "vars"},
		{"var builtin", func(c *Config) { c.Vars["VU"] = "x" }, "VU"},
		{"p99 lt p95", func(c *Config) {
			c.Thresholds.LatencyP95 = &Duration{Duration: time.Second}
			c.Thresholds.LatencyP99 = &Duration{Duration: time.Second / 2}
		}, "latency_p99"},
		{"error rate", func(c *Config) {
			v := 1.5
			c.Thresholds.ErrorRate = &v
		}, "error_rate"},
		{"min rps", func(c *Config) {
			v := -1.0
			c.Thresholds.MinRPS = &v
		}, "min_rps"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig(t)
			tt.mutate(cfg)
			err := cfg.Validate()
			if err == nil {
				t.Fatalf("Validate() = nil, want error containing %q", tt.want)
			}
			if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.want)) {
				t.Errorf("Validate() error = %q, want substring %q", err, tt.want)
			}
		})
	}
}

func TestValidateMethodNormalized(t *testing.T) {
	cfg := validConfig(t)
	cfg.Scenarios[0].Steps[0].Request.Method = "post"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if got := cfg.Scenarios[0].Steps[0].Request.Method; got != "POST" {
		t.Errorf("method = %q, want POST", got)
	}
}

func TestValidateForwardExtract(t *testing.T) {
	cfg := validConfig(t)
	cfg.Scenarios[0].Steps[0].Request.URL = "/${later}"
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "later") {
		t.Fatalf("error = %v, want forward reference error", err)
	}
}

func TestValidateExtractVisibleToLaterStep(t *testing.T) {
	cfg := validConfig(t)
	cfg.Scenarios[0].Steps = append(cfg.Scenarios[0].Steps, Step{
		Name:    "two",
		Request: Request{Method: "GET", URL: "/${id}"},
	})
	cfg.Scenarios[0].Steps[0].Extract = map[string]string{"id": "json:data.id"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateExamples(t *testing.T) {
	for _, path := range []string{
		"../../examples/pages.yaml",
		"../../examples/api.yaml",
		"../../examples/demo.yaml",
		"../../examples/failing.yaml",
	} {
		if _, err := Load(path); err != nil {
			t.Errorf("Load(%s) error = %v", path, err)
		}
	}
}

func TestValidateCAFileMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	doc := `
version: 1
name: ca
target:
  base_url: https://localhost:8443
  tls:
    ca_file: missing.pem
load:
  stages: [{duration: 1s, target_vus: 1}]
scenarios:
  - name: s
    steps:
      - name: one
        request: {url: /}
`
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "ca_file") {
		t.Fatalf("error = %v, want ca_file error", err)
	}
}

func TestEnvRefs(t *testing.T) {
	cfg := validConfig(t)
	cfg.Target.Headers = map[string]string{"X": "${ENV:ONE}"}
	cfg.Scenarios[0].Steps[0].Request.Headers = map[string]string{"Y": "${ENV:TWO}"}
	cfg.Scenarios[0].Steps[0].Request.Body = "${ENV:ONE} ${ENV:THREE}"
	refs := cfg.EnvRefs()
	if strings.Join(refs, ",") != "ONE,THREE,TWO" {
		t.Errorf("EnvRefs() = %v", refs)
	}
}

func TestSummary(t *testing.T) {
	summary := validConfig(t).Summary()
	for _, want := range []string{"prueba", "1 escenario", "1 paso", "10s", "5 VUs", "p95"} {
		if !strings.Contains(summary, want) {
			t.Errorf("Summary() = %q, want %q", summary, want)
		}
	}
}

func TestValidateTargetHeaderTemplate(t *testing.T) {
	cfg := validConfig(t)
	cfg.Target.Headers["X-Bad"] = "${sin cerrar"
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "X-Bad") {
		t.Fatalf("error = %v, want target header template error", err)
	}
}

func TestValidateTargetHeaderUnknownVariable(t *testing.T) {
	cfg := validConfig(t)
	cfg.Target.Headers["X-Bad"] = "${nope}"
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("error = %v, want unknown target header variable", err)
	}
}

func TestHTTPVersionValidation(t *testing.T) {
	tests := []struct {
		name    string
		version string
		baseURL string
		wantErr string
	}{
		{"auto", "auto", "https://localhost:8443", ""},
		{"http1", "1.1", "https://localhost:8443", ""},
		{"http2", "2", "https://localhost:8443", ""},
		{"http2 plain", "2", "http://localhost:8080", "http_version"},
		{"http3", "3", "https://localhost:8443", ""},
		{"http3 plain", "3", "http://localhost:8080", "http_version"},
		{"unknown", "4", "https://localhost:8443", "http_version"},
		{"empty defaults to auto", "", "http://localhost:8080", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig(t)
			cfg.Target.HTTPVersion = tc.version
			cfg.Target.BaseURL = tc.baseURL
			err := cfg.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Validate() = %v, want error containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestSummaryIncludesHTTPVersion(t *testing.T) {
	for _, version := range []string{"1.1", "2", "3"} {
		cfg := validConfig(t)
		cfg.Target.HTTPVersion = version
		if summary := cfg.Summary(); !strings.Contains(summary, "HTTP: "+version) {
			t.Errorf("Summary() = %q, want HTTP: %s", summary, version)
		}
	}
}
