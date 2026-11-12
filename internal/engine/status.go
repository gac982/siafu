// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package engine

import (
	"time"

	"github.com/gac982/siafu/internal/metrics"
)

// Status is a snapshot of the engine state for the UI.
type Status struct {
	Mode         string
	ActiveVUs    int
	TargetVUs    int
	PeakVUs      int
	SpawnedVUs   int
	InFlight     int
	MaxInFlight  int
	TargetRate   float64
	AchievedRate float64
	Dropped      int64
	Stage        int
	Stages       int
	Elapsed      time.Duration
	Planned      time.Duration
	Finishing    bool
}

// Result is the outcome of one engine run.
type Result struct {
	Snapshot metrics.Snapshot
	Err      error
	Aborted  bool
	Seed     int64
	OpenLoop *OpenLoopResult
}

// Run exposes the live snapshot stream and the final result.
type Run struct {
	Snapshots <-chan metrics.Snapshot
	Done      <-chan Result
}

// Options configures the engine. Zero values use production defaults.
type Options struct {
	Seed          int64
	SnapshotEvery time.Duration
	TickEvery     time.Duration
	StopGrace     time.Duration
	Logger        Logger
	Now           func() time.Time
	// OnSnapshot receives every snapshot before the UI fan-out. It must not
	// block; persistence appends to memory or a buffered writer.
	OnSnapshot func(metrics.Snapshot)
}
