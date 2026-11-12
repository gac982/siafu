// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package engine

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/gac982/siafu/internal/config"
)

// TestEngine15000VUs is the opt-in concurrency gate. It is intentionally
// excluded from make check because it opens 15 000 sockets and goroutines.
//
//	SIAFU_STRESS=1 go test ./internal/engine -run TestEngine15000VUs -count=1
func TestEngine15000VUs(t *testing.T) {
	if os.Getenv("SIAFU_STRESS") != "1" {
		t.Skip("set SIAFU_STRESS=1 to run the 15 000 VU gate")
	}

	srv, hits := fastServer(t)
	cfg := engineConfig(srv.URL, []config.Stage{
		{Duration: config.Duration{Duration: 15 * time.Second}, TargetVUs: 15000},
		{Duration: config.Duration{Duration: 5 * time.Second}, TargetVUs: 15000},
	})
	cfg.Load.MaxVUs = 15000
	cfg.Load.ThinkTime = config.ThinkTime{
		Min: config.Duration{Duration: 100 * time.Millisecond},
		Max: config.Duration{Duration: 100 * time.Millisecond},
	}

	e, err := New(cfg, Options{
		Seed:          1,
		SnapshotEvery: time.Second,
		TickEvery:     100 * time.Millisecond,
		StopGrace:     10 * time.Second,
		Logger:        NopLogger{},
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	run := e.Start(ctx)
	result := waitResult(t, run, 90*time.Second)

	if result.Err != nil {
		t.Fatalf("engine error = %v", result.Err)
	}
	if result.Aborted {
		t.Fatal("engine aborted before finishing")
	}
	status := e.Status()
	if status.PeakVUs < 15000 {
		t.Fatalf("peak VUs = %d, want >= 15000", status.PeakVUs)
	}
	if hits.Load() == 0 || result.Snapshot.Total == 0 {
		t.Fatalf("requests = %d (server hits %d), want > 0", result.Snapshot.Total, hits.Load())
	}
	if result.Snapshot.Failed != 0 {
		t.Fatalf("failed requests = %d, want 0", result.Snapshot.Failed)
	}
}
