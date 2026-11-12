// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gac982/siafu/internal/config"
	"github.com/gac982/siafu/internal/metrics"
)

func engineConfig(base string, stages []config.Stage) *config.Config {
	return &config.Config{
		Version: 1,
		Name:    "engine-test",
		Target: config.Target{
			BaseURL: base,
			Timeout: config.Duration{Duration: time.Second},
		},
		Load: config.LoadConfig{MaxVUs: 1000, Stages: stages},
		Scenarios: []config.Scenario{{
			Name: "s",
			Steps: []config.Step{{
				Name:    "one",
				Request: config.Request{Method: "GET", URL: "/"},
				Expect:  config.Expect{Status: config.StatusCodes{200}},
			}},
		}},
	}
}

func fastServer(t *testing.T) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(200)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func waitResult(t *testing.T, run *Run, timeout time.Duration) Result {
	t.Helper()
	go func() {
		for range run.Snapshots {
		}
	}()
	select {
	case result := <-run.Done:
		return result
	case <-time.After(timeout):
		t.Fatal("engine did not finish in time")
		return Result{}
	}
}

func TestEngineRampCreatesAndRetiresVUs(t *testing.T) {
	srv, _ := fastServer(t)
	cfg := engineConfig(srv.URL, []config.Stage{
		{Duration: config.Duration{Duration: 400 * time.Millisecond}, TargetVUs: 10},
		{Duration: config.Duration{Duration: 400 * time.Millisecond}, TargetVUs: 20},
		{Duration: config.Duration{Duration: 400 * time.Millisecond}, TargetVUs: 0},
	})
	e, err := New(cfg, Options{Seed: 1, SnapshotEvery: 100 * time.Millisecond, TickEvery: 20 * time.Millisecond, StopGrace: time.Second, Logger: NopLogger{}})
	if err != nil {
		t.Fatal(err)
	}
	run := e.Start(context.Background())

	deadline := time.After(2 * time.Second)
	maxActive := 0
	for {
		select {
		case <-deadline:
			t.Fatal("ramp did not finish")
		default:
		}
		status := e.Status()
		if status.ActiveVUs > maxActive {
			maxActive = status.ActiveVUs
		}
		select {
		case result := <-run.Done:
			for range run.Snapshots {
			}
			if maxActive < 10 {
				t.Errorf("max active VUs = %d, want at least 10", maxActive)
			}
			if result.Aborted {
				t.Errorf("natural completion marked aborted")
			}
			if result.Snapshot.Total == 0 {
				t.Errorf("no requests recorded")
			}
			return
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func TestEngineCompletesAtPlannedDuration(t *testing.T) {
	srv, _ := fastServer(t)
	cfg := engineConfig(srv.URL, []config.Stage{
		{Duration: config.Duration{Duration: 700 * time.Millisecond}, TargetVUs: 2},
	})
	e, err := New(cfg, Options{Seed: 1, SnapshotEvery: 100 * time.Millisecond, TickEvery: 20 * time.Millisecond, StopGrace: time.Second, Logger: NopLogger{}})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	result := waitResult(t, e.Start(context.Background()), 3*time.Second)
	if elapsed := time.Since(start); elapsed < 600*time.Millisecond || elapsed > 2*time.Second {
		t.Errorf("elapsed = %v", elapsed)
	}
	if result.Snapshot.Total == 0 || result.Aborted {
		t.Errorf("result = %+v", result)
	}
}

func TestEngineStopGracefulWithInFlightRequest(t *testing.T) {
	started := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case started <- struct{}{}:
		default:
		}
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	cfg := engineConfig(srv.URL, []config.Stage{{Duration: config.Duration{Duration: 200 * time.Millisecond}, TargetVUs: 1}})
	e, err := New(cfg, Options{Seed: 1, SnapshotEvery: 100 * time.Millisecond, TickEvery: 20 * time.Millisecond, StopGrace: time.Second, Logger: NopLogger{}})
	if err != nil {
		t.Fatal(err)
	}
	run := e.Start(context.Background())
	<-started
	time.Sleep(20 * time.Millisecond)
	e.Stop()
	result := waitResult(t, run, 2*time.Second)
	if !result.Aborted {
		t.Errorf("Aborted = false, want true")
	}
}

func TestEngineAbortContext(t *testing.T) {
	srv, _ := fastServer(t)
	cfg := engineConfig(srv.URL, []config.Stage{{Duration: config.Duration{Duration: 5 * time.Second}, TargetVUs: 5}})
	e, err := New(cfg, Options{Seed: 1, SnapshotEvery: 100 * time.Millisecond, TickEvery: 20 * time.Millisecond, StopGrace: time.Second, Logger: NopLogger{}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	run := e.Start(ctx)
	time.Sleep(200 * time.Millisecond)
	cancel()
	result := waitResult(t, run, 2*time.Second)
	if !result.Aborted {
		t.Errorf("Aborted = false, want true")
	}
}

func TestEngineHighConcurrency(t *testing.T) {
	srv, hits := fastServer(t)
	cfg := engineConfig(srv.URL, []config.Stage{
		{Duration: config.Duration{Duration: time.Second}, TargetVUs: 500},
		{Duration: config.Duration{Duration: time.Second}, TargetVUs: 500},
	})
	e, err := New(cfg, Options{Seed: 1, SnapshotEvery: 200 * time.Millisecond, TickEvery: 20 * time.Millisecond, StopGrace: 2 * time.Second, Logger: NopLogger{}})
	if err != nil {
		t.Fatal(err)
	}
	result := waitResult(t, e.Start(context.Background()), 5*time.Second)
	if result.Snapshot.Total == 0 {
		t.Fatal("no requests recorded")
	}
	if result.Snapshot.Total > hits.Load() {
		t.Errorf("recorded %d > server hits %d", result.Snapshot.Total, hits.Load())
	}
	if float64(result.Snapshot.Total) < 0.9*float64(hits.Load()) {
		t.Errorf("recorded %d, server hits %d", result.Snapshot.Total, hits.Load())
	}
}

func TestEngineNoRequests(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	base := srv.URL
	srv.Close()

	cfg := engineConfig(base, []config.Stage{{Duration: config.Duration{Duration: 300 * time.Millisecond}, TargetVUs: 1}})
	cfg.Target.Timeout = config.Duration{Duration: 100 * time.Millisecond}
	e, err := New(cfg, Options{Seed: 1, SnapshotEvery: 100 * time.Millisecond, TickEvery: 20 * time.Millisecond, StopGrace: time.Second, Logger: NopLogger{}})
	if err != nil {
		t.Fatal(err)
	}
	result := waitResult(t, e.Start(context.Background()), 3*time.Second)
	if result.Snapshot.Total == 0 || result.Snapshot.Failed != result.Snapshot.Total {
		t.Errorf("total/failed = %d/%d, want equal and > 0", result.Snapshot.Total, result.Snapshot.Failed)
	}
	if len(result.Snapshot.Errors) == 0 {
		t.Errorf("expected transport errors")
	}
}

func TestEngineRampDownRetiresOnlySurplus(t *testing.T) {
	srv, _ := fastServer(t)
	cfg := engineConfig(srv.URL, []config.Stage{
		{Duration: config.Duration{Duration: 200 * time.Millisecond}, TargetVUs: 20},
		{Duration: config.Duration{Duration: 600 * time.Millisecond}, TargetVUs: 10},
	})
	e, err := New(cfg, Options{Seed: 1, SnapshotEvery: 200 * time.Millisecond, TickEvery: 10 * time.Millisecond, StopGrace: time.Second, Logger: NopLogger{}})
	if err != nil {
		t.Fatal(err)
	}
	waitResult(t, e.Start(context.Background()), 3*time.Second)
	status := e.Status()
	if status.SpawnedVUs > 25 {
		t.Fatalf("spawned %d VUs during a 20→10 ramp; the whole pool was replaced", status.SpawnedVUs)
	}
}

func TestEnginePeakVUs(t *testing.T) {
	srv, _ := fastServer(t)
	cfg := engineConfig(srv.URL, []config.Stage{{Duration: config.Duration{Duration: 300 * time.Millisecond}, TargetVUs: 10}})
	e, err := New(cfg, Options{Seed: 1, SnapshotEvery: 100 * time.Millisecond, TickEvery: 20 * time.Millisecond, StopGrace: time.Second, Logger: NopLogger{}})
	if err != nil {
		t.Fatal(err)
	}
	run := e.Start(context.Background())
	time.Sleep(200 * time.Millisecond)
	status := e.Status()
	waitResult(t, run, 2*time.Second)
	if status.PeakVUs < 5 {
		t.Errorf("PeakVUs = %d, want at least 5 mid-ramp", status.PeakVUs)
	}
}

func TestSnapshotSink(t *testing.T) {
	srv, _ := fastServer(t)
	cfg := engineConfig(srv.URL, []config.Stage{
		{Duration: config.Duration{Duration: 400 * time.Millisecond}, TargetVUs: 5},
		{Duration: config.Duration{Duration: 400 * time.Millisecond}, TargetVUs: 0},
	})

	var mu sync.Mutex
	var snapshots []metrics.Snapshot
	e, err := New(cfg, Options{
		Seed:          1,
		SnapshotEvery: 50 * time.Millisecond,
		TickEvery:     20 * time.Millisecond,
		StopGrace:     time.Second,
		Logger:        NopLogger{},
		OnSnapshot: func(snapshot metrics.Snapshot) {
			mu.Lock()
			snapshots = append(snapshots, snapshot)
			mu.Unlock()
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	waitResult(t, e.Start(context.Background()), 5*time.Second)

	mu.Lock()
	defer mu.Unlock()
	if len(snapshots) < 2 {
		t.Fatalf("snapshots = %d, want >= 2", len(snapshots))
	}
	if last := snapshots[len(snapshots)-1]; last.Total == 0 {
		t.Fatalf("last snapshot has no requests: %+v", last)
	}
}
