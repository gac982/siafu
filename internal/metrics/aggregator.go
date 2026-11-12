// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package metrics

import (
	"context"
	"sort"
	"time"

	hdrhistogram "github.com/HdrHistogram/hdrhistogram-go"
)

const canceledKind = "canceled"

type checkKey struct {
	scenario string
	step     string
	reason   string
}

type bucket struct {
	total            int64
	failed           int64
	canceled         int64
	iterations       int64
	failedIterations int64
	checksTotal      int64
	checksPassed     int64
	analysisSkipped  int64
	bytes            int64
	hist             *hdrhistogram.Histogram
	status           map[int]int64
	failedStatus     map[int]int64
	protocols        map[string]int64
	errors           map[string]int64
	checkFailures    map[checkKey]int64
	perSecond        map[int64]int64
	peak             float64
	peakAt           time.Duration
}

func newBucket() *bucket {
	return &bucket{
		hist:          newHistogram(),
		status:        map[int]int64{},
		failedStatus:  map[int]int64{},
		protocols:     map[string]int64{},
		errors:        map[string]int64{},
		checkFailures: map[checkKey]int64{},
		perSecond:     map[int64]int64{},
	}
}

func (b *bucket) updatePeak(second int64) {
	sum := int64(0)
	for s := second - 4; s <= second; s++ {
		if s >= 0 {
			sum += b.perSecond[s]
		}
	}
	rps := float64(sum) / 5
	if rps > b.peak {
		b.peak = rps
		start := second - 4
		if start < 0 {
			start = 0
		}
		b.peakAt = time.Duration(start) * time.Second
	}
}

func windowCount(counts map[int64]int64, second int64) int64 {
	sum := int64(0)
	for s := second - 4; s <= second; s++ {
		if s >= 0 {
			sum += counts[s]
		}
	}
	return sum
}

// Aggregator owns all metric state behind a single goroutine.
type Aggregator struct {
	start     time.Time
	now       func() time.Time
	global    *bucket
	scenarios map[string]*bucket
}

// NewAggregator creates an aggregator; now is injectable for tests.
func NewAggregator(start time.Time, now func() time.Time) *Aggregator {
	if now == nil {
		now = time.Now
	}
	return &Aggregator{
		start:     start,
		now:       now,
		global:    newBucket(),
		scenarios: map[string]*bucket{},
	}
}

// Run consumes batches until the channel closes, emitting periodic snapshots
// and one final snapshot. The returned channel is closed after the final one.
func (a *Aggregator) Run(ctx context.Context, in <-chan Batch, every time.Duration) <-chan Snapshot {
	if every <= 0 {
		every = time.Second
	}
	out := make(chan Snapshot, 4)

	go func() {
		defer close(out)
		ticker := time.NewTicker(every)
		defer ticker.Stop()

		for {
			select {
			case batch, ok := <-in:
				if !ok {
					out <- a.snapshot()
					return
				}
				a.add(batch)
			case <-ticker.C:
				out <- a.snapshot()
			case <-ctx.Done():
				// Engine guarantees closing the channel after VUs stop.
				for batch := range in {
					a.add(batch)
				}
				out <- a.snapshot()
				return
			}
		}
	}()

	return out
}

func (a *Aggregator) add(batch Batch) {
	for _, sample := range batch.Requests {
		a.addSample(sample)
	}
	for _, iteration := range batch.Iterations {
		b := a.scenario(iteration.Scenario)
		a.global.iterations++
		b.iterations++
		if iteration.Failed {
			a.global.failedIterations++
			b.failedIterations++
		}
	}
}

func (a *Aggregator) addSample(sample Sample) {
	b := a.scenario(sample.Scenario)

	if sample.ErrorKind == canceledKind {
		a.global.canceled++
		b.canceled++
		return
	}

	a.global.total++
	b.total++
	if sample.Failed {
		a.global.failed++
		b.failed++
	}
	a.global.checksTotal += int64(sample.ChecksTotal)
	a.global.checksPassed += int64(sample.ChecksPassed)
	b.checksTotal += int64(sample.ChecksTotal)
	b.checksPassed += int64(sample.ChecksPassed)
	if sample.AnalysisSkipped {
		a.global.analysisSkipped++
		b.analysisSkipped++
	}
	a.global.bytes += sample.Bytes
	b.bytes += sample.Bytes
	if sample.Protocol != "" {
		a.global.protocols[sample.Protocol]++
		b.protocols[sample.Protocol]++
	}

	recordDuration(a.global.hist, sample.Duration)
	recordDuration(b.hist, sample.Duration)

	if sample.Status != 0 {
		a.global.status[sample.Status]++
		b.status[sample.Status]++
		if sample.CheckReason == "status" {
			a.global.failedStatus[sample.Status]++
			b.failedStatus[sample.Status]++
		}
	}
	if sample.ErrorKind != "" {
		a.global.errors[sample.ErrorKind]++
		b.errors[sample.ErrorKind]++
	}
	if sample.CheckReason != "" {
		key := checkKey{scenario: sample.Scenario, step: sample.Step, reason: sample.CheckReason}
		a.global.checkFailures[key]++
		b.checkFailures[key]++
	}

	second := int64(sample.At.Sub(a.start) / time.Second)
	if second < 0 {
		second = 0
	}
	a.global.perSecond[second]++
	a.global.updatePeak(second)
	b.perSecond[second]++
	b.updatePeak(second)
}

func (a *Aggregator) scenario(name string) *bucket {
	if name == "" {
		name = "(sin escenario)"
	}
	if b, ok := a.scenarios[name]; ok {
		return b
	}
	b := newBucket()
	a.scenarios[name] = b
	return b
}

func (a *Aggregator) snapshot() Snapshot {
	now := a.now()
	elapsed := now.Sub(a.start)
	if elapsed < 0 {
		elapsed = 0
	}
	second := int64(elapsed / time.Second)

	snap := Snapshot{
		StartedAt:         a.start,
		Elapsed:           elapsed,
		Total:             a.global.total,
		Failed:            a.global.failed,
		Canceled:          a.global.canceled,
		RPSRecent:         float64(windowCount(a.global.perSecond, second)) / 5,
		RPSPeak:           a.global.peak,
		RPSPeakAt:         a.global.peakAt,
		Latency:           percentiles(a.global.hist),
		StatusCodes:       copyStatus(a.global.status),
		FailedStatusCodes: copyStatus(a.global.failedStatus),
		Protocols:         copyProtocols(a.global.protocols),
		Errors:            sortedErrors(a.global.errors),
		CheckFailures:     sortedCheckFailures(a.global.checkFailures),
		ChecksTotal:       a.global.checksTotal,
		ChecksPassed:      a.global.checksPassed,
		Iterations:        a.global.iterations,
		FailedIterations:  a.global.failedIterations,
		TransferredBytes:  a.global.bytes,
		AnalysisSkipped:   a.global.analysisSkipped,
	}
	if elapsed > 0 {
		snap.RPSAvg = float64(a.global.total) / elapsed.Seconds()
	}
	if snap.Total > 0 {
		snap.ErrorRate = float64(snap.Failed) / float64(snap.Total)
	}

	names := make([]string, 0, len(a.scenarios))
	for name := range a.scenarios {
		names = append(names, name)
	}
	sort.Strings(names)
	snap.Scenarios = make([]ScenarioSnapshot, 0, len(names))
	for _, name := range names {
		b := a.scenarios[name]
		scenario := ScenarioSnapshot{
			Name:             name,
			Total:            b.total,
			Failed:           b.failed,
			Canceled:         b.canceled,
			Iterations:       b.iterations,
			FailedIterations: b.failedIterations,
			RPSPeak:          b.peak,
			Latency:          percentiles(b.hist),
			ChecksTotal:      b.checksTotal,
			ChecksPassed:     b.checksPassed,
			TransferredBytes: b.bytes,
			AnalysisSkipped:  b.analysisSkipped,
			StatusCodes:      copyStatus(b.status),
			Protocols:        copyProtocols(b.protocols),
		}
		if elapsed > 0 {
			scenario.RPSAvg = float64(b.total) / elapsed.Seconds()
		}
		if b.total > 0 {
			scenario.ErrorRate = float64(b.failed) / float64(b.total)
		}
		snap.Scenarios = append(snap.Scenarios, scenario)
	}

	return snap
}

func copyStatus(source map[int]int64) map[int]int64 {
	clone := make(map[int]int64, len(source))
	for code, count := range source {
		clone[code] = count
	}
	return clone
}

func copyProtocols(source map[string]int64) map[string]int64 {
	clone := make(map[string]int64, len(source))
	for protocol, count := range source {
		clone[protocol] = count
	}
	return clone
}

func sortedErrors(source map[string]int64) []ErrorCount {
	errors := make([]ErrorCount, 0, len(source))
	for kind, count := range source {
		errors = append(errors, ErrorCount{Kind: kind, Count: count})
	}
	sort.Slice(errors, func(i, j int) bool {
		if errors[i].Count != errors[j].Count {
			return errors[i].Count > errors[j].Count
		}
		return errors[i].Kind < errors[j].Kind
	})
	return errors
}

func sortedCheckFailures(source map[checkKey]int64) []CheckFailure {
	failures := make([]CheckFailure, 0, len(source))
	for key, count := range source {
		failures = append(failures, CheckFailure{
			Scenario: key.scenario,
			Step:     key.step,
			Reason:   key.reason,
			Count:    count,
		})
	}
	sort.Slice(failures, func(i, j int) bool {
		if failures[i].Count != failures[j].Count {
			return failures[i].Count > failures[j].Count
		}
		if failures[i].Scenario != failures[j].Scenario {
			return failures[i].Scenario < failures[j].Scenario
		}
		if failures[i].Step != failures[j].Step {
			return failures[i].Step < failures[j].Step
		}
		return failures[i].Reason < failures[j].Reason
	})
	return failures
}
