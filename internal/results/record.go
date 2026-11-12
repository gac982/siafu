// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

// Package results defines the persisted, versioned representation of a run.
package results

import (
	"time"

	"github.com/gac982/siafu/internal/metrics"
	"github.com/gac982/siafu/internal/report"
	"github.com/gac982/siafu/internal/sysmetrics"
)

// SchemaVersion is the current run.json schema.
const SchemaVersion = 1

// RunRecord is one persisted run.
type RunRecord struct {
	SchemaVersion  int         `json:"schema_version"`
	RunID          string      `json:"run_id"`
	CreatedAt      time.Time   `json:"created_at"`
	Name           string      `json:"name"`
	Tag            string      `json:"tag,omitempty"`
	Mode           string      `json:"mode"`
	Target         string      `json:"target"`
	Source         string      `json:"source"`
	ConfigHash     string      `json:"config_hash"`
	Version        string      `json:"version"`
	Commit         string      `json:"commit"`
	Seed           int64       `json:"seed"`
	PlannedMS      int64       `json:"planned_ms"`
	ActualMS       int64       `json:"actual_ms"`
	Verdict        string      `json:"verdict"`
	ExitCode       int         `json:"exit_code"`
	Aborted        bool        `json:"aborted,omitempty"`
	MemoryExceeded bool        `json:"memory_exceeded,omitempty"`
	OpenLoop       *OpenLoop   `json:"open_loop,omitempty"`
	Global         Global      `json:"global"`
	Scenarios      []Scenario  `json:"scenarios"`
	Thresholds     []Threshold `json:"thresholds"`
	Diagnosis      []Diagnosis `json:"diagnosis"`
	Host           Host        `json:"host"`
}

// Meta carries data the report does not own.
type Meta struct {
	RunID      string
	CreatedAt  time.Time
	Tag        string
	Source     string
	ConfigHash string
	Version    string
	Commit     string
	ExitCode   int
}

// Global is the global metrics block.
type Global struct {
	Requests         int64            `json:"requests"`
	Failed           int64            `json:"failed"`
	Canceled         int64            `json:"canceled"`
	Iterations       int64            `json:"iterations"`
	FailedIterations int64            `json:"failed_iterations"`
	ErrorRate        float64          `json:"error_rate"`
	RPSAvg           float64          `json:"rps_avg"`
	RPSPeak          float64          `json:"rps_peak"`
	P50MS            float64          `json:"p50_ms"`
	P90MS            float64          `json:"p90_ms"`
	P95MS            float64          `json:"p95_ms"`
	P99MS            float64          `json:"p99_ms"`
	MinMS            float64          `json:"min_ms"`
	MaxMS            float64          `json:"max_ms"`
	MeanMS           float64          `json:"mean_ms"`
	ChecksTotal      int64            `json:"checks_total"`
	ChecksPassed     int64            `json:"checks_passed"`
	TransferredBytes int64            `json:"transferred_bytes"`
	AnalysisSkipped  int64            `json:"analysis_skipped"`
	StatusCodes      map[int]int64    `json:"status_codes,omitempty"`
	Protocols        map[string]int64 `json:"protocols,omitempty"`
	Errors           []ErrorCount     `json:"errors,omitempty"`
	CheckFailures    []CheckFailure   `json:"check_failures,omitempty"`
}

// Scenario is one scenario's metrics block.
type Scenario struct {
	Name             string           `json:"name"`
	Requests         int64            `json:"requests"`
	Failed           int64            `json:"failed"`
	Canceled         int64            `json:"canceled"`
	Iterations       int64            `json:"iterations"`
	FailedIterations int64            `json:"failed_iterations"`
	ErrorRate        float64          `json:"error_rate"`
	RPSAvg           float64          `json:"rps_avg"`
	RPSPeak          float64          `json:"rps_peak"`
	P50MS            float64          `json:"p50_ms"`
	P90MS            float64          `json:"p90_ms"`
	P95MS            float64          `json:"p95_ms"`
	P99MS            float64          `json:"p99_ms"`
	ChecksTotal      int64            `json:"checks_total"`
	ChecksPassed     int64            `json:"checks_passed"`
	TransferredBytes int64            `json:"transferred_bytes"`
	AnalysisSkipped  int64            `json:"analysis_skipped"`
	StatusCodes      map[int]int64    `json:"status_codes,omitempty"`
	Protocols        map[string]int64 `json:"protocols,omitempty"`
}

// ErrorCount is one transport/plantilla/extraction error bucket.
type ErrorCount struct {
	Kind  string `json:"kind"`
	Count int64  `json:"count"`
}

// CheckFailure is one failed-check bucket.
type CheckFailure struct {
	Scenario string `json:"scenario"`
	Step     string `json:"step"`
	Reason   string `json:"reason"`
	Count    int64  `json:"count"`
}

// Threshold is one evaluated acceptance criterion.
type Threshold struct {
	Name      string  `json:"name"`
	Expected  string  `json:"expected"`
	Observed  string  `json:"observed"`
	Pass      bool    `json:"pass"`
	Evaluated bool    `json:"evaluated"`
	Limit     float64 `json:"limit,omitempty"`
	Unit      string  `json:"unit,omitempty"`
}

// Diagnosis is one deterministic finding.
type Diagnosis struct {
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Partial  bool   `json:"partial,omitempty"`
}

// Host is the generator summary.
type Host struct {
	CPUAvg        float64 `json:"cpu_avg"`
	CPUMax        float64 `json:"cpu_max"`
	MemMaxPercent float64 `json:"mem_max_percent"`
	NetRxPeak     float64 `json:"net_rx_peak"`
	NetTxPeak     float64 `json:"net_tx_peak"`
	Samples       int     `json:"samples"`
}

// OpenLoop is the open-loop summary.
type OpenLoop struct {
	TargetRate   float64         `json:"target_rate"`
	AchievedRate float64         `json:"achieved_rate"`
	AchievedRPS  float64         `json:"achieved_rps"`
	Dropped      int64           `json:"dropped"`
	PeakInFlight int             `json:"peak_in_flight"`
	Stages       []OpenLoopStage `json:"stages,omitempty"`
}

// OpenLoopStage is one open-loop stage result.
type OpenLoopStage struct {
	Index      int     `json:"index"`
	DurationMS float64 `json:"duration_ms"`
	TargetRate float64 `json:"target_rate"`
	Requests   int64   `json:"requests"`
	Failed     int64   `json:"failed"`
	Dropped    int64   `json:"dropped"`
	Iterations int64   `json:"iterations"`
}

// SeriesPoint is one per-second sample.
type SeriesPoint struct {
	TMS          int64   `json:"t_ms"`
	RPS          float64 `json:"rps"`
	P95MS        float64 `json:"p95_ms"`
	P99MS        float64 `json:"p99_ms"`
	ErrorRate    float64 `json:"error_rate"`
	InFlight     int     `json:"in_flight,omitempty"`
	TargetRate   float64 `json:"target_rate,omitempty"`
	AchievedRate float64 `json:"achieved_rate,omitempty"`
	Dropped      int64   `json:"dropped,omitempty"`
	CPUPercent   float64 `json:"cpu_percent,omitempty"`
	MemPercent   float64 `json:"mem_percent,omitempty"`
}

// IndexEntry is the compact entry appended to index.jsonl.
type IndexEntry struct {
	SchemaVersion int       `json:"schema_version"`
	RunID         string    `json:"run_id"`
	CreatedAt     time.Time `json:"created_at"`
	Name          string    `json:"name"`
	Tag           string    `json:"tag,omitempty"`
	Mode          string    `json:"mode"`
	Verdict       string    `json:"verdict"`
	ExitCode      int       `json:"exit_code"`
	DurationMS    int64     `json:"duration_ms"`
	Requests      int64     `json:"requests"`
	Failed        int64     `json:"failed"`
	ErrorRate     float64   `json:"error_rate"`
	P95MS         float64   `json:"p95_ms"`
	RPSAvg        float64   `json:"rps_avg"`
	PeakInFlight  int       `json:"peak_in_flight,omitempty"`
	Dropped       int64     `json:"dropped,omitempty"`
}

// FromReport converts a report into a persisted record.
func FromReport(rep report.Report, meta Meta) RunRecord {
	createdAt := meta.CreatedAt
	if createdAt.IsZero() {
		createdAt = rep.StartedAt
	}
	rec := RunRecord{
		SchemaVersion:  SchemaVersion,
		RunID:          meta.RunID,
		CreatedAt:      createdAt,
		Name:           rep.Name,
		Tag:            meta.Tag,
		Mode:           "closed",
		Target:         rep.Target,
		Source:         meta.Source,
		ConfigHash:     meta.ConfigHash,
		Version:        meta.Version,
		Commit:         meta.Commit,
		Seed:           rep.Seed,
		PlannedMS:      rep.Planned.Milliseconds(),
		ActualMS:       rep.Actual.Milliseconds(),
		Verdict:        string(rep.Verdict),
		ExitCode:       meta.ExitCode,
		Aborted:        rep.Aborted,
		MemoryExceeded: rep.MemoryExceeded,
		Global:         globalFromSnapshot(rep.Global),
		Scenarios:      scenariosFromSnapshot(rep.Scenarios),
		Thresholds:     thresholdsFromReport(rep.Thresholds),
		Diagnosis:      diagnosisFromReport(rep.Diagnosis),
		Host:           hostFromSummary(rep.Host),
	}
	if rep.OpenLoop != nil {
		rec.Mode = "open"
		rec.OpenLoop = openLoopFromReport(rep.OpenLoop)
	}
	return rec
}

// IndexEntry returns the compact index view of the record.
func (r RunRecord) IndexEntry() IndexEntry {
	duration := r.ActualMS
	if duration <= 0 {
		duration = r.PlannedMS
	}
	entry := IndexEntry{
		SchemaVersion: r.SchemaVersion,
		RunID:         r.RunID,
		CreatedAt:     r.CreatedAt,
		Name:          r.Name,
		Tag:           r.Tag,
		Mode:          r.Mode,
		Verdict:       r.Verdict,
		ExitCode:      r.ExitCode,
		DurationMS:    duration,
		Requests:      r.Global.Requests,
		Failed:        r.Global.Failed,
		ErrorRate:     r.Global.ErrorRate,
		P95MS:         r.Global.P95MS,
		RPSAvg:        r.Global.RPSAvg,
	}
	if r.OpenLoop != nil {
		entry.PeakInFlight = r.OpenLoop.PeakInFlight
		entry.Dropped = r.OpenLoop.Dropped
	}
	return entry
}

func globalFromSnapshot(s metrics.Snapshot) Global {
	return Global{
		Requests:         s.Total,
		Failed:           s.Failed,
		Canceled:         s.Canceled,
		Iterations:       s.Iterations,
		FailedIterations: s.FailedIterations,
		ErrorRate:        s.ErrorRate,
		RPSAvg:           s.RPSAvg,
		RPSPeak:          s.RPSPeak,
		P50MS:            durationMS(s.Latency.P50),
		P90MS:            durationMS(s.Latency.P90),
		P95MS:            durationMS(s.Latency.P95),
		P99MS:            durationMS(s.Latency.P99),
		MinMS:            durationMS(s.Latency.Min),
		MaxMS:            durationMS(s.Latency.Max),
		MeanMS:           durationMS(s.Latency.Mean),
		ChecksTotal:      s.ChecksTotal,
		ChecksPassed:     s.ChecksPassed,
		TransferredBytes: s.TransferredBytes,
		AnalysisSkipped:  s.AnalysisSkipped,
		StatusCodes:      copyStatusCodes(s.StatusCodes),
		Protocols:        copyProtocols(s.Protocols),
		Errors:           errorsFromMetrics(s.Errors),
		CheckFailures:    checkFailuresFromMetrics(s.CheckFailures),
	}
}

func scenariosFromSnapshot(snapshots []metrics.ScenarioSnapshot) []Scenario {
	scenarios := make([]Scenario, 0, len(snapshots))
	for _, s := range snapshots {
		scenarios = append(scenarios, Scenario{
			Name:             s.Name,
			Requests:         s.Total,
			Failed:           s.Failed,
			Canceled:         s.Canceled,
			Iterations:       s.Iterations,
			FailedIterations: s.FailedIterations,
			ErrorRate:        s.ErrorRate,
			RPSAvg:           s.RPSAvg,
			RPSPeak:          s.RPSPeak,
			P50MS:            durationMS(s.Latency.P50),
			P90MS:            durationMS(s.Latency.P90),
			P95MS:            durationMS(s.Latency.P95),
			P99MS:            durationMS(s.Latency.P99),
			ChecksTotal:      s.ChecksTotal,
			ChecksPassed:     s.ChecksPassed,
			TransferredBytes: s.TransferredBytes,
			AnalysisSkipped:  s.AnalysisSkipped,
			StatusCodes:      copyStatusCodes(s.StatusCodes),
			Protocols:        copyProtocols(s.Protocols),
		})
	}
	return scenarios
}

func thresholdsFromReport(results []report.ThresholdResult) []Threshold {
	thresholds := make([]Threshold, 0, len(results))
	for _, result := range results {
		thresholds = append(thresholds, Threshold{
			Name:      result.Name,
			Expected:  result.Expected,
			Observed:  result.Observed,
			Pass:      result.Pass,
			Evaluated: result.Evaluated,
			Limit:     result.Limit,
			Unit:      result.Unit,
		})
	}
	return thresholds
}

func diagnosisFromReport(findings []report.Finding) []Diagnosis {
	diagnosis := make([]Diagnosis, 0, len(findings))
	for _, finding := range findings {
		diagnosis = append(diagnosis, Diagnosis{
			Severity: string(finding.Severity),
			Message:  finding.Message,
			Partial:  finding.Partial,
		})
	}
	return diagnosis
}

func hostFromSummary(summary sysmetrics.Summary) Host {
	return Host{
		CPUAvg:        summary.CPUAvg,
		CPUMax:        summary.CPUMax,
		MemMaxPercent: summary.MemMaxPercent,
		NetRxPeak:     summary.NetRxPeak,
		NetTxPeak:     summary.NetTxPeak,
		Samples:       summary.Samples,
	}
}

func openLoopFromReport(summary *report.OpenLoopSummary) *OpenLoop {
	if summary == nil {
		return nil
	}
	open := &OpenLoop{
		TargetRate:   summary.TargetRate,
		AchievedRate: summary.AchievedRate,
		AchievedRPS:  summary.AchievedRPS,
		Dropped:      summary.Dropped,
		PeakInFlight: summary.PeakInFlight,
		Stages:       make([]OpenLoopStage, 0, len(summary.Stages)),
	}
	for _, stage := range summary.Stages {
		open.Stages = append(open.Stages, OpenLoopStage{
			Index:      stage.Index,
			DurationMS: durationMS(stage.Duration),
			TargetRate: stage.TargetRate,
			Requests:   stage.Requests,
			Failed:     stage.Failed,
			Dropped:    stage.Dropped,
			Iterations: stage.Iterations,
		})
	}
	return open
}

func copyProtocols(source map[string]int64) map[string]int64 {
	clone := make(map[string]int64, len(source))
	for protocol, count := range source {
		clone[protocol] = count
	}
	return clone
}

func errorsFromMetrics(errors []metrics.ErrorCount) []ErrorCount {
	if len(errors) == 0 {
		return nil
	}
	out := make([]ErrorCount, 0, len(errors))
	for _, errCount := range errors {
		out = append(out, ErrorCount{Kind: errCount.Kind, Count: errCount.Count})
	}
	return out
}

func checkFailuresFromMetrics(failures []metrics.CheckFailure) []CheckFailure {
	if len(failures) == 0 {
		return nil
	}
	out := make([]CheckFailure, 0, len(failures))
	for _, failure := range failures {
		out = append(out, CheckFailure{
			Scenario: failure.Scenario,
			Step:     failure.Step,
			Reason:   failure.Reason,
			Count:    failure.Count,
		})
	}
	return out
}

func copyStatusCodes(codes map[int]int64) map[int]int64 {
	if codes == nil {
		return nil
	}
	clone := make(map[int]int64, len(codes))
	for code, count := range codes {
		clone[code] = count
	}
	return clone
}

func durationMS(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}
