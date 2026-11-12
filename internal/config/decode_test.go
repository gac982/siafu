// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package config

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestDecodeFullConfig(t *testing.T) {
	f, err := os.Open("testdata/full.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	cfg, err := Decode(f)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	if cfg.Version != 1 || cfg.Name != "Prueba completa" {
		t.Errorf("version/name = %d/%q", cfg.Version, cfg.Name)
	}
	if cfg.Target.BaseURL != "https://api.example.com" {
		t.Errorf("base_url = %q", cfg.Target.BaseURL)
	}
	if got := cfg.Target.Timeout.Value(); got != 12*time.Second {
		t.Errorf("timeout = %v, want 12s", got)
	}
	if cfg.Target.HTTPVersion != "2" {
		t.Errorf("http_version = %q, want 2", cfg.Target.HTTPVersion)
	}
	if !cfg.Target.TLS.InsecureSkipVerify || cfg.Target.TLS.CAFile != "/tmp/ca.pem" {
		t.Errorf("tls = %+v", cfg.Target.TLS)
	}
	if cfg.Load.MaxVUs != 500 || cfg.Load.Seed == nil || *cfg.Load.Seed != 42 {
		t.Errorf("load = %+v", cfg.Load)
	}
	if cfg.Load.ThinkTime.Min.Value() != 100*time.Millisecond || cfg.Load.ThinkTime.Max.Value() != 300*time.Millisecond {
		t.Errorf("think_time = %+v", cfg.Load.ThinkTime)
	}
	if len(cfg.Load.Stages) != 2 || cfg.Load.Stages[1].TargetVUs != 50 {
		t.Errorf("stages = %+v", cfg.Load.Stages)
	}
	if got := cfg.Load.Stages[1].Duration.Value(); got != 20*time.Second {
		t.Errorf("stage duration = %v", got)
	}
	if cfg.Vars["email"] != "load@example.com" {
		t.Errorf("vars = %+v", cfg.Vars)
	}
	if len(cfg.Scenarios) != 1 || len(cfg.Scenarios[0].Steps) != 1 {
		t.Fatalf("scenarios = %+v", cfg.Scenarios)
	}
	step := cfg.Scenarios[0].Steps[0]
	if step.Request.Method != "GET" || step.Request.URL != "/items" || step.Request.Headers["X-Test"] != "1" {
		t.Errorf("request = %+v", step.Request)
	}
	if len(step.Expect.Status) != 2 || step.Expect.Status[1] != 304 || step.Expect.BodyContains != "items" {
		t.Errorf("expect = %+v", step.Expect)
	}
	if got := step.Expect.MaxDuration.Value(); got != 2*time.Second {
		t.Errorf("max_duration = %v", got)
	}
	if step.Extract["id"] != "json:items.0.id" {
		t.Errorf("extract = %+v", step.Extract)
	}
	if cfg.Thresholds.LatencyP95 == nil || cfg.Thresholds.LatencyP95.Value() != 300*time.Millisecond {
		t.Errorf("thresholds = %+v", cfg.Thresholds)
	}
	if cfg.Thresholds.ErrorRate == nil || *cfg.Thresholds.ErrorRate != 0.01 {
		t.Errorf("error_rate = %v", cfg.Thresholds.ErrorRate)
	}
}

func TestDecodeDefaults(t *testing.T) {
	const doc = `
version: 1
name: minimo
target:
  base_url: http://localhost:8080
load:
  stages:
    - duration: 1s
      target_vus: 1
scenarios:
  - name: s
    steps:
      - name: one
        request:
          url: /
`
	cfg, err := Decode(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if got := cfg.Target.Timeout.Value(); got != 30*time.Second {
		t.Errorf("default timeout = %v, want 30s", got)
	}
	if cfg.Target.HTTPVersion != "auto" {
		t.Errorf("default http_version = %q, want auto", cfg.Target.HTTPVersion)
	}
	if cfg.Load.MaxVUs != 2000 {
		t.Errorf("default max_vus = %d, want 2000", cfg.Load.MaxVUs)
	}
	if got := cfg.Scenarios[0].Steps[0].Request.Method; got != "GET" {
		t.Errorf("default method = %q, want GET", got)
	}
}

func TestThinkTimeScalarAndRange(t *testing.T) {
	base := `
version: 1
name: t
target: {base_url: "http://x"}
load:
  %s
  stages: [{duration: 1s, target_vus: 1}]
scenarios: [{name: s, steps: [{name: one, request: {url: /}}]}]
`
	scalar, err := Decode(strings.NewReader(fmt.Sprintf(base, "think_time: 500ms")))
	if err != nil {
		t.Fatalf("scalar: %v", err)
	}
	if scalar.Load.ThinkTime.Min.Value() != 500*time.Millisecond || scalar.Load.ThinkTime.Max.Value() != 500*time.Millisecond {
		t.Errorf("scalar think_time = %+v", scalar.Load.ThinkTime)
	}

	ranged, err := Decode(strings.NewReader(fmt.Sprintf(base, "think_time: {min: 100ms, max: 200ms}")))
	if err != nil {
		t.Fatalf("range: %v", err)
	}
	if ranged.Load.ThinkTime.Min.Value() != 100*time.Millisecond || ranged.Load.ThinkTime.Max.Value() != 200*time.Millisecond {
		t.Errorf("range think_time = %+v", ranged.Load.ThinkTime)
	}
}

func TestThinkTimeUnknownKey(t *testing.T) {
	const doc = `
version: 1
name: t
target: {base_url: "http://x"}
load:
  think_time: {min: 1s, max: 2s, nope: 1}
  stages: [{duration: 1s, target_vus: 1}]
scenarios: [{name: s, steps: [{name: one, request: {url: /}}]}]
`
	_, err := Decode(strings.NewReader(doc))
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("error = %v, want unknown think_time key", err)
	}
}

func TestStatusCodesIntAndList(t *testing.T) {
	base := `
version: 1
name: t
target: {base_url: "http://x"}
load:
  stages: [{duration: 1s, target_vus: 1}]
scenarios:
  - name: s
    steps:
      - name: one
        request: {url: /}
        expect: {status: %s}
`
	for raw, want := range map[string][]int{
		"200":       {200},
		"[200,304]": {200, 304},
	} {
		cfg, err := Decode(strings.NewReader(strings.Replace(base, "%s", raw, 1)))
		if err != nil {
			t.Fatalf("status %s: %v", raw, err)
		}
		got := cfg.Scenarios[0].Steps[0].Expect.Status
		if len(got) != len(want) {
			t.Fatalf("status %s = %v, want %v", raw, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("status %s = %v, want %v", raw, got, want)
			}
		}
	}
}

func TestDecodeUnknownField(t *testing.T) {
	_, err := Decode(strings.NewReader("version: 1\nnope: true\n"))
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("error = %v, want unknown field nope", err)
	}
}

func TestDecodeEmpty(t *testing.T) {
	_, err := Decode(strings.NewReader("  \n\t\n"))
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "vac") {
		t.Fatalf("error = %v, want empty document error", err)
	}
}

func TestDecodeMultipleDocuments(t *testing.T) {
	_, err := Decode(strings.NewReader("version: 1\n---\nversion: 1\n"))
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "document") {
		t.Fatalf("error = %v, want multiple document error", err)
	}
}

func TestDecodeBadDuration(t *testing.T) {
	_, err := Decode(strings.NewReader("version: 1\ntarget: {base_url: http://x, timeout: nope}\n"))
	if err == nil || !strings.Contains(err.Error(), "duración") {
		t.Fatalf("error = %v, want duration error", err)
	}
}
