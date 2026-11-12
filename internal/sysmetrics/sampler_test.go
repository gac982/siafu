// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package sysmetrics

import (
	"context"
	"runtime"
	"testing"
	"time"
)

func TestSamplerProducesValidSnapshots(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("host metrics are only supported on linux and darwin")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out := NewSampler(150 * time.Millisecond).Run(ctx)
	var samples []Snapshot
	timeout := time.After(3 * time.Second)
	for len(samples) < 2 {
		select {
		case snap, ok := <-out:
			if !ok {
				t.Fatal("channel closed early")
			}
			samples = append(samples, snap)
		case <-timeout:
			t.Fatalf("only %d samples received", len(samples))
		}
	}

	for _, snap := range samples {
		if snap.CPUPercent < 0 || snap.CPUPercent > 100 {
			t.Errorf("CPU = %v", snap.CPUPercent)
		}
		if snap.MemTotal == 0 {
			t.Errorf("MemTotal = 0")
		}
		if snap.MemPercent < 0 || snap.MemPercent > 100 {
			t.Errorf("MemPercent = %v", snap.MemPercent)
		}
		if snap.Err == nil && (snap.DiskUsedPercent < 0 || snap.DiskUsedPercent > 100) {
			t.Errorf("DiskUsedPercent = %v", snap.DiskUsedPercent)
		}
	}
}

func TestSamplerCancelClosesChannel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	out := NewSampler(50 * time.Millisecond).Run(ctx)
	cancel()
	select {
	case _, ok := <-out:
		if ok {
			// A final sample may already be buffered; drain until close.
			for range out {
			}
		}
	case <-time.After(time.Second):
		t.Fatal("channel did not close after cancel")
	}
}

func TestAccumulator(t *testing.T) {
	var acc Accumulator
	acc.Add(Snapshot{CPUPercent: 10, MemPercent: 40, NetRxBps: 100, NetTxBps: 50})
	acc.Add(Snapshot{CPUPercent: 30, MemPercent: 60, NetRxBps: 300, NetTxBps: 20})
	acc.Add(Snapshot{CPUPercent: 20, MemPercent: 50, NetRxBps: 200, NetTxBps: 70})

	summary := acc.Summary()
	if summary.Samples != 3 {
		t.Errorf("Samples = %d", summary.Samples)
	}
	if summary.CPUAvg != 20 {
		t.Errorf("CPUAvg = %v, want 20", summary.CPUAvg)
	}
	if summary.CPUMax != 30 {
		t.Errorf("CPUMax = %v, want 30", summary.CPUMax)
	}
	if summary.MemMaxPercent != 60 {
		t.Errorf("MemMaxPercent = %v, want 60", summary.MemMaxPercent)
	}
	if summary.NetRxPeak != 300 || summary.NetTxPeak != 70 {
		t.Errorf("net peaks = %v/%v", summary.NetRxPeak, summary.NetTxPeak)
	}
}

func TestAccumulatorEmpty(t *testing.T) {
	var acc Accumulator
	summary := acc.Summary()
	if summary.Samples != 0 || summary.CPUAvg != 0 {
		t.Errorf("summary = %+v", summary)
	}
}
