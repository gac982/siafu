// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package metrics

import (
	"context"
	"sync"
	"testing"
	"time"
)

func collect(t *testing.T, agg *Aggregator, batches ...Batch) Snapshot {
	t.Helper()
	in := make(chan Batch, len(batches)+1)
	out := agg.Run(context.Background(), in, time.Hour)
	for _, b := range batches {
		in <- b
	}
	close(in)

	snap, ok := <-out
	if !ok {
		t.Fatal("no snapshot received")
	}
	for range out {
	}
	return snap
}

func TestCountsAndErrorRate(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := func() time.Time { return start.Add(10 * time.Second) }
	agg := NewAggregator(start, now)

	snap := collect(t, agg, Batch{
		Requests: []Sample{
			{At: start, Duration: 10 * time.Millisecond, Status: 200, ChecksTotal: 1, ChecksPassed: 1},
			{At: start.Add(time.Second), Duration: 20 * time.Millisecond, Status: 200, ChecksTotal: 1, ChecksPassed: 1},
			{At: start.Add(2 * time.Second), Duration: 30 * time.Millisecond, Status: 200, ChecksTotal: 1, ChecksPassed: 1},
			{At: start.Add(3 * time.Second), Duration: 40 * time.Millisecond, Status: 500, Failed: true, CheckReason: "status", ChecksTotal: 1},
		},
		Iterations: []Iteration{
			{At: start, Failed: false},
			{At: start.Add(time.Second), Failed: true},
		},
	})

	if snap.Total != 4 || snap.Failed != 1 {
		t.Errorf("total/failed = %d/%d", snap.Total, snap.Failed)
	}
	if snap.ErrorRate != 0.25 {
		t.Errorf("error rate = %v, want 0.25", snap.ErrorRate)
	}
	if snap.ChecksTotal != 4 || snap.ChecksPassed != 3 {
		t.Errorf("checks = %d/%d", snap.ChecksPassed, snap.ChecksTotal)
	}
	if snap.Iterations != 2 || snap.FailedIterations != 1 {
		t.Errorf("iterations = %d/%d", snap.Iterations, snap.FailedIterations)
	}
	if snap.StatusCodes[500] != 1 || snap.StatusCodes[200] != 3 {
		t.Errorf("status codes = %v", snap.StatusCodes)
	}
	if len(snap.CheckFailures) != 1 || snap.CheckFailures[0].Reason != "status" {
		t.Errorf("check failures = %+v", snap.CheckFailures)
	}
}

func TestPercentilesKnownValues(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := func() time.Time { return start.Add(100 * time.Second) }
	agg := NewAggregator(start, now)

	samples := make([]Sample, 0, 100)
	for i := 1; i <= 100; i++ {
		samples = append(samples, Sample{
			At:       start.Add(time.Duration(i) * time.Second),
			Duration: time.Duration(i) * time.Millisecond,
			Status:   200,
		})
	}
	snap := collect(t, agg, Batch{Requests: samples})

	cases := []struct {
		name string
		got  time.Duration
		want time.Duration
	}{
		{"p50", snap.Latency.P50, 50 * time.Millisecond},
		{"p90", snap.Latency.P90, 90 * time.Millisecond},
		{"p95", snap.Latency.P95, 95 * time.Millisecond},
		{"p99", snap.Latency.P99, 99 * time.Millisecond},
		{"min", snap.Latency.Min, time.Millisecond},
		{"max", snap.Latency.Max, 100 * time.Millisecond},
	}
	for _, c := range cases {
		diff := c.got - c.want
		if diff < 0 {
			diff = -diff
		}
		if diff > 2*time.Millisecond {
			t.Errorf("%s = %v, want ~%v", c.name, c.got, c.want)
		}
	}
	if snap.Latency.Mean < 49*time.Millisecond || snap.Latency.Mean > 52*time.Millisecond {
		t.Errorf("mean = %v", snap.Latency.Mean)
	}
}

func TestStatusCodesAndErrors(t *testing.T) {
	start := time.Now()
	agg := NewAggregator(start, time.Now)
	snap := collect(t, agg, Batch{Requests: []Sample{
		{At: start, Duration: time.Millisecond, Status: 429, Failed: true, ErrorKind: "timeout"},
		{At: start, Duration: time.Millisecond, Status: 500, Failed: true, CheckReason: "status"},
	}})
	if snap.StatusCodes[429] != 1 || snap.StatusCodes[500] != 1 {
		t.Errorf("status = %v", snap.StatusCodes)
	}
	if len(snap.Errors) != 1 || snap.Errors[0].Kind != "timeout" || snap.Errors[0].Count != 1 {
		t.Errorf("errors = %+v", snap.Errors)
	}
}

func TestPerScenarioIsolation(t *testing.T) {
	start := time.Now()
	agg := NewAggregator(start, time.Now)
	snap := collect(t, agg, Batch{
		Requests: []Sample{
			{Scenario: "a", At: start, Duration: 5 * time.Millisecond, Status: 200, ChecksTotal: 1, ChecksPassed: 1},
			{Scenario: "a", At: start, Duration: 7 * time.Millisecond, Status: 200, ChecksTotal: 1, ChecksPassed: 1},
			{Scenario: "b", At: start, Duration: 9 * time.Millisecond, Status: 500, Failed: true, CheckReason: "status", ChecksTotal: 1},
		},
		Iterations: []Iteration{{Scenario: "a"}, {Scenario: "b", Failed: true}},
	})
	if len(snap.Scenarios) != 2 {
		t.Fatalf("scenarios = %+v", snap.Scenarios)
	}
	if snap.Scenarios[0].Name != "a" || snap.Scenarios[0].Total != 2 || snap.Scenarios[0].Failed != 0 {
		t.Errorf("scenario a = %+v", snap.Scenarios[0])
	}
	if snap.Scenarios[1].Name != "b" || snap.Scenarios[1].Total != 1 || snap.Scenarios[1].Failed != 1 {
		t.Errorf("scenario b = %+v", snap.Scenarios[1])
	}
}

func TestCanceledExcludedFromTotals(t *testing.T) {
	start := time.Now()
	agg := NewAggregator(start, time.Now)
	snap := collect(t, agg, Batch{Requests: []Sample{
		{Scenario: "a", At: start, Duration: time.Second, ErrorKind: "canceled"},
	}})
	if snap.Total != 0 || snap.Failed != 0 || snap.Canceled != 1 {
		t.Errorf("snapshot = total %d failed %d canceled %d", snap.Total, snap.Failed, snap.Canceled)
	}
	if len(snap.Scenarios) != 1 || snap.Scenarios[0].Total != 0 || snap.Scenarios[0].Canceled != 1 {
		t.Errorf("scenarios = %+v", snap.Scenarios)
	}
}

func TestRPSBucketsAndPeak(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := func() time.Time { return start.Add(10 * time.Second) }
	agg := NewAggregator(start, now)

	batch := Batch{}
	for i := 0; i < 10; i++ {
		batch.Requests = append(batch.Requests, Sample{At: start, Duration: time.Millisecond, Status: 200})
	}
	for i := 0; i < 20; i++ {
		batch.Requests = append(batch.Requests, Sample{At: start.Add(10 * time.Second), Duration: time.Millisecond, Status: 200})
	}
	snap := collect(t, agg, batch)

	if snap.RPSAvg != 3 {
		t.Errorf("RPSAvg = %v, want 3", snap.RPSAvg)
	}
	if snap.RPSRecent != 4 {
		t.Errorf("RPSRecent = %v, want 4", snap.RPSRecent)
	}
	if snap.RPSPeak != 4 {
		t.Errorf("RPSPeak = %v, want 4", snap.RPSPeak)
	}
	if snap.RPSPeakAt != 6*time.Second {
		t.Errorf("RPSPeakAt = %v, want 6s", snap.RPSPeakAt)
	}
}

func TestConcurrentBatches(t *testing.T) {
	start := time.Now()
	agg := NewAggregator(start, time.Now)
	in := make(chan Batch, 64)
	out := agg.Run(context.Background(), in, time.Hour)

	var wg sync.WaitGroup
	for g := 0; g < 50; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				in <- Batch{Requests: []Sample{{At: start, Duration: time.Millisecond, Status: 200}}}
			}
		}()
	}
	wg.Wait()
	close(in)

	snap := <-out
	for range out {
	}
	if snap.Total != 5000 {
		t.Fatalf("total = %d, want 5000", snap.Total)
	}
}

func TestSnapshotFinalOnClose(t *testing.T) {
	start := time.Now()
	agg := NewAggregator(start, time.Now)
	in := make(chan Batch)
	out := agg.Run(context.Background(), in, time.Hour)
	close(in)
	count := 0
	for range out {
		count++
	}
	if count != 1 {
		t.Fatalf("snapshots = %d, want 1", count)
	}
}

func BenchmarkAggregator(b *testing.B) {
	start := time.Now()
	agg := NewAggregator(start, time.Now)
	in := make(chan Batch, 1024)
	out := agg.Run(context.Background(), in, time.Hour)
	samples := make([]Sample, 64)
	for i := range samples {
		samples[i] = Sample{At: start, Duration: time.Millisecond, Status: 200, ChecksTotal: 1, ChecksPassed: 1}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		in <- Batch{Requests: samples}
	}
	close(in)
	for range out {
	}
}

func TestFailedStatusCodesOnlyCountFailedChecks(t *testing.T) {
	start := time.Now()
	agg := NewAggregator(start, time.Now)
	snap := collect(t, agg, Batch{Requests: []Sample{
		{At: start, Duration: time.Millisecond, Status: 500, Failed: true, CheckReason: "status"},
		{At: start, Duration: time.Millisecond, Status: 404, Failed: false},
		{At: start, Duration: time.Millisecond, Status: 200, Failed: true, CheckReason: "body"},
	}})
	if snap.FailedStatusCodes[500] != 1 {
		t.Errorf("failed status codes = %+v", snap.FailedStatusCodes)
	}
	if _, ok := snap.FailedStatusCodes[404]; ok {
		t.Errorf("expected 404 must not count as failed status: %+v", snap.FailedStatusCodes)
	}
}

func TestAnalysisSkippedCounted(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := func() time.Time { return start.Add(time.Second) }
	agg := NewAggregator(start, now)

	snap := collect(t, agg, Batch{Requests: []Sample{
		{Scenario: "s", At: start, Status: 200, AnalysisSkipped: true},
		{Scenario: "s", At: start, Status: 200},
	}})

	if snap.AnalysisSkipped != 1 {
		t.Errorf("global skipped = %d, want 1", snap.AnalysisSkipped)
	}
	if len(snap.Scenarios) != 1 || snap.Scenarios[0].AnalysisSkipped != 1 {
		t.Errorf("scenario skipped = %+v", snap.Scenarios)
	}
}

func TestProtocolCounts(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	agg := NewAggregator(start, func() time.Time { return start.Add(5 * time.Second) })
	snap := collect(t, agg, Batch{Requests: []Sample{
		{At: start, Duration: time.Millisecond, Status: 200, Protocol: "HTTP/2.0", Scenario: "s"},
		{At: start, Duration: time.Millisecond, Status: 200, Protocol: "HTTP/2.0", Scenario: "s"},
		{At: start, Duration: time.Millisecond, Status: 200, Protocol: "HTTP/1.1", Scenario: "s"},
	}})
	if snap.Protocols["HTTP/2.0"] != 2 || snap.Protocols["HTTP/1.1"] != 1 {
		t.Errorf("protocolos = %#v", snap.Protocols)
	}
	if len(snap.Scenarios) != 1 || snap.Scenarios[0].Protocols["HTTP/2.0"] != 2 {
		t.Errorf("protocolos por escenario = %#v", snap.Scenarios)
	}
}
