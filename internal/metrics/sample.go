// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

// Package metrics aggregates request samples into immutable snapshots.
package metrics

import "time"

// Sample is one completed request.
type Sample struct {
	Scenario        string
	Step            string
	At              time.Time
	Duration        time.Duration
	Status          int
	Protocol        string
	Bytes           int64
	Failed          bool
	ErrorKind       string
	ChecksTotal     int
	ChecksPassed    int
	CheckReason     string
	AnalysisSkipped bool
}

// Iteration is one completed scenario iteration.
type Iteration struct {
	Scenario string
	At       time.Time
	Duration time.Duration
	Failed   bool
}

// Batch is the unit sent from a virtual user to the aggregator.
type Batch struct {
	Requests   []Sample
	Iterations []Iteration
}
