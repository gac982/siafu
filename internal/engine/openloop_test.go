// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gac982/siafu/internal/config"
)

func openEngineConfig(base string, rate float64, maxInFlight int, duration time.Duration) *config.Config {
	cfg := engineConfig(base, nil)
	cfg.Load.Mode = config.ModeOpen
	cfg.Load.OpenLoop = config.OpenLoopConfig{
		Stages: []config.RateStage{{
			Duration:   config.Duration{Duration: duration},
			TargetRate: rate,
		}},
		MaxInFlight:         maxInFlight,
		OnOverload:          config.OverloadDrop,
		OnAnalysisExhausted: config.AnalysisSkip,
	}
	return cfg
}

func slowServer(t *testing.T, latency time.Duration) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(latency)
		w.WriteHeader(200)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestOpenLoopRespectsMaxInFlight(t *testing.T) {
	srv := slowServer(t, 50*time.Millisecond)
	cfg := openEngineConfig(srv.URL, 1000, 100, time.Second)

	e, err := New(cfg, Options{Seed: 1, SnapshotEvery: 100 * time.Millisecond, TickEvery: 10 * time.Millisecond, StopGrace: 5 * time.Second, Logger: NopLogger{}})
	if err != nil {
		t.Fatal(err)
	}
	result := waitResult(t, e.Start(context.Background()), 10*time.Second)

	if result.OpenLoop == nil {
		t.Fatal("OpenLoop result is nil")
	}
	if result.OpenLoop.PeakInFlight > 100 {
		t.Fatalf("peak in-flight = %d, want <= 100", result.OpenLoop.PeakInFlight)
	}
	if result.OpenLoop.PeakInFlight == 0 {
		t.Fatal("peak in-flight = 0, want > 0")
	}
	if result.OpenLoop.Dropped != 0 {
		t.Fatalf("dropped = %d, want 0 under capacity", result.OpenLoop.Dropped)
	}
	if result.Snapshot.Total == 0 {
		t.Fatal("no requests completed")
	}
	if status := e.Status(); status.InFlight != 0 {
		t.Fatalf("in-flight after stop = %d, want 0", status.InFlight)
	}
}

func TestOpenLoopDropsWhenSaturated(t *testing.T) {
	srv := slowServer(t, 50*time.Millisecond)
	cfg := openEngineConfig(srv.URL, 5000, 10, time.Second)

	e, err := New(cfg, Options{Seed: 1, SnapshotEvery: 100 * time.Millisecond, TickEvery: 10 * time.Millisecond, StopGrace: 5 * time.Second, Logger: NopLogger{}})
	if err != nil {
		t.Fatal(err)
	}
	result := waitResult(t, e.Start(context.Background()), 10*time.Second)

	if result.OpenLoop == nil {
		t.Fatal("OpenLoop result is nil")
	}
	if result.OpenLoop.PeakInFlight > 10 {
		t.Fatalf("peak in-flight = %d, want <= 10", result.OpenLoop.PeakInFlight)
	}
	if result.OpenLoop.Dropped == 0 {
		t.Fatal("dropped = 0, want > 0 when saturated")
	}
	if result.Snapshot.Total == 0 {
		t.Fatal("no requests completed")
	}
}

func TestOpenLoopStopGrace(t *testing.T) {
	srv := slowServer(t, 500*time.Millisecond)
	cfg := openEngineConfig(srv.URL, 500, 50, 5*time.Second)

	e, err := New(cfg, Options{Seed: 1, SnapshotEvery: 50 * time.Millisecond, TickEvery: 10 * time.Millisecond, StopGrace: 50 * time.Millisecond, Logger: NopLogger{}})
	if err != nil {
		t.Fatal(err)
	}
	run := e.Start(context.Background())

	deadline := time.Now().Add(2 * time.Second)
	for e.Status().InFlight == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if e.Status().InFlight == 0 {
		t.Fatal("no in-flight iterations to stop")
	}

	started := time.Now()
	e.Stop()
	result := waitResult(t, run, 3*time.Second)
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("stop took %s, want <= 1s", elapsed)
	}
	if !result.Aborted {
		t.Fatal("run was not marked aborted")
	}
	if status := e.Status(); status.InFlight != 0 {
		t.Fatalf("in-flight after stop = %d, want 0", status.InFlight)
	}
}

func TestOpenLoopRateAccuracy(t *testing.T) {
	srv, _ := fastServer(t)
	cfg := openEngineConfig(srv.URL, 1000, 100, time.Second)
	cfg.Load.OpenLoop.Stages = []config.RateStage{
		{Duration: config.Duration{Duration: time.Second}, TargetRate: 1000},
		{Duration: config.Duration{Duration: time.Second}, TargetRate: 1000},
	}

	e, err := New(cfg, Options{Seed: 1, SnapshotEvery: 100 * time.Millisecond, TickEvery: 10 * time.Millisecond, StopGrace: 5 * time.Second, Logger: NopLogger{}})
	if err != nil {
		t.Fatal(err)
	}
	result := waitResult(t, e.Start(context.Background()), 10*time.Second)
	if result.OpenLoop == nil {
		t.Fatal("OpenLoop result is nil")
	}

	var plateau *OpenLoopStage
	for i := range result.OpenLoop.Stages {
		if result.OpenLoop.Stages[i].Index == 1 {
			plateau = &result.OpenLoop.Stages[i]
		}
	}
	if plateau == nil {
		t.Fatalf("stages = %+v, want stage 1", result.OpenLoop.Stages)
	}
	if plateau.Iterations < 950 || plateau.Iterations > 1050 {
		t.Fatalf("plateau iterations = %d, want 950..1050", plateau.Iterations)
	}
}
