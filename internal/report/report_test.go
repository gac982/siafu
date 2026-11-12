// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package report

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/gac982/siafu/internal/config"
	"github.com/gac982/siafu/internal/metrics"
	"github.com/gac982/siafu/internal/sysmetrics"
)

func durationPtr(d time.Duration) *config.Duration {
	return &config.Duration{Duration: d}
}

func floatPtr(v float64) *float64 { return &v }

func baseConfig() *config.Config {
	return &config.Config{
		Name:   "Prueba",
		Target: config.Target{BaseURL: "http://localhost:8080"},
		Load: config.LoadConfig{Stages: []config.Stage{
			{Duration: config.Duration{Duration: time.Minute}, TargetVUs: 10},
		}},
		Thresholds: config.Thresholds{
			LatencyP95: durationPtr(300 * time.Millisecond),
			LatencyP99: durationPtr(time.Second),
			ErrorRate:  floatPtr(0.01),
			MinRPS:     floatPtr(10),
		},
	}
}

func baseSnapshot() metrics.Snapshot {
	return metrics.Snapshot{
		StartedAt:    time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
		Elapsed:      time.Minute,
		Total:        1000,
		Failed:       5,
		ErrorRate:    0.005,
		RPSAvg:       100,
		RPSRecent:    95,
		RPSPeak:      120,
		RPSPeakAt:    time.Second,
		Latency:      metrics.Percentiles{P50: 50 * time.Millisecond, P95: 200 * time.Millisecond, P99: 500 * time.Millisecond},
		ChecksTotal:  1000,
		ChecksPassed: 995,
		Iterations:   300,
		StatusCodes:  map[int]int64{200: 995, 500: 5},
		Errors:       []metrics.ErrorCount{{Kind: "timeout", Count: 2}},
		Scenarios: []metrics.ScenarioSnapshot{
			{Name: "a", Total: 600, Failed: 2, ErrorRate: 0.003, RPSAvg: 60, Latency: metrics.Percentiles{P95: 180 * time.Millisecond}},
			{Name: "b", Total: 400, Failed: 3, ErrorRate: 0.0075, RPSAvg: 40, Latency: metrics.Percentiles{P95: 230 * time.Millisecond}},
		},
	}
}

func build(cfg *config.Config, snap metrics.Snapshot, aborted bool) Report {
	return Build(snap, cfg, sysmetrics.Summary{CPUAvg: 20, CPUMax: 40, MemMaxPercent: 50}, Meta{
		Seed: 7, PeakVUs: 10, Planned: time.Minute, Actual: time.Minute, Aborted: aborted,
	})
}

func TestBuildPass(t *testing.T) {
	rep := build(baseConfig(), baseSnapshot(), false)
	if rep.Verdict != VerdictPass {
		t.Fatalf("verdict = %s, want PASS (%+v)", rep.Verdict, rep.Thresholds)
	}
	if rep.ExitCode() != 0 {
		t.Errorf("exit = %d", rep.ExitCode())
	}
	if len(rep.Thresholds) != 4 {
		t.Errorf("thresholds = %+v", rep.Thresholds)
	}
}

func TestBuildFailLatency(t *testing.T) {
	cfg := baseConfig()
	snap := baseSnapshot()
	snap.Latency.P95 = 500 * time.Millisecond
	rep := build(cfg, snap, false)
	if rep.Verdict != VerdictFail || rep.ExitCode() != 1 {
		t.Fatalf("verdict/exit = %s/%d", rep.Verdict, rep.ExitCode())
	}
}

func TestBuildFailErrorRate(t *testing.T) {
	cfg := baseConfig()
	snap := baseSnapshot()
	snap.Failed = 100
	snap.ErrorRate = 0.1
	rep := build(cfg, snap, false)
	if rep.Verdict != VerdictFail {
		t.Fatalf("verdict = %s", rep.Verdict)
	}
}

func TestBuildFailMinRPS(t *testing.T) {
	cfg := baseConfig()
	snap := baseSnapshot()
	snap.Total = 100
	snap.Failed = 0
	snap.ErrorRate = 0
	snap.RPSAvg = 0.5
	rep := build(cfg, snap, false)
	if rep.Verdict != VerdictFail {
		t.Fatalf("verdict = %s (RPS promedio %v)", rep.Verdict, rep.Global.RPSAvg)
	}
}

func TestBuildUsesPlannedDurationForRPS(t *testing.T) {
	cfg := baseConfig()
	minRPS := 1.0
	cfg.Thresholds = config.Thresholds{MinRPS: &minRPS}
	snap := baseSnapshot()
	snap.Total = 100
	snap.RPSAvg = 0.5
	snap.Elapsed = 2 * time.Minute
	rep := build(cfg, snap, false)
	if rep.Verdict != VerdictPass {
		t.Fatalf("verdict = %s; RPS threshold must use planned duration", rep.Verdict)
	}
}

func TestBuildMasksTargetUserinfo(t *testing.T) {
	cfg := baseConfig()
	cfg.Target.BaseURL = "https://user:supersecret@example.com/api"
	rep := build(cfg, baseSnapshot(), false)
	if strings.Contains(rep.Target, "supersecret") {
		t.Errorf("report target leaks password: %q", rep.Target)
	}
}

func TestBuildNotEvaluated(t *testing.T) {
	cfg := baseConfig()
	cfg.Thresholds = config.Thresholds{}
	rep := build(cfg, baseSnapshot(), false)
	if rep.Verdict != VerdictNotEvaluated || rep.ExitCode() != 0 {
		t.Fatalf("verdict/exit = %s/%d", rep.Verdict, rep.ExitCode())
	}
	if len(rep.Thresholds) != 0 {
		t.Errorf("thresholds = %+v", rep.Thresholds)
	}
}

func TestBuildAbortedSkipsThresholds(t *testing.T) {
	rep := build(baseConfig(), baseSnapshot(), true)
	if rep.Verdict != VerdictAborted || rep.ExitCode() != 130 {
		t.Fatalf("verdict/exit = %s/%d", rep.Verdict, rep.ExitCode())
	}
	if len(rep.Thresholds) != 0 {
		t.Errorf("thresholds = %+v", rep.Thresholds)
	}
}

func TestBuildZeroRequestsIsError(t *testing.T) {
	snap := baseSnapshot()
	snap.Total = 0
	rep := build(baseConfig(), snap, false)
	if rep.Verdict != VerdictError || rep.ExitCode() != 3 {
		t.Fatalf("verdict/exit = %s/%d", rep.Verdict, rep.ExitCode())
	}
}

func TestVerdictLabels(t *testing.T) {
	cases := map[Verdict]string{
		VerdictPass:         "APROBADA",
		VerdictFail:         "FALLIDA",
		VerdictNotEvaluated: "SIN UMBRALES",
		VerdictAborted:      "ABORTADA",
		VerdictError:        "ERROR",
	}
	for verdict, want := range cases {
		if got := verdict.Label(); got != want {
			t.Errorf("%s.Label() = %q, want %q", verdict, got, want)
		}
	}
}

func TestTextContainsSections(t *testing.T) {
	rep := build(baseConfig(), baseSnapshot(), false)
	var out bytes.Buffer
	if err := WriteText(&out, rep); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{
		"SIAFU", "RESUMEN GLOBAL", "POR ESCENARIO", "CÓDIGOS DE ESTADO",
		"ERRORES Y CHECKS", "UMBRALES", "GENERADOR", "APROBADA", "p95",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("text does not contain %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "\x1b[") {
		t.Errorf("text report contains ANSI escapes")
	}
}

func TestTextMarksPassFail(t *testing.T) {
	cfg := baseConfig()
	snap := baseSnapshot()
	snap.Latency.P95 = time.Second
	var fail bytes.Buffer
	if err := WriteText(&fail, build(cfg, snap, false)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fail.String(), "FALLIDA") || !strings.Contains(fail.String(), "✗") {
		t.Errorf("fail report missing verdict/mark:\n%s", fail.String())
	}

	var pass bytes.Buffer
	if err := WriteText(&pass, build(baseConfig(), baseSnapshot(), false)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(pass.String(), "APROBADA") || !strings.Contains(pass.String(), "✓") {
		t.Errorf("pass report missing verdict/mark:\n%s", pass.String())
	}
}

func TestBuildMemoryExceededIsError(t *testing.T) {
	rep := Build(baseSnapshot(), baseConfig(), sysmetrics.Summary{}, Meta{
		Seed: 1, Planned: time.Minute, Actual: time.Minute, MemoryExceeded: true,
	})
	if rep.Verdict != VerdictError || rep.ExitCode() != 3 {
		t.Fatalf("verdict/exit = %s/%d, want ERROR/3", rep.Verdict, rep.ExitCode())
	}
	if !rep.MemoryExceeded {
		t.Error("MemoryExceeded = false")
	}
	if len(rep.Diagnosis) == 0 || !strings.Contains(rep.Diagnosis[0].Message, "límite de memoria") {
		t.Errorf("diagnosis = %+v", rep.Diagnosis)
	}
}

func TestOpenLoopReport(t *testing.T) {
	rep := Build(baseSnapshot(), baseConfig(), sysmetrics.Summary{}, Meta{
		Seed: 1, Planned: time.Minute, Actual: time.Minute,
		OpenLoop: &OpenLoopSummary{
			TargetRate:   1000,
			AchievedRate: 950,
			AchievedRPS:  950,
			Dropped:      50,
			PeakInFlight: 800,
			Stages: []OpenLoopStage{{
				Index:      0,
				Duration:   time.Minute,
				TargetRate: 1000,
				Requests:   57000,
				Dropped:    50,
				Iterations: 57000,
			}},
		},
	})

	text := Text(rep)
	for _, want := range []string{"MODO OPEN LOOP", "Tasa objetivo", "Tasa alcanzada", "In-flight pico", "Descartes", "Punto de fallo", "etapa 1"} {
		if !strings.Contains(text, want) {
			t.Errorf("text missing %q:\n%s", want, text)
		}
	}

	found := false
	for _, finding := range rep.Diagnosis {
		if strings.Contains(finding.Message, "generador") && strings.Contains(finding.Message, "descart") {
			found = true
		}
	}
	if !found {
		t.Errorf("diagnosis does not report generator saturation: %+v", rep.Diagnosis)
	}
}

func TestOpenLoopReportWithoutFailure(t *testing.T) {
	rep := Build(baseSnapshot(), baseConfig(), sysmetrics.Summary{}, Meta{
		Seed: 1, Planned: time.Minute, Actual: time.Minute,
		OpenLoop: &OpenLoopSummary{
			AchievedRate: 1000,
			Stages: []OpenLoopStage{{
				Index:      0,
				Duration:   time.Minute,
				TargetRate: 1000,
				Requests:   60000,
				Iterations: 60000,
			}},
		},
	})
	text := Text(rep)
	if !strings.Contains(text, "sin fallo") {
		t.Errorf("text missing no-failure line:\n%s", text)
	}
}

func TestOpenLoopFailurePointIgnoresDrops(t *testing.T) {
	stages := []OpenLoopStage{
		{Index: 0, Requests: 100, Dropped: 50},
		{Index: 1, Requests: 100},
	}
	if got := openLoopFailurePoint(stages); got != nil {
		t.Fatalf("failure point = %+v, want nil for generator drops", got)
	}
}

func TestOpenLoopFailurePointUsesErrorRate(t *testing.T) {
	stages := []OpenLoopStage{
		{Index: 0, Requests: 100, Failed: 1},
		{Index: 1, Requests: 100, Failed: 5},
	}
	got := openLoopFailurePoint(stages)
	if got == nil || got.Index != 1 {
		t.Fatalf("failure point = %+v, want stage 1", got)
	}
}

func TestReportShowsAnalysisSkipped(t *testing.T) {
	snap := baseSnapshot()
	snap.AnalysisSkipped = 3
	rep := Build(snap, baseConfig(), sysmetrics.Summary{}, Meta{Seed: 1, Planned: time.Minute, Actual: time.Minute})
	if !strings.Contains(Text(rep), "Análisis omitido: 3") {
		t.Errorf("text missing analysis skipped count:\n%s", Text(rep))
	}
	found := false
	for _, finding := range rep.Diagnosis {
		if strings.Contains(finding.Message, "Análisis de cuerpo omitido") {
			found = true
		}
	}
	if !found {
		t.Errorf("diagnosis missing analysis skipped finding: %+v", rep.Diagnosis)
	}
}
