// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package engine

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gac982/siafu/internal/config"
	"github.com/gac982/siafu/internal/httpx"
	"github.com/gac982/siafu/internal/metrics"
	"github.com/gac982/siafu/internal/vars"
)

// OpenLoopStage is the per-stage result of an open-loop run.
type OpenLoopStage struct {
	Index      int
	Duration   time.Duration
	TargetRate float64
	Requests   int64
	Failed     int64
	Dropped    int64
	Iterations int64
}

// OpenLoopResult is the open-loop summary carried into the report.
type OpenLoopResult struct {
	TargetRate   float64
	AchievedRate float64
	AchievedRPS  float64
	Dropped      int64
	PeakInFlight int
	Stages       []OpenLoopStage
}

var errNoWorkers = errors.New("no se pudo crear ningún worker open-loop")

type openLoopWorker struct {
	vu    *vu
	scope vars.Scope
}

func (e *Engine) superviseOpen(ctx context.Context, start time.Time, batches chan metrics.Batch, finalCh <-chan metrics.Snapshot, done chan<- Result) {
	defer e.cancel()

	stageConfig := e.cfg.Load.OpenLoop.Stages
	schedule := newRateSchedule(stageConfig)
	maxInFlight := e.cfg.Load.OpenLoop.MaxInFlight
	if maxInFlight < 1 {
		maxInFlight = 1
	}

	flusher := newFlusher(batches, e.cfg.Limits.MaxBatchSamples)
	flushDone := make(chan struct{})
	go func() {
		defer close(flushDone)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				flusher.flush(ctx)
			case <-ctx.Done():
				return
			case <-e.stopCh:
				return
			}
		}
	}()

	idle := make(chan *openLoopWorker, maxInFlight)
	for i := 0; i < maxInFlight; i++ {
		client, err := httpx.NewClient(e.cfg.Target, e.transport, e.redactor)
		if err != nil {
			e.opts.Logger.Error("open-loop worker: " + err.Error())
			continue
		}
		client.SetAnalysisBudget(e.analysis)
		client.SetAnalysisPolicy(e.cfg.Load.OpenLoop.OnAnalysisExhausted)
		scenario := e.plan.Scenarios[i%len(e.plan.Scenarios)]
		worker := newVU(i, scenario, client, e.globals, batches, e.opts.Logger, e.seed)
		worker.cfg = e.cfg.Load
		worker.maxBatch = e.cfg.Limits.MaxBatchSamples
		idle <- &openLoopWorker{
			vu: worker,
			scope: vars.Scope{
				VU:        i + 1,
				Iter:      1,
				Extracted: map[string]string{},
				Globals:   e.globals,
				LookupEnv: os.LookupEnv,
			},
		}
	}
	if len(idle) == 0 {
		close(batches)
		final := <-finalCh
		done <- Result{Snapshot: final, Seed: e.seed, Err: errNoWorkers}
		return
	}

	var workerWG sync.WaitGroup
	var inFlight, dropped, completed, requests int64
	var peakInFlight int64

	var stageMu sync.Mutex
	var stageRequests, stageFailed, stageDropped, stageIterations int64
	stageResults := make([]OpenLoopStage, 0, len(stageConfig))
	currentStage := 0

	recordStage := func(index int, duration time.Duration) {
		if index < 0 || index >= len(stageConfig) {
			return
		}
		stageMu.Lock()
		stageResults = append(stageResults, OpenLoopStage{
			Index:      index,
			Duration:   duration,
			TargetRate: stageConfig[index].TargetRate,
			Requests:   stageRequests,
			Failed:     stageFailed,
			Dropped:    stageDropped,
			Iterations: stageIterations,
		})
		stageRequests, stageFailed, stageDropped, stageIterations = 0, 0, 0, 0
		stageMu.Unlock()
	}

	addStats := func(stats iterationStats) {
		stageMu.Lock()
		stageRequests += int64(stats.Requests)
		stageFailed += int64(stats.Failed)
		stageIterations++
		stageMu.Unlock()
	}

	dispatch := func(worker *openLoopWorker) {
		current := atomic.AddInt64(&inFlight, 1)
		if current > atomic.LoadInt64(&peakInFlight) {
			atomic.StoreInt64(&peakInFlight, current)
		}
		workerWG.Add(1)
		go func() {
			defer workerWG.Done()
			defer func() {
				atomic.AddInt64(&inFlight, -1)
				idle <- worker
			}()
			ok, stats := worker.vu.runIteration(ctx, &worker.scope, flusher)
			if !ok {
				return
			}
			atomic.AddInt64(&completed, 1)
			atomic.AddInt64(&requests, int64(stats.Requests))
			addStats(stats)
		}()
	}

	var acc rateAccumulator
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	last := start
	stageStart := start
	aborted := false

loop:
	for {
		elapsed := e.opts.Now().Sub(start)
		if elapsed >= schedule.Total() {
			break
		}
		select {
		case <-ctx.Done():
			aborted = true
			break loop
		case <-e.stopCh:
			aborted = true
			break loop
		case <-ticker.C:
		}

		elapsed = e.opts.Now().Sub(start)
		now := e.opts.Now()
		dt := now.Sub(last)
		last = now
		if dt > 100*time.Millisecond {
			dt = 100 * time.Millisecond
		}

		rate := schedule.At(elapsed)
		stage := schedule.StageAt(elapsed)
		if stage != currentStage {
			recordStage(currentStage, now.Sub(stageStart))
			stageStart = now
			currentStage = stage
		}

		for n := acc.Add(rate, dt); n > 0; n-- {
			select {
			case worker := <-idle:
				dispatch(worker)
			default:
				atomic.AddInt64(&dropped, 1)
				stageMu.Lock()
				stageDropped++
				stageMu.Unlock()
			}
		}

		achieved := 0.0
		if seconds := elapsed.Seconds(); seconds > 0 {
			achieved = float64(atomic.LoadInt64(&completed)) / seconds
		}
		e.setOpenStatus(
			int(atomic.LoadInt64(&inFlight)),
			maxInFlight,
			int(atomic.LoadInt64(&peakInFlight)),
			rate,
			achieved,
			atomic.LoadInt64(&dropped),
			stage+1,
			len(stageConfig),
			elapsed,
			schedule.Total(),
			false,
		)
	}

	e.setOpenStatus(
		int(atomic.LoadInt64(&inFlight)),
		maxInFlight,
		int(atomic.LoadInt64(&peakInFlight)),
		schedule.At(e.opts.Now().Sub(start)),
		float64(atomic.LoadInt64(&completed))/maxSeconds(e.opts.Now().Sub(start).Seconds()),
		atomic.LoadInt64(&dropped),
		len(stageConfig),
		len(stageConfig),
		e.opts.Now().Sub(start),
		schedule.Total(),
		true,
	)
	waited := make(chan struct{})
	go func() {
		workerWG.Wait()
		close(waited)
	}()
	select {
	case <-waited:
	case <-time.After(e.opts.StopGrace):
		e.cancel()
		<-waited
	}
	recordStage(currentStage, e.opts.Now().Sub(stageStart))
	e.Stop() // unblock the flusher without canceling a normal completion
	<-flushDone
	flusher.flush(context.Background())
	close(batches)

	final := <-finalCh
	elapsed := final.Elapsed
	if elapsed <= 0 {
		elapsed = e.opts.Now().Sub(start)
	}
	result := &OpenLoopResult{
		TargetRate:   schedule.At(elapsed),
		Dropped:      atomic.LoadInt64(&dropped),
		PeakInFlight: int(atomic.LoadInt64(&peakInFlight)),
		Stages:       stageResults,
	}
	if seconds := elapsed.Seconds(); seconds > 0 {
		result.AchievedRate = float64(atomic.LoadInt64(&completed)) / seconds
		result.AchievedRPS = float64(atomic.LoadInt64(&requests)) / seconds
	}
	if result.TargetRate == 0 && len(stageConfig) > 0 {
		result.TargetRate = stageConfig[len(stageConfig)-1].TargetRate
	}

	e.setOpenStatus(
		int(atomic.LoadInt64(&inFlight)),
		maxInFlight,
		int(atomic.LoadInt64(&peakInFlight)),
		result.TargetRate,
		result.AchievedRate,
		result.Dropped,
		len(stageConfig),
		len(stageConfig),
		elapsed,
		schedule.Total(),
		true,
	)
	done <- Result{Snapshot: final, Aborted: aborted, Seed: e.seed, OpenLoop: result}
}

func (e *Engine) setOpenStatus(inFlight, maxInFlight, peak int, targetRate, achievedRate float64, dropped int64, stage, stages int, elapsed, planned time.Duration, finishing bool) {
	e.statusMu.Lock()
	e.status = Status{
		Mode:         config.ModeOpen,
		ActiveVUs:    inFlight,
		TargetVUs:    maxInFlight,
		PeakVUs:      peak,
		InFlight:     inFlight,
		MaxInFlight:  maxInFlight,
		TargetRate:   targetRate,
		AchievedRate: achievedRate,
		Dropped:      dropped,
		Stage:        stage,
		Stages:       stages,
		Elapsed:      elapsed,
		Planned:      planned,
		Finishing:    finishing,
	}
	e.statusMu.Unlock()
}

func maxSeconds(seconds float64) float64 {
	if seconds <= 0 {
		return 1
	}
	return seconds
}
