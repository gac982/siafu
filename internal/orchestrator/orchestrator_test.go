// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package orchestrator

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gac982/siafu/internal/config"
	"github.com/gac982/siafu/internal/engine"
	"github.com/gac982/siafu/internal/logbuf"
	"github.com/gac982/siafu/internal/metrics"
	"github.com/gac982/siafu/internal/report"
	"github.com/gac982/siafu/internal/results"
	"github.com/gac982/siafu/internal/sysmetrics"
)

func orchConfig(base string, thresholds config.Thresholds, stages []config.Stage) *config.Config {
	return &config.Config{
		Version: 1,
		Name:    "orch",
		Target:  config.Target{BaseURL: base, Timeout: config.Duration{Duration: time.Second}},
		Load:    config.LoadConfig{MaxVUs: 100, Stages: stages},
		Scenarios: []config.Scenario{{
			Name: "s",
			Steps: []config.Step{{
				Name:    "one",
				Request: config.Request{Method: "GET", URL: "/"},
				Expect:  config.Expect{Status: config.StatusCodes{200}},
			}},
		}},
		Thresholds: thresholds,
	}
}

func shortStages() []config.Stage {
	return []config.Stage{
		{Duration: config.Duration{Duration: 200 * time.Millisecond}, TargetVUs: 1},
		{Duration: config.Duration{Duration: 300 * time.Millisecond}, TargetVUs: 1},
	}
}

func passingThresholds() config.Thresholds {
	p95 := config.Duration{Duration: time.Second}
	errRate := 0.05
	return config.Thresholds{LatencyP95: &p95, ErrorRate: &errRate}
}

func TestRunPlainSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	rep, code := Run(context.Background(), orchConfig(srv.URL, passingThresholds(), shortStages()), Options{
		NoTUI: true, NoSave: true, Stdout: &stdout, Stderr: &stderr, IsTTY: false,
	})
	if code != 0 || rep.Verdict != report.VerdictPass {
		t.Fatalf("code/verdict = %d/%s total=%d errors=%+v peak=%d (stderr: %s)", code, rep.Verdict, rep.Global.Total, rep.Global.Errors, rep.PeakVUs, stderr.String())
	}
	if !strings.Contains(stdout.String(), "APROBADA") || !strings.Contains(stdout.String(), "RESUMEN GLOBAL") {
		t.Errorf("stdout = %q", stdout.String())
	}
}

func TestRunPlainThresholdFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer srv.Close()

	thresholds := passingThresholds()
	impossible := 100000.0
	thresholds.MinRPS = &impossible

	var stdout, stderr bytes.Buffer
	rep, code := Run(context.Background(), orchConfig(srv.URL, thresholds, shortStages()), Options{
		NoTUI: true, NoSave: true, Stdout: &stdout, Stderr: &stderr, IsTTY: false,
	})
	if code != 1 || rep.Verdict != report.VerdictFail {
		t.Fatalf("code/verdict = %d/%s", code, rep.Verdict)
	}
	if !strings.Contains(stdout.String(), "FALLIDA") {
		t.Errorf("stdout does not contain FALLIDA")
	}
}

func TestRunPlainAborted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer srv.Close()

	stages := []config.Stage{{Duration: config.Duration{Duration: 5 * time.Second}, TargetVUs: 1}}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(400 * time.Millisecond)
		cancel()
	}()

	var stdout, stderr bytes.Buffer
	rep, code := Run(ctx, orchConfig(srv.URL, passingThresholds(), stages), Options{
		NoTUI: true, NoSave: true, Stdout: &stdout, Stderr: &stderr, IsTTY: false,
	})
	if code != 130 || rep.Verdict != report.VerdictAborted || !rep.Aborted {
		t.Fatalf("code/verdict/aborted = %d/%s/%v", code, rep.Verdict, rep.Aborted)
	}
	if !strings.Contains(stdout.String(), "ABORTADA") {
		t.Errorf("stdout does not contain ABORTADA")
	}
}

func TestRunNonTTYFallsBackToPlain(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	rep, code := Run(context.Background(), orchConfig(srv.URL, passingThresholds(), shortStages()), Options{
		NoTUI: false, NoSave: true, Stdout: &stdout, Stderr: &stderr, IsTTY: false,
	})
	if code != 0 || rep.Verdict != report.VerdictPass {
		t.Fatalf("code/verdict = %d/%s", code, rep.Verdict)
	}
	if !strings.Contains(stdout.String(), "APROBADA") {
		t.Errorf("non-TTY did not use plain mode")
	}
}

func TestBuildFrame(t *testing.T) {
	cfg := orchConfig("http://localhost:8080", passingThresholds(), shortStages())
	logs := logbuf.New(10)
	logs.Info("hola")
	status := engine.Status{ActiveVUs: 7, TargetVUs: 10, PeakVUs: 12, Stage: 2, Stages: 3, Elapsed: time.Second, Planned: time.Minute}
	latest := metrics.Snapshot{Total: 10, RPSRecent: 5}
	host := sysmetrics.Snapshot{CPUPercent: 20}

	frame := frameFrom(cfg, latest, host, logs, status)
	if frame.Name != "orch" || frame.Target != cfg.Target.BaseURL {
		t.Errorf("frame identity = %+v", frame)
	}
	if frame.VUsActive != 7 || frame.VUsTarget != 10 || frame.Stage != 2 || frame.Metrics.Total != 10 || frame.Host.CPUPercent != 20 {
		t.Errorf("frame = %+v", frame)
	}
	if len(frame.Logs) != 1 || !strings.Contains(frame.Logs[0], "hola") {
		t.Errorf("logs = %+v", frame.Logs)
	}
}

func TestRunSavesResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer srv.Close()

	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	rep, code := Run(context.Background(), orchConfig(srv.URL, config.Thresholds{}, shortStages()), Options{
		NoTUI: true, Stdout: &stdout, Stderr: &stderr, IsTTY: false,
		ResultsDir: dir, Tag: "smoke", Version: "v-test", Commit: "abc",
	})
	if code != 0 || rep.Global.Total == 0 {
		t.Fatalf("code/total = %d/%d (stderr: %s)", code, rep.Global.Total, stderr.String())
	}
	if !strings.Contains(stderr.String(), "resultados guardados en") {
		t.Errorf("stderr = %q", stderr.String())
	}

	store, err := results.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	list, err := store.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %+v, err = %v", list, err)
	}
	entry := list[0]
	if entry.Tag != "smoke" || entry.Mode != "closed" || entry.Requests == 0 {
		t.Errorf("entry = %+v", entry)
	}
	rec, err := store.Load(entry.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Tag != "smoke" || rec.Version != "v-test" || rec.Commit != "abc" {
		t.Errorf("record = %+v", rec)
	}
	series, err := store.Series(entry.RunID)
	if err != nil || len(series) == 0 {
		t.Fatalf("series = %+v, err = %v", series, err)
	}
}

func TestRunNoSave(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer srv.Close()

	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	_, code := Run(context.Background(), orchConfig(srv.URL, config.Thresholds{}, shortStages()), Options{
		NoTUI: true, NoSave: true, Stdout: &stdout, Stderr: &stderr, IsTTY: false,
		ResultsDir: dir,
	})
	if code != 0 {
		t.Fatalf("code = %d (stderr: %s)", code, stderr.String())
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("results dir is not empty: %+v", entries)
	}
}

func TestResolveResultsDir(t *testing.T) {
	t.Setenv("SIAFU_RESULTS_DIR", "/env/results")
	if got := ResolveResultsDir(Options{}); got != "/env/results" {
		t.Errorf("env dir = %q", got)
	}
	if got := ResolveResultsDir(Options{ResultsDir: "/flag/results"}); got != "/flag/results" {
		t.Errorf("flag dir = %q", got)
	}
	t.Setenv("SIAFU_RESULTS_DIR", "")
	if got := ResolveResultsDir(Options{}); got != ".siafu/runs" {
		t.Errorf("default dir = %q", got)
	}
}

func TestFrameTargetRedactsPassword(t *testing.T) {
	cfg := &config.Config{
		Name:   "redact",
		Target: config.Target{BaseURL: "https://user:s3cret@example.com"},
	}
	frame := frameFrom(cfg, metrics.Snapshot{}, sysmetrics.Snapshot{}, logbuf.New(10), engine.Status{})
	if strings.Contains(frame.Target, "s3cret") {
		t.Errorf("target sin redactar: %q", frame.Target)
	}
	if !strings.Contains(frame.Target, "***") && !strings.Contains(frame.Target, "%2A%2A%2A") {
		t.Errorf("target sin máscara: %q", frame.Target)
	}
}
