// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

// Package report evaluates thresholds and renders the final test report.
package report

import (
	"time"

	"github.com/gac982/siafu/internal/config"
	"github.com/gac982/siafu/internal/metrics"
	"github.com/gac982/siafu/internal/redact"
	"github.com/gac982/siafu/internal/sysmetrics"
)

// Verdict is the overall outcome of a run.
type Verdict string

const (
	// VerdictPass means every threshold passed.
	VerdictPass Verdict = "PASS"
	// VerdictFail means at least one threshold failed.
	VerdictFail Verdict = "FAIL"
	// VerdictNotEvaluated means the file defines no thresholds.
	VerdictNotEvaluated Verdict = "NOT_EVALUATED"
	// VerdictAborted means the user stopped the run.
	VerdictAborted Verdict = "ABORTED"
	// VerdictError means no requests completed or the engine failed.
	VerdictError Verdict = "ERROR"
)

// Label returns the Spanish label shown to the user.
func (v Verdict) Label() string {
	switch v {
	case VerdictPass:
		return "APROBADA"
	case VerdictFail:
		return "FALLIDA"
	case VerdictNotEvaluated:
		return "SIN UMBRALES"
	case VerdictAborted:
		return "ABORTADA"
	default:
		return "ERROR"
	}
}

// Severity ranks a diagnostic finding.
type Severity string

const (
	// SeverityInfo is informational.
	SeverityInfo Severity = "info"
	// SeverityWarning points at a likely cause.
	SeverityWarning Severity = "warning"
	// SeverityCritical blocks acceptance.
	SeverityCritical Severity = "critical"
)

// Finding is one diagnostic conclusion.
type Finding struct {
	Severity Severity
	Message  string
	Partial  bool
}

// ThresholdResult compares one observed metric with its threshold.
type ThresholdResult struct {
	Name      string
	Expected  string
	Observed  string
	Pass      bool
	Evaluated bool
	Limit     float64
	Unit      string
}

// OpenLoopStage is the report view of one open-loop stage.
type OpenLoopStage struct {
	Index      int
	Duration   time.Duration
	TargetRate float64
	Requests   int64
	Failed     int64
	Dropped    int64
	Iterations int64
}

// OpenLoopSummary is the open-loop data carried into the report.
type OpenLoopSummary struct {
	TargetRate   float64
	AchievedRate float64
	AchievedRPS  float64
	Dropped      int64
	PeakInFlight int
	Stages       []OpenLoopStage
}

// Meta carries engine data the metrics snapshot does not own.
type Meta struct {
	Seed           int64
	PeakVUs        int
	Planned        time.Duration
	Actual         time.Duration
	Aborted        bool
	MemoryExceeded bool
	OpenLoop       *OpenLoopSummary
}

// Report is the complete, serializable outcome of a run.
type Report struct {
	Name           string
	Target         string
	StartedAt      time.Time
	Planned        time.Duration
	Actual         time.Duration
	PeakVUs        int
	Seed           int64
	Verdict        Verdict
	Aborted        bool
	MemoryExceeded bool
	OpenLoop       *OpenLoopSummary
	Global         metrics.Snapshot
	Scenarios      []metrics.ScenarioSnapshot
	Host           sysmetrics.Summary
	Thresholds     []ThresholdResult
	Diagnosis      []Finding
}

// Build assembles the report from the final snapshot and run metadata.
func Build(snapshot metrics.Snapshot, cfg *config.Config, host sysmetrics.Summary, meta Meta) Report {
	planned := meta.Planned
	if planned == 0 {
		if cfg.Load.Mode == config.ModeOpen {
			for _, stage := range cfg.Load.OpenLoop.Stages {
				planned += stage.Duration.Value()
			}
		} else {
			for _, stage := range cfg.Load.Stages {
				planned += stage.Duration.Value()
			}
		}
	}
	actual := meta.Actual
	if actual == 0 {
		actual = snapshot.Elapsed
	}
	if planned > 0 && snapshot.Total > 0 {
		snapshot.RPSAvg = float64(snapshot.Total) / planned.Seconds()
		for i := range snapshot.Scenarios {
			if snapshot.Scenarios[i].Total > 0 {
				snapshot.Scenarios[i].RPSAvg = float64(snapshot.Scenarios[i].Total) / planned.Seconds()
			}
		}
	}

	report := Report{
		Name:           cfg.Name,
		Target:         redact.New(nil).URL(cfg.Target.BaseURL),
		StartedAt:      snapshot.StartedAt,
		Planned:        planned,
		Actual:         actual,
		PeakVUs:        meta.PeakVUs,
		Seed:           meta.Seed,
		Aborted:        meta.Aborted,
		MemoryExceeded: meta.MemoryExceeded,
		OpenLoop:       meta.OpenLoop,
		Global:         snapshot,
		Scenarios:      snapshot.Scenarios,
		Host:           host,
	}

	switch {
	case meta.MemoryExceeded:
		report.Verdict = VerdictError
	case meta.Aborted:
		report.Verdict = VerdictAborted
	case snapshot.Total == 0:
		report.Verdict = VerdictError
	default:
		report.Thresholds = Evaluate(cfg.Thresholds, snapshot)
		switch {
		case len(report.Thresholds) == 0:
			report.Verdict = VerdictNotEvaluated
		default:
			report.Verdict = VerdictPass
			for _, threshold := range report.Thresholds {
				if !threshold.Pass {
					report.Verdict = VerdictFail
					break
				}
			}
		}
	}

	report.Diagnosis = Diagnose(report)
	return report
}

// ExitCode maps the verdict to the process exit code.
func (r Report) ExitCode() int {
	switch r.Verdict {
	case VerdictPass, VerdictNotEvaluated:
		return 0
	case VerdictFail:
		return 1
	case VerdictAborted:
		return 130
	default:
		return 3
	}
}
