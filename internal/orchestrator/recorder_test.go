// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package orchestrator

import (
	"math"
	"testing"
	"time"

	"github.com/gac982/siafu/internal/engine"
	"github.com/gac982/siafu/internal/metrics"
	"github.com/gac982/siafu/internal/sysmetrics"
)

func TestRecorderComputesDeltas(t *testing.T) {
	recorder := NewRecorder()
	recorder.AddHost(sysmetrics.Snapshot{CPUPercent: 50, MemPercent: 20})
	recorder.AddMetrics(metrics.Snapshot{
		Elapsed:   time.Second,
		Total:     100,
		Failed:    1,
		ErrorRate: 0.01,
		RPSRecent: 20,
		Latency:   metrics.Percentiles{P95: 40 * time.Millisecond, P99: 80 * time.Millisecond},
	}, engine.Status{InFlight: 10, TargetRate: 100, AchievedRate: 90, Dropped: 2})
	recorder.AddMetrics(metrics.Snapshot{
		Elapsed: 2 * time.Second,
		Total:   250,
		Failed:  3,
		Latency: metrics.Percentiles{P95: 45 * time.Millisecond, P99: 90 * time.Millisecond},
	}, engine.Status{InFlight: 12, TargetRate: 150, AchievedRate: 140, Dropped: 3})

	points := recorder.Points()
	if len(points) != 2 {
		t.Fatalf("points = %d, want 2", len(points))
	}
	if points[0].RPS != 100 || points[0].ErrorRate != 0.01 {
		t.Errorf("first point = %+v", points[0])
	}
	second := points[1]
	if second.TMS != 2000 || second.P95MS != 45 || second.P99MS != 90 {
		t.Errorf("second point = %+v", second)
	}
	if second.RPS != 150 {
		t.Errorf("rps = %v, want 150", second.RPS)
	}
	if math.Abs(second.ErrorRate-(2.0/150.0)) > 1e-9 {
		t.Errorf("error rate = %v, want %v", second.ErrorRate, 2.0/150.0)
	}
	if second.InFlight != 12 || second.TargetRate != 150 || second.AchievedRate != 140 || second.Dropped != 3 {
		t.Errorf("open-loop fields = %+v", second)
	}
	if second.CPUPercent != 50 || second.MemPercent != 20 {
		t.Errorf("host fields = %+v", second)
	}

	points[0].RPS = 999
	if recorder.Points()[0].RPS != 100 {
		t.Error("Points() did not return a copy")
	}
}

func TestRecorderDeduplicatesCloseSnapshots(t *testing.T) {
	recorder := NewRecorder()
	recorder.AddMetrics(metrics.Snapshot{
		Elapsed:   time.Second,
		Total:     100,
		RPSRecent: 20,
		Latency:   metrics.Percentiles{P95: 40 * time.Millisecond},
	}, engine.Status{})
	recorder.AddMetrics(metrics.Snapshot{
		Elapsed: time.Second + 300*time.Microsecond,
		Total:   100,
		Latency: metrics.Percentiles{P95: 50 * time.Millisecond},
	}, engine.Status{})

	points := recorder.Points()
	if len(points) != 1 {
		t.Fatalf("points = %d, want 1 (close snapshots must be merged)", len(points))
	}
	if points[0].RPS != 100 {
		t.Errorf("rps = %v, want 100", points[0].RPS)
	}
	if points[0].P95MS != 50 {
		t.Errorf("p95 = %v, want the latest value", points[0].P95MS)
	}
}
