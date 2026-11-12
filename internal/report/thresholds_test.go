// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package report

import (
	"testing"
	"time"

	"github.com/gac982/siafu/internal/config"
	"github.com/gac982/siafu/internal/metrics"
)

func TestEvaluateExposesLimits(t *testing.T) {
	p95 := config.Duration{Duration: 300 * time.Millisecond}
	p99 := config.Duration{Duration: 500 * time.Millisecond}
	errorRate := 0.01
	minRPS := 10.0

	results := Evaluate(config.Thresholds{
		LatencyP95: &p95,
		LatencyP99: &p99,
		ErrorRate:  &errorRate,
		MinRPS:     &minRPS,
	}, metrics.Snapshot{})

	want := map[string]struct {
		limit float64
		unit  string
	}{
		"latencia p95":  {300, "ms"},
		"latencia p99":  {500, "ms"},
		"tasa de error": {0.01, "ratio"},
		"RPS promedio":  {10, "rps"},
	}
	if len(results) != len(want) {
		t.Fatalf("results = %d, want %d", len(results), len(want))
	}
	for _, result := range results {
		expected, ok := want[result.Name]
		if !ok {
			t.Errorf("unexpected threshold %q", result.Name)
			continue
		}
		if result.Limit != expected.limit || result.Unit != expected.unit {
			t.Errorf("%s limit/unit = %v/%q, want %v/%q", result.Name, result.Limit, result.Unit, expected.limit, expected.unit)
		}
	}
}
