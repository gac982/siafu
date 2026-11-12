// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package report

import (
	"strings"
	"testing"
	"time"

	"github.com/gac982/siafu/internal/metrics"
	"github.com/gac982/siafu/internal/sysmetrics"
)

func passingThresholds() []ThresholdResult {
	return []ThresholdResult{
		{Name: "latencia p95", Pass: true, Evaluated: true},
		{Name: "latencia p99", Pass: true, Evaluated: true},
		{Name: "tasa de error", Pass: true, Evaluated: true},
		{Name: "RPS promedio", Pass: true, Evaluated: true},
	}
}

func diagnosisReport() Report {
	return Report{
		Name:    "diag",
		Actual:  100 * time.Second,
		Verdict: VerdictPass,
		Global: metrics.Snapshot{
			Total:             100,
			Failed:            0,
			ErrorRate:         0,
			RPSAvg:            100,
			RPSRecent:         100,
			RPSPeak:           100,
			RPSPeakAt:         90 * time.Second,
			Latency:           metrics.Percentiles{P95: 100 * time.Millisecond},
			StatusCodes:       map[int]int64{200: 100},
			FailedStatusCodes: map[int]int64{},
			Errors:            nil,
			ChecksTotal:       100,
			ChecksPassed:      100,
		},
		Host:       sysmetrics.Summary{CPUAvg: 20, CPUMax: 40},
		Thresholds: passingThresholds(),
	}
}

func hasFinding(findings []Finding, substring string) bool {
	for _, finding := range findings {
		if strings.Contains(finding.Message, substring) {
			return true
		}
	}
	return false
}

func TestDiagnoseNoRequests(t *testing.T) {
	r := diagnosisReport()
	r.Global.Total = 0
	findings := Diagnose(r)
	if len(findings) != 1 || findings[0].Severity != SeverityCritical || !strings.Contains(findings[0].Message, "ninguna petición") {
		t.Fatalf("findings = %+v", findings)
	}
}

func TestDiagnoseTemplateErrors(t *testing.T) {
	r := diagnosisReport()
	r.Global.Errors = []metrics.ErrorCount{{Kind: "template", Count: 3}}
	if findings := Diagnose(r); !hasFinding(findings, "variables") {
		t.Fatalf("findings = %+v", findings)
	}
}

func TestDiagnoseTransportErrors(t *testing.T) {
	r := diagnosisReport()
	r.Global.Errors = []metrics.ErrorCount{{Kind: "timeout", Count: 20}, {Kind: "dns", Count: 5}}
	findings := Diagnose(r)
	if !hasFinding(findings, "transporte") || !hasFinding(findings, "timeout") {
		t.Fatalf("findings = %+v", findings)
	}
}

func TestDiagnose429(t *testing.T) {
	r := diagnosisReport()
	r.Global.StatusCodes = map[int]int64{429: 40, 500: 5}
	r.Global.FailedStatusCodes = map[int]int64{429: 40, 500: 5}
	r.Global.Errors = nil
	if findings := Diagnose(r); !hasFinding(findings, "rate limiting") {
		t.Fatalf("findings = %+v", findings)
	}
}

func TestDiagnose5xx(t *testing.T) {
	r := diagnosisReport()
	r.Global.StatusCodes = map[int]int64{500: 40, 503: 10}
	r.Global.FailedStatusCodes = map[int]int64{500: 40, 503: 10}
	if findings := Diagnose(r); !hasFinding(findings, "5xx") {
		t.Fatalf("findings = %+v", findings)
	}
}

func TestDiagnose4xx(t *testing.T) {
	r := diagnosisReport()
	r.Global.StatusCodes = map[int]int64{400: 30, 404: 20}
	r.Global.FailedStatusCodes = map[int]int64{400: 30, 404: 20}
	if findings := Diagnose(r); !hasFinding(findings, "4xx") {
		t.Fatalf("findings = %+v", findings)
	}
}

func TestDiagnoseExpected4xxNoFinding(t *testing.T) {
	r := diagnosisReport()
	r.Global.StatusCodes = map[int]int64{404: 100}
	r.Global.FailedStatusCodes = map[int]int64{}
	r.Global.Failed = 0
	if findings := Diagnose(r); hasFinding(findings, "4xx") {
		t.Fatalf("expected 404 produced a status finding: %+v", findings)
	}
}

func TestDiagnoseLatencyTrend(t *testing.T) {
	r := diagnosisReport()
	r.Thresholds[0].Pass = false
	r.Global.RPSPeak = 100
	r.Global.RPSPeakAt = 10 * time.Second
	r.Global.RPSRecent = 50
	if findings := Diagnose(r); !hasFinding(findings, "Throughput") {
		t.Fatalf("findings = %+v", findings)
	}
}

func TestDiagnoseLatencyNoTrend(t *testing.T) {
	r := diagnosisReport()
	r.Thresholds[0].Pass = false
	r.Global.RPSRecent = 100
	if findings := Diagnose(r); !hasFinding(findings, "Latencia alta") {
		t.Fatalf("findings = %+v", findings)
	}
}

func TestDiagnoseMinRPS(t *testing.T) {
	r := diagnosisReport()
	for i := range r.Thresholds {
		if r.Thresholds[i].Name == "RPS promedio" {
			r.Thresholds[i].Pass = false
		}
	}
	if findings := Diagnose(r); !hasFinding(findings, "suficiente carga") {
		t.Fatalf("findings = %+v", findings)
	}
}

func TestDiagnoseGeneratorCPU(t *testing.T) {
	r := diagnosisReport()
	r.Host.CPUAvg = 90
	if findings := Diagnose(r); !hasFinding(findings, "generador") {
		t.Fatalf("findings = %+v", findings)
	}
}

func TestDiagnoseBodyTooLarge(t *testing.T) {
	r := diagnosisReport()
	r.Global.Errors = []metrics.ErrorCount{{Kind: "body_too_large", Count: 2}}
	if findings := Diagnose(r); !hasFinding(findings, "límite de lectura") {
		t.Fatalf("findings = %+v", findings)
	}
}

func TestDiagnoseAllPass(t *testing.T) {
	findings := Diagnose(diagnosisReport())
	if len(findings) != 1 || findings[0].Severity != SeverityInfo || !strings.Contains(findings[0].Message, "umbrales") {
		t.Fatalf("findings = %+v", findings)
	}
}

func TestDiagnoseAbortedMarksPartial(t *testing.T) {
	r := diagnosisReport()
	r.Aborted = true
	r.Verdict = VerdictAborted
	r.Thresholds = nil
	r.Global.Errors = []metrics.ErrorCount{{Kind: "timeout", Count: 20}}
	findings := Diagnose(r)
	if len(findings) == 0 {
		t.Fatal("no findings")
	}
	for _, finding := range findings {
		if !finding.Partial {
			t.Errorf("finding not marked partial: %+v", finding)
		}
	}
	if hasFinding(findings, "umbrales") {
		t.Errorf("aborted run produced compliance finding: %+v", findings)
	}
}

func TestDiagnose429Beats5xx(t *testing.T) {
	r := diagnosisReport()
	r.Global.StatusCodes = map[int]int64{429: 30, 500: 30}
	r.Global.FailedStatusCodes = map[int]int64{429: 30, 500: 30}
	findings := Diagnose(r)
	if !hasFinding(findings, "rate limiting") {
		t.Fatalf("findings = %+v", findings)
	}
	if hasFinding(findings, "5xx") {
		t.Errorf("duplicate cause reported: %+v", findings)
	}
}
