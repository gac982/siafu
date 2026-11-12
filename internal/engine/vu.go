// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package engine

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"sync"
	"time"

	"github.com/gac982/siafu/internal/config"
	"github.com/gac982/siafu/internal/httpx"
	"github.com/gac982/siafu/internal/metrics"
	"github.com/gac982/siafu/internal/vars"
)

type vu struct {
	id       int
	scenario httpx.Scenario
	client   *httpx.Client
	globals  map[string]vars.Template
	batches  chan<- metrics.Batch
	logger   Logger
	rng      *rand.Rand
	maxBatch int
	cfg      config.LoadConfig
}

func newVU(id int, scenario httpx.Scenario, client *httpx.Client, globals map[string]vars.Template, batches chan<- metrics.Batch, logger Logger, seed int64) *vu {
	if logger == nil {
		logger = NopLogger{}
	}
	return &vu{
		id:       id,
		scenario: scenario,
		client:   client,
		globals:  globals,
		batches:  batches,
		logger:   logger,
		rng:      rand.New(rand.NewPCG(uint64(seed), uint64(id))),
		maxBatch: 64,
	}
}

func (v *vu) run(ctx context.Context, retire <-chan struct{}) {
	flusher := newFlusher(v.batches, v.maxBatch)
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				flusher.flush(ctx)
			case <-stop:
				return
			}
		}
	}()
	defer func() {
		close(stop)
		<-done
		flusher.flush(context.Background())
	}()

	scope := vars.Scope{
		VU:        v.id + 1,
		Iter:      1,
		Extracted: map[string]string{},
		Globals:   v.globals,
		LookupEnv: os.LookupEnv,
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-retire:
			return
		default:
		}

		ok, _ := v.runIteration(ctx, &scope, flusher)
		if !ok {
			return
		}
		if !v.think(ctx, retire) {
			return
		}
	}
}

// iterationStats counts the work done by one iteration.
type iterationStats struct {
	Requests        int
	Failed          int
	FailedIteration bool
}

// runIteration executes one full scenario iteration and reports whether the
// context is still usable. It is shared by closed and open-loop modes.
func (v *vu) runIteration(ctx context.Context, scope *vars.Scope, flusher *flusher) (bool, iterationStats) {
	defer v.client.CloseWebSockets()
	resetExtracted(scope)
	stats := iterationStats{}
	iterationFailed := false

	for _, step := range v.scenario.Steps {
		result := v.client.Do(ctx, step, *scope)
		if result.ErrKind == httpx.ErrCanceled {
			return false, stats
		}
		stats.Requests++

		full := flusher.addSample(metrics.Sample{
			Scenario:        v.scenario.Name,
			Step:            step.Name,
			At:              time.Now(),
			Duration:        result.Duration,
			Status:          result.Status,
			Protocol:        result.Protocol,
			Bytes:           result.Bytes,
			Failed:          result.Failed(),
			ErrorKind:       string(result.ErrKind),
			ChecksTotal:     result.ChecksTotal(),
			ChecksPassed:    result.ChecksPassed(),
			CheckReason:     result.CheckReason,
			AnalysisSkipped: result.AnalysisSkipped,
		})
		if full {
			flusher.flush(ctx)
		}

		if result.Failed() {
			iterationFailed = true
			stats.Failed++
			v.logFailure(step.Name, result)
		}
		if result.ErrKind != "" {
			break
		}
		if result.AnalysisSkipped && len(step.Extract) > 0 {
			iterationFailed = true
			break
		}
		for key, value := range result.Extracted {
			scope.Extracted[key] = value
		}
	}

	flusher.addIteration(metrics.Iteration{
		Scenario: v.scenario.Name,
		At:       time.Now(),
		Failed:   iterationFailed,
	})

	if scope.Iter%100 == 0 {
		v.logger.Info(fmt.Sprintf("%s · VU %d · iteración %d", v.scenario.Name, v.id+1, scope.Iter))
	}
	scope.Iter++
	stats.FailedIteration = iterationFailed
	return true, stats
}

func (v *vu) logFailure(step string, result httpx.Result) {
	if result.ErrKind != "" {
		v.logger.Error(fmt.Sprintf("%s/%s → %s: %v", v.scenario.Name, step, result.ErrKind, result.Err))
		return
	}
	v.logger.Warn(fmt.Sprintf("%s/%s → %d (%s)", v.scenario.Name, step, result.Status, result.CheckReason))
}

func (v *vu) think(ctx context.Context, retire <-chan struct{}) bool {
	min := v.cfg.ThinkTime.Min.Value()
	max := v.cfg.ThinkTime.Max.Value()
	if max <= 0 {
		return true
	}
	if max < min {
		max = min
	}
	wait := min
	if max > min {
		wait += time.Duration(v.rng.Int64N(int64(max-min) + 1))
	}
	if wait <= 0 {
		return true
	}

	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-retire:
		return false
	case <-timer.C:
		return true
	}
}

func resetExtracted(scope *vars.Scope) {
	if scope.Extracted == nil {
		scope.Extracted = make(map[string]string)
		return
	}
	clear(scope.Extracted)
}

type flusher struct {
	mu         sync.Mutex
	requests   []metrics.Sample
	iterations []metrics.Iteration
	out        chan<- metrics.Batch
	maxSamples int
}

func newFlusher(out chan<- metrics.Batch, maxSamples int) *flusher {
	if maxSamples <= 0 {
		maxSamples = 64
	}
	return &flusher{out: out, maxSamples: maxSamples}
}

func (f *flusher) addSample(sample metrics.Sample) bool {
	f.mu.Lock()
	f.requests = append(f.requests, sample)
	full := len(f.requests) >= f.maxSamples
	f.mu.Unlock()
	return full
}

func (f *flusher) full() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.requests) >= f.maxSamples
}

func (f *flusher) addIteration(iteration metrics.Iteration) {
	f.mu.Lock()
	f.iterations = append(f.iterations, iteration)
	f.mu.Unlock()
}

func (f *flusher) flush(ctx context.Context) {
	f.mu.Lock()
	if len(f.requests) == 0 && len(f.iterations) == 0 {
		f.mu.Unlock()
		return
	}
	batch := metrics.Batch{Requests: f.requests, Iterations: f.iterations}
	f.requests = nil
	f.iterations = nil
	f.mu.Unlock()

	select {
	case f.out <- batch:
	case <-ctx.Done():
	}
}
