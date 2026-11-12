// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package engine

import (
	"testing"
	"time"

	"github.com/gac982/siafu/internal/config"
)

func TestScheduleBoundaries(t *testing.T) {
	stages := []config.Stage{
		{Duration: config.Duration{Duration: 10 * time.Second}, TargetVUs: 10},
		{Duration: config.Duration{Duration: 20 * time.Second}, TargetVUs: 30},
		{Duration: config.Duration{Duration: 5 * time.Second}, TargetVUs: 0},
	}
	s := newSchedule(stages)

	cases := []struct {
		elapsed time.Duration
		vus     int
		stage   int
		remain  time.Duration
	}{
		{0, 0, 1, 10 * time.Second},
		{5 * time.Second, 5, 1, 5 * time.Second},
		{10 * time.Second, 10, 2, 20 * time.Second},
		{20 * time.Second, 20, 2, 10 * time.Second},
		{22*time.Second + 500*time.Millisecond, 23, 2, 7500 * time.Millisecond},
		{30 * time.Second, 30, 3, 5 * time.Second},
		{32*time.Second + 500*time.Millisecond, 15, 3, 2500 * time.Millisecond},
		{35 * time.Second, 0, 3, 0},
		{40 * time.Second, 0, 3, 0},
	}
	for _, c := range cases {
		vus, stage, remain := s.At(c.elapsed)
		if vus != c.vus || stage != c.stage || remain != c.remain {
			t.Errorf("At(%v) = (%d,%d,%v), want (%d,%d,%v)", c.elapsed, vus, stage, remain, c.vus, c.stage, c.remain)
		}
	}
}
