// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package metrics

import "time"

// ErrorCount is one transport/plantilla/extraction error bucket.
type ErrorCount struct {
	Kind  string
	Count int64
}

// CheckFailure is one failed-check bucket.
type CheckFailure struct {
	Scenario string
	Step     string
	Reason   string
	Count    int64
}

// ScenarioSnapshot is the report view of one scenario.
type ScenarioSnapshot struct {
	Name             string
	Total            int64
	Failed           int64
	Canceled         int64
	Iterations       int64
	FailedIterations int64
	ErrorRate        float64
	RPSAvg           float64
	RPSPeak          float64
	Latency          Percentiles
	ChecksTotal      int64
	ChecksPassed     int64
	TransferredBytes int64
	AnalysisSkipped  int64
	StatusCodes      map[int]int64
	Protocols        map[string]int64
}

// Snapshot is an immutable view of the metrics at one instant.
type Snapshot struct {
	StartedAt         time.Time
	Elapsed           time.Duration
	Total             int64
	Failed            int64
	Canceled          int64
	ErrorRate         float64
	RPSAvg            float64
	RPSRecent         float64
	RPSPeak           float64
	RPSPeakAt         time.Duration
	Latency           Percentiles
	StatusCodes       map[int]int64
	FailedStatusCodes map[int]int64
	Errors            []ErrorCount
	CheckFailures     []CheckFailure
	ChecksTotal       int64
	ChecksPassed      int64
	Iterations        int64
	FailedIterations  int64
	TransferredBytes  int64
	AnalysisSkipped   int64
	Protocols         map[string]int64
	Scenarios         []ScenarioSnapshot
}
