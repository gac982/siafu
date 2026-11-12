// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package engine

import (
	"math"
	"testing"
	"time"

	"github.com/gac982/siafu/internal/config"
)

func rateStage(d time.Duration, rate float64) config.RateStage {
	return config.RateStage{Duration: config.Duration{Duration: d}, TargetRate: rate}
}

func TestRateScheduleInterpolates(t *testing.T) {
	s := newRateSchedule([]config.RateStage{
		rateStage(60*time.Second, 1000),
		rateStage(30*time.Second, 1000),
		rateStage(10*time.Second, 0),
	})

	cases := []struct {
		at   time.Duration
		want float64
	}{
		{0, 0},
		{30 * time.Second, 500},
		{60 * time.Second, 1000},
		{90 * time.Second, 1000},
		{95 * time.Second, 500},
		{100 * time.Second, 0},
	}
	for _, tc := range cases {
		if got := s.At(tc.at); math.Abs(got-tc.want) > 0.001 {
			t.Errorf("At(%s) = %v, want %v", tc.at, got, tc.want)
		}
	}
	if s.Total() != 100*time.Second {
		t.Errorf("Total() = %s", s.Total())
	}
}

func TestRateScheduleAfterEnd(t *testing.T) {
	s := newRateSchedule([]config.RateStage{rateStage(10*time.Second, 100)})
	if got := s.At(20 * time.Second); got != 0 {
		t.Errorf("At(after end) = %v, want 0", got)
	}
}

func TestRateAccumulator(t *testing.T) {
	var acc rateAccumulator
	total := 0
	for i := 0; i < 100; i++ {
		total += acc.Add(1000, 10*time.Millisecond)
	}
	if total != 1000 {
		t.Fatalf("dispatched %d iterations, want 1000", total)
	}
}

func TestRateAccumulatorNoBurst(t *testing.T) {
	var acc rateAccumulator
	for i := 0; i < 1000; i++ {
		n := acc.Add(1000, 10*time.Millisecond)
		if n > 11 {
			t.Fatalf("tick %d dispatched %d, want <= 11", i, n)
		}
	}
}
