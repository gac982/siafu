// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package metrics

import (
	"time"

	hdrhistogram "github.com/HdrHistogram/hdrhistogram-go"
)

const (
	histogramMin = int64(1)           // 1 microsecond
	histogramMax = int64(300_000_000) // 300 seconds in microseconds
)

func newHistogram() *hdrhistogram.Histogram {
	return hdrhistogram.New(histogramMin, histogramMax, 3)
}

func recordDuration(hist *hdrhistogram.Histogram, d time.Duration) {
	value := d.Microseconds()
	if value < histogramMin {
		value = histogramMin
	}
	if value > histogramMax {
		value = histogramMax
	}
	_ = hist.RecordValue(value)
}

// Percentiles is a duration distribution.
type Percentiles struct {
	P50  time.Duration
	P90  time.Duration
	P95  time.Duration
	P99  time.Duration
	Min  time.Duration
	Max  time.Duration
	Mean time.Duration
}

func percentiles(hist *hdrhistogram.Histogram) Percentiles {
	if hist == nil || hist.TotalCount() == 0 {
		return Percentiles{}
	}
	us := func(value int64) time.Duration { return time.Duration(value) * time.Microsecond }
	return Percentiles{
		P50:  us(hist.ValueAtQuantile(50)),
		P90:  us(hist.ValueAtQuantile(90)),
		P95:  us(hist.ValueAtQuantile(95)),
		P99:  us(hist.ValueAtQuantile(99)),
		Min:  us(hist.Min()),
		Max:  us(hist.Max()),
		Mean: time.Duration(hist.Mean()) * time.Microsecond,
	}
}
