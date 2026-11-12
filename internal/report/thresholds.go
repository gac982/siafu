// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package report

import (
	"fmt"
	"time"

	"github.com/gac982/siafu/internal/config"
	"github.com/gac982/siafu/internal/metrics"
)

// Evaluate compares every configured threshold with the final snapshot.
func Evaluate(thresholds config.Thresholds, snapshot metrics.Snapshot) []ThresholdResult {
	var results []ThresholdResult

	if thresholds.LatencyP95 != nil {
		expected := thresholds.LatencyP95.Value()
		observed := snapshot.Latency.P95
		results = append(results, ThresholdResult{
			Name:      "latencia p95",
			Expected:  "≤ " + formatDuration(expected),
			Observed:  formatDuration(observed),
			Pass:      observed <= expected,
			Evaluated: true,
			Limit:     float64(expected) / float64(time.Millisecond),
			Unit:      "ms",
		})
	}
	if thresholds.LatencyP99 != nil {
		expected := thresholds.LatencyP99.Value()
		observed := snapshot.Latency.P99
		results = append(results, ThresholdResult{
			Name:      "latencia p99",
			Expected:  "≤ " + formatDuration(expected),
			Observed:  formatDuration(observed),
			Pass:      observed <= expected,
			Evaluated: true,
			Limit:     float64(expected) / float64(time.Millisecond),
			Unit:      "ms",
		})
	}
	if thresholds.ErrorRate != nil {
		expected := *thresholds.ErrorRate
		observed := snapshot.ErrorRate
		results = append(results, ThresholdResult{
			Name:      "tasa de error",
			Expected:  fmt.Sprintf("≤ %.2f%%", expected*100),
			Observed:  fmt.Sprintf("%.2f%%", observed*100),
			Pass:      observed <= expected,
			Evaluated: true,
			Limit:     expected,
			Unit:      "ratio",
		})
	}
	if thresholds.MinRPS != nil {
		expected := *thresholds.MinRPS
		observed := snapshot.RPSAvg
		results = append(results, ThresholdResult{
			Name:      "RPS promedio",
			Expected:  fmt.Sprintf("≥ %.1f", expected),
			Observed:  fmt.Sprintf("%.1f", observed),
			Pass:      observed >= expected,
			Evaluated: true,
			Limit:     expected,
			Unit:      "rps",
		})
	}

	return results
}

func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}
