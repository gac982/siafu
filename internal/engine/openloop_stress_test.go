// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package engine

import (
	"context"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gac982/siafu/internal/config"
	"github.com/gac982/siafu/internal/memguard"
)

// TestOpenLoopStress1M is the opt-in throughput gate: one million requests
// with a bounded worker pool.
//
//	SIAFU_STRESS=1 go test ./internal/engine -run TestOpenLoopStress1M -count=1
func TestOpenLoopStress1M(t *testing.T) {
	if os.Getenv("SIAFU_STRESS") != "1" {
		t.Skip("set SIAFU_STRESS=1 to run the open-loop stress gate")
	}

	srv, _ := fastServer(t)
	cfg := openEngineConfig(srv.URL, 50000, 8000, 20*time.Second)
	cfg.Load.OpenLoop.Stages = []config.RateStage{
		{Duration: config.Duration{Duration: 2 * time.Second}, TargetRate: 50000},
		{Duration: config.Duration{Duration: 20 * time.Second}, TargetRate: 50000},
	}

	e, err := New(cfg, Options{Seed: 1, SnapshotEvery: time.Second, TickEvery: 10 * time.Millisecond, StopGrace: 10 * time.Second, Logger: NopLogger{}})
	if err != nil {
		t.Fatal(err)
	}

	peakRSS := maxRSSDuring(func() {
		result := waitResult(t, e.Start(context.Background()), 50*time.Second)
		if result.Err != nil {
			t.Fatalf("engine error = %v", result.Err)
		}
		if result.OpenLoop == nil {
			t.Fatal("OpenLoop result is nil")
		}
		if result.OpenLoop.PeakInFlight > 8000 {
			t.Fatalf("peak in-flight = %d, want <= 8000", result.OpenLoop.PeakInFlight)
		}
		if result.Snapshot.Total < 1_000_000 {
			t.Fatalf("requests = %d, want >= 1000000", result.Snapshot.Total)
		}
		if result.Snapshot.Failed != 0 {
			t.Fatalf("failed = %d, want 0", result.Snapshot.Failed)
		}
	})

	t.Logf("peak RSS = %d MiB", peakRSS)
	if peakRSS > 450 {
		t.Fatalf("peak RSS = %d MiB, want <= 450", peakRSS)
	}
}

func maxRSSDuring(fn func()) int {
	var peak atomic.Int64
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if rss, err := memguard.SampleRSSMB(); err == nil {
					for {
						current := peak.Load()
						if int64(rss) <= current || peak.CompareAndSwap(current, int64(rss)) {
							break
						}
					}
				}
			case <-done:
				return
			}
		}
	}()
	fn()
	close(done)
	return int(peak.Load())
}

// TestOpenLoopStress8kInFlight fills the worker pool and verifies that the
// in-flight cap is respected. It is opt-in because it opens 8 000 sockets.
func TestOpenLoopStress8kInFlight(t *testing.T) {
	if os.Getenv("SIAFU_STRESS") != "1" {
		t.Skip("set SIAFU_STRESS=1 to run the open-loop in-flight gate")
	}

	srv := slowServer(t, 100*time.Millisecond)
	cfg := openEngineConfig(srv.URL, 80000, 8000, 5*time.Second)
	cfg.Load.OpenLoop.Stages = []config.RateStage{
		{Duration: config.Duration{Duration: time.Second}, TargetRate: 80000},
		{Duration: config.Duration{Duration: 4 * time.Second}, TargetRate: 80000},
	}

	e, err := New(cfg, Options{Seed: 1, SnapshotEvery: time.Second, TickEvery: 10 * time.Millisecond, StopGrace: 10 * time.Second, Logger: NopLogger{}})
	if err != nil {
		t.Fatal(err)
	}

	peakRSS := maxRSSDuring(func() {
		result := waitResult(t, e.Start(context.Background()), 20*time.Second)
		if result.OpenLoop == nil {
			t.Fatal("OpenLoop result is nil")
		}
		if result.OpenLoop.PeakInFlight > 8000 {
			t.Fatalf("peak in-flight = %d, want <= 8000", result.OpenLoop.PeakInFlight)
		}
		if result.OpenLoop.PeakInFlight < 7000 {
			t.Fatalf("peak in-flight = %d, want >= 7000", result.OpenLoop.PeakInFlight)
		}
		if result.Snapshot.Total == 0 {
			t.Fatal("no requests completed")
		}
	})

	t.Logf("peak RSS = %d MiB", peakRSS)
}
