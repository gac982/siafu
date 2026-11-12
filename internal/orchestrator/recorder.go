// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package orchestrator

import (
	"sync"
	"time"

	"github.com/gac982/siafu/internal/engine"
	"github.com/gac982/siafu/internal/metrics"
	"github.com/gac982/siafu/internal/results"
	"github.com/gac982/siafu/internal/sysmetrics"
)

const minSeriesDelta = 100 * time.Millisecond

// Recorder accumulates per-second series points for persistence.
type Recorder struct {
	mu       sync.Mutex
	points   []results.SeriesPoint
	last     metrics.Snapshot
	haveLast bool
	host     sysmetrics.Snapshot
}

// NewRecorder creates an empty recorder.
func NewRecorder() *Recorder { return &Recorder{} }

// AddHost remembers the latest host sample for subsequent metric points.
func (r *Recorder) AddHost(snapshot sysmetrics.Snapshot) {
	r.mu.Lock()
	r.host = snapshot
	r.mu.Unlock()
}

// AddMetrics appends one series point and computes deltas against the previous
// snapshot.
func (r *Recorder) AddMetrics(snapshot metrics.Snapshot, status engine.Status) {
	r.mu.Lock()
	defer r.mu.Unlock()

	point := results.SeriesPoint{
		TMS:          snapshot.Elapsed.Milliseconds(),
		P95MS:        durationMillis(snapshot.Latency.P95),
		P99MS:        durationMillis(snapshot.Latency.P99),
		InFlight:     status.InFlight,
		TargetRate:   status.TargetRate,
		AchievedRate: status.AchievedRate,
		Dropped:      status.Dropped,
		CPUPercent:   r.host.CPUPercent,
		MemPercent:   r.host.MemPercent,
	}

	if r.haveLast && snapshot.Elapsed-r.last.Elapsed < minSeriesDelta {
		if len(r.points) > 0 {
			last := &r.points[len(r.points)-1]
			last.P95MS = point.P95MS
			last.P99MS = point.P99MS
			last.InFlight = point.InFlight
			last.TargetRate = point.TargetRate
			last.AchievedRate = point.AchievedRate
			last.Dropped = point.Dropped
			last.CPUPercent = point.CPUPercent
			last.MemPercent = point.MemPercent
		}
		return
	}
	if r.haveLast {
		elapsed := (snapshot.Elapsed - r.last.Elapsed).Seconds()
		if elapsed > 0 {
			deltaTotal := snapshot.Total - r.last.Total
			point.RPS = float64(deltaTotal) / elapsed
			if deltaTotal > 0 {
				point.ErrorRate = float64(snapshot.Failed-r.last.Failed) / float64(deltaTotal)
			}
		}
	} else {
		if elapsed := snapshot.Elapsed.Seconds(); elapsed > 0 {
			point.RPS = float64(snapshot.Total) / elapsed
		} else {
			point.RPS = snapshot.RPSRecent
		}
		point.ErrorRate = snapshot.ErrorRate
	}

	r.points = append(r.points, point)
	r.last = snapshot
	r.haveLast = true
}

// Points returns a copy of the accumulated series.
func (r *Recorder) Points() []results.SeriesPoint {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]results.SeriesPoint, len(r.points))
	copy(out, r.points)
	return out
}

func durationMillis(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}
