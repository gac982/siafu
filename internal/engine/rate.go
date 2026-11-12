// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package engine

import (
	"time"

	"github.com/gac982/siafu/internal/config"
)

type rateSpan struct {
	start, end time.Duration
	from, to   float64
}

// rateSchedule interpolates the target arrival rate across open-loop stages.
type rateSchedule struct {
	spans []rateSpan
	total time.Duration
}

func newRateSchedule(stages []config.RateStage) *rateSchedule {
	s := &rateSchedule{}
	from := 0.0
	start := time.Duration(0)
	for _, stage := range stages {
		duration := stage.Duration.Value()
		if duration <= 0 {
			continue
		}
		end := start + duration
		s.spans = append(s.spans, rateSpan{
			start: start,
			end:   end,
			from:  from,
			to:    stage.TargetRate,
		})
		start = end
		from = stage.TargetRate
	}
	s.total = start
	return s
}

// Total returns the planned duration of every stage.
func (s *rateSchedule) Total() time.Duration { return s.total }

// At returns the target arrival rate at elapsed. Rates ramp linearly from the
// previous stage target, starting at zero.
func (s *rateSchedule) At(elapsed time.Duration) float64 {
	if elapsed <= 0 || s.total <= 0 || elapsed >= s.total {
		return 0
	}
	for _, span := range s.spans {
		if elapsed < span.end {
			if span.end <= span.start {
				return span.to
			}
			frac := float64(elapsed-span.start) / float64(span.end-span.start)
			return span.from + (span.to-span.from)*frac
		}
	}
	return 0
}

// StageAt returns the zero-based stage index at elapsed.
func (s *rateSchedule) StageAt(elapsed time.Duration) int {
	for i, span := range s.spans {
		if elapsed < span.end {
			return i
		}
	}
	if len(s.spans) == 0 {
		return 0
	}
	return len(s.spans) - 1
}

// rateAccumulator converts a fractional per-tick rate into whole dispatches.
type rateAccumulator struct {
	pending float64
}

// Add accumulates rate*dt and returns the whole iterations to dispatch now.
func (a *rateAccumulator) Add(rate float64, dt time.Duration) int {
	if rate <= 0 || dt <= 0 {
		return 0
	}
	a.pending += rate * dt.Seconds()
	n := int(a.pending)
	a.pending -= float64(n)
	return n
}
