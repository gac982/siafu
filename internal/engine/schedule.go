// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package engine

import (
	"math"
	"time"

	"github.com/gac982/siafu/internal/config"
)

type stageSpan struct {
	start time.Duration
	end   time.Duration
	from  int
	to    int
}

type schedule struct {
	spans []stageSpan
	total time.Duration
}

func newSchedule(stages []config.Stage) *schedule {
	s := &schedule{spans: make([]stageSpan, 0, len(stages))}
	from := 0
	var cursor time.Duration
	for _, stage := range stages {
		duration := stage.Duration.Value()
		s.spans = append(s.spans, stageSpan{
			start: cursor,
			end:   cursor + duration,
			from:  from,
			to:    stage.TargetVUs,
		})
		cursor += duration
		from = stage.TargetVUs
	}
	s.total = cursor
	return s
}

// At returns the desired virtual users, the 1-based stage and the remaining
// duration for a given elapsed time.
func (s *schedule) At(elapsed time.Duration) (int, int, time.Duration) {
	if elapsed < 0 {
		elapsed = 0
	}
	if elapsed >= s.total {
		return 0, len(s.spans), 0
	}
	for i, span := range s.spans {
		if elapsed < span.end {
			spanDuration := span.end - span.start
			fraction := float64(elapsed-span.start) / float64(spanDuration)
			vus := int(math.Floor(float64(span.from) + fraction*float64(span.to-span.from) + 0.5))
			if vus < 0 {
				vus = 0
			}
			return vus, i + 1, span.end - elapsed
		}
	}
	return 0, len(s.spans), 0
}
