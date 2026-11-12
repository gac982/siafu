// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package results

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/gac982/siafu/internal/metrics"
	"github.com/gac982/siafu/internal/report"
	"github.com/gac982/siafu/internal/sysmetrics"
)

func testReport() report.Report {
	return report.Report{
		Name:      "Demo local",
		Target:    "http://127.0.0.1:8080",
		StartedAt: time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC),
		Planned:   10 * time.Second,
		Actual:    9 * time.Second,
		PeakVUs:   20,
		Seed:      42,
		Verdict:   report.VerdictPass,
		Global: metrics.Snapshot{
			Total:        100,
			Failed:       2,
			ErrorRate:    0.02,
			RPSAvg:       11.1,
			RPSPeak:      15,
			Iterations:   50,
			ChecksTotal:  100,
			ChecksPassed: 98,
			Latency:      metrics.Percentiles{P95: 40 * time.Millisecond, P99: 80 * time.Millisecond},
			StatusCodes:  map[int]int64{200: 100},
			Protocols:    map[string]int64{"HTTP/2.0": 100},
			Errors:       []metrics.ErrorCount{{Kind: "timeout", Count: 2}},
		},
		Scenarios: []metrics.ScenarioSnapshot{{
			Name:        "home",
			Total:       100,
			Failed:      2,
			Iterations:  50,
			ErrorRate:   0.02,
			RPSAvg:      11.1,
			Latency:     metrics.Percentiles{P95: 40 * time.Millisecond},
			StatusCodes: map[int]int64{200: 100},
			Protocols:   map[string]int64{"HTTP/2.0": 100},
		}},
		Host: sysmetrics.Summary{CPUAvg: 10, CPUMax: 20, MemMaxPercent: 30, NetRxPeak: 1000, NetTxPeak: 200},
		Thresholds: []report.ThresholdResult{{
			Name: "latencia p95", Expected: "≤40ms", Observed: "40ms", Pass: true, Evaluated: true,
			Limit: 40, Unit: "ms",
		}},
		Diagnosis: []report.Finding{{
			Severity: report.SeverityInfo, Message: "todo correcto",
		}},
	}
}

func TestFromReportClosed(t *testing.T) {
	meta := Meta{
		RunID:      "20261108-120000-demo-a1b2",
		CreatedAt:  time.Date(2026, 11, 8, 12, 0, 1, 0, time.UTC),
		Tag:        "smoke",
		Source:     "demo.yaml",
		ConfigHash: "sha256:abc",
		Version:    "v0.1.0",
		Commit:     "abc123",
		ExitCode:   0,
	}

	rec := FromReport(testReport(), meta)

	if rec.SchemaVersion != SchemaVersion {
		t.Errorf("schema version = %d, want %d", rec.SchemaVersion, SchemaVersion)
	}
	if rec.RunID != meta.RunID || rec.CreatedAt != meta.CreatedAt {
		t.Errorf("identity = %+v", rec)
	}
	if rec.Mode != "closed" {
		t.Errorf("mode = %q, want closed", rec.Mode)
	}
	if rec.Target != "http://127.0.0.1:8080" {
		t.Errorf("target = %q", rec.Target)
	}
	if rec.Global.Requests != 100 || rec.Global.Failed != 2 {
		t.Errorf("global = %+v", rec.Global)
	}
	if rec.Global.P95MS != 40 || rec.Global.P99MS != 80 {
		t.Errorf("percentiles = %+v", rec.Global)
	}
	if rec.Global.Protocols["HTTP/2.0"] != 100 {
		t.Errorf("protocols = %#v", rec.Global.Protocols)
	}
	if rec.PlannedMS != 10000 || rec.ActualMS != 9000 {
		t.Errorf("durations = %d/%d", rec.PlannedMS, rec.ActualMS)
	}
	if len(rec.Scenarios) != 1 || rec.Scenarios[0].Name != "home" {
		t.Errorf("scenarios = %+v", rec.Scenarios)
	}
	if len(rec.Thresholds) != 1 || !rec.Thresholds[0].Pass {
		t.Errorf("thresholds = %+v", rec.Thresholds)
	}
	if rec.Thresholds[0].Limit != 40 || rec.Thresholds[0].Unit != "ms" {
		t.Errorf("threshold limit/unit = %+v", rec.Thresholds[0])
	}
	if len(rec.Diagnosis) != 1 || rec.Diagnosis[0].Message != "todo correcto" {
		t.Errorf("diagnosis = %+v", rec.Diagnosis)
	}
	if rec.Host.CPUAvg != 10 || rec.Host.MemMaxPercent != 30 {
		t.Errorf("host = %+v", rec.Host)
	}
}

func TestFromReportOpenLoop(t *testing.T) {
	rep := testReport()
	rep.OpenLoop = &report.OpenLoopSummary{
		TargetRate:   20000,
		AchievedRate: 19999,
		AchievedRPS:  19999,
		Dropped:      3,
		PeakInFlight: 8000,
		Stages: []report.OpenLoopStage{{
			Index: 0, Duration: time.Second, TargetRate: 20000, Requests: 20000, Iterations: 20000,
		}},
	}

	rec := FromReport(rep, Meta{RunID: "open-1", CreatedAt: time.Now().UTC()})

	if rec.Mode != "open" {
		t.Errorf("mode = %q, want open", rec.Mode)
	}
	if rec.OpenLoop == nil {
		t.Fatal("OpenLoop is nil")
	}
	if rec.OpenLoop.TargetRate != 20000 || rec.OpenLoop.Dropped != 3 || rec.OpenLoop.PeakInFlight != 8000 {
		t.Errorf("open loop = %+v", rec.OpenLoop)
	}
	if len(rec.OpenLoop.Stages) != 1 || rec.OpenLoop.Stages[0].Iterations != 20000 {
		t.Errorf("stages = %+v", rec.OpenLoop.Stages)
	}
}

func TestIndexEntry(t *testing.T) {
	rec := RunRecord{
		SchemaVersion: SchemaVersion,
		RunID:         "run-1",
		CreatedAt:     time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC),
		Name:          "demo",
		Tag:           "smoke",
		Mode:          "open",
		Verdict:       "PASS",
		ExitCode:      0,
		PlannedMS:     10000,
		ActualMS:      9000,
		Global: Global{
			Requests:  100,
			Failed:    2,
			ErrorRate: 0.02,
			RPSAvg:    11.1,
			P95MS:     40,
		},
		OpenLoop: &OpenLoop{PeakInFlight: 8000, Dropped: 3},
	}

	idx := rec.IndexEntry()
	if idx.RunID != rec.RunID || idx.Mode != rec.Mode || idx.Verdict != rec.Verdict {
		t.Errorf("identity = %+v", idx)
	}
	if idx.DurationMS != 9000 || idx.Requests != 100 || idx.Failed != 2 {
		t.Errorf("summary = %+v", idx)
	}
	if idx.ErrorRate != 0.02 || idx.P95MS != 40 || idx.RPSAvg != 11.1 {
		t.Errorf("metrics = %+v", idx)
	}
	if idx.PeakInFlight != 8000 || idx.Dropped != 3 {
		t.Errorf("open loop = %+v", idx)
	}
}

func TestRecordJSONRoundTrip(t *testing.T) {
	rec := FromReport(testReport(), Meta{
		RunID:     "run-1",
		CreatedAt: time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC),
	})

	encoded, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	var decoded RunRecord
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rec, decoded) {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", decoded, rec)
	}
}
