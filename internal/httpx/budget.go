// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package httpx

import (
	"context"

	"golang.org/x/sync/semaphore"
)

// AnalysisBudget bounds the total bytes retained by concurrent response-body
// analysis. A nil or zero budget is unlimited.
type AnalysisBudget struct {
	sem *semaphore.Weighted
}

// NewAnalysisBudget creates a byte budget; mb<=0 disables the limit.
func NewAnalysisBudget(mb int) *AnalysisBudget {
	if mb <= 0 {
		return &AnalysisBudget{}
	}
	return &AnalysisBudget{sem: semaphore.NewWeighted(int64(mb) * 1024 * 1024)}
}

// Acquire reserves n bytes, waiting for other analyses to release capacity.
func (b *AnalysisBudget) Acquire(ctx context.Context, n int64) error {
	if b == nil || b.sem == nil || n <= 0 {
		return nil
	}
	return b.sem.Acquire(ctx, n)
}

// TryAcquire reserves n bytes without waiting. It reports false when the
// budget is exhausted.
func (b *AnalysisBudget) TryAcquire(n int64) bool {
	if b == nil || b.sem == nil || n <= 0 {
		return true
	}
	return b.sem.TryAcquire(n)
}

// Release returns n reserved bytes.
func (b *AnalysisBudget) Release(n int64) {
	if b == nil || b.sem == nil || n <= 0 {
		return
	}
	b.sem.Release(n)
}
