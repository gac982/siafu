// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package httpx

import (
	"context"
	"testing"
	"time"
)

const halfMegabyte = 512 * 1024

func TestAnalysisBudgetAcquireBytes(t *testing.T) {
	budget := NewAnalysisBudget(1)
	if err := budget.Acquire(context.Background(), halfMegabyte); err != nil {
		t.Fatal(err)
	}
	if err := budget.Acquire(context.Background(), halfMegabyte); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if err := budget.Acquire(ctx, 1); err == nil {
		t.Fatal("Acquire past the budget succeeded")
	}

	budget.Release(halfMegabyte)
	if err := budget.Acquire(context.Background(), halfMegabyte); err != nil {
		t.Fatalf("Acquire after release: %v", err)
	}
	budget.Release(halfMegabyte)
	budget.Release(halfMegabyte)
}

func TestAnalysisBudgetZeroIsUnlimited(t *testing.T) {
	budget := NewAnalysisBudget(0)
	for i := 0; i < 5; i++ {
		if err := budget.Acquire(context.Background(), 2*1024*1024); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAnalysisBudgetCanceled(t *testing.T) {
	budget := NewAnalysisBudget(1)
	if err := budget.Acquire(context.Background(), 1024*1024); err != nil {
		t.Fatal(err)
	}
	defer budget.Release(1024 * 1024)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := budget.Acquire(ctx, 1); err == nil {
		t.Fatal("Acquire with canceled context succeeded")
	}
}

func TestAnalysisBudgetTryAcquire(t *testing.T) {
	budget := NewAnalysisBudget(1)
	if !budget.TryAcquire(halfMegabyte) {
		t.Fatal("first TryAcquire failed")
	}
	if !budget.TryAcquire(halfMegabyte) {
		t.Fatal("second TryAcquire failed")
	}
	if budget.TryAcquire(1) {
		t.Fatal("TryAcquire past the budget succeeded")
	}
	budget.Release(halfMegabyte)
	budget.Release(halfMegabyte)
}
