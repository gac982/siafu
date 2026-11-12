// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package engine

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gac982/siafu/internal/config"
	"github.com/gac982/siafu/internal/httpx"
	"github.com/gac982/siafu/internal/metrics"
	"github.com/gac982/siafu/internal/redact"
	"github.com/gac982/siafu/internal/vars"
)

type vuHandle struct {
	retire   chan struct{}
	retiring bool
}

// Engine runs one load test.
type Engine struct {
	cfg       *config.Config
	opts      Options
	plan      *httpx.Plan
	transport *httpx.Transport
	globals   map[string]vars.Template
	redactor  *redact.Redactor
	analysis  *httpx.AnalysisBudget
	seed      int64

	started  atomic.Bool
	stopCh   chan struct{}
	stopOnce sync.Once
	cancel   context.CancelFunc
	wg       sync.WaitGroup

	runMu sync.Mutex
	run   *Run

	statusMu sync.RWMutex
	status   Status
}

// New prepares the engine. Configuration is expected to be valid.
func New(cfg *config.Config, opts Options) (*Engine, error) {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.SnapshotEvery <= 0 {
		opts.SnapshotEvery = time.Second
	}
	if opts.TickEvery <= 0 {
		opts.TickEvery = 250 * time.Millisecond
	}
	if opts.StopGrace <= 0 {
		opts.StopGrace = 15 * time.Second
	}
	if opts.Logger == nil {
		opts.Logger = NopLogger{}
	}

	plan, err := httpx.Prepare(cfg)
	if err != nil {
		return nil, fmt.Errorf("preparar escenarios: %w", err)
	}
	maxConns := cfg.Load.MaxVUs
	if cfg.Load.Mode == config.ModeOpen {
		maxConns = cfg.Load.OpenLoop.MaxInFlight
	}
	transport, err := httpx.NewTransportWithIdle(cfg.Target, maxConns, cfg.Limits.MaxIdleConns)
	if err != nil {
		return nil, fmt.Errorf("preparar transporte: %w", err)
	}

	globals := make(map[string]vars.Template, len(cfg.Vars))
	for name, raw := range cfg.Vars {
		tpl, err := vars.Compile(raw)
		if err != nil {
			return nil, fmt.Errorf("vars.%s: %w", name, err)
		}
		globals[name] = tpl
	}

	secrets := make([]string, 0, len(cfg.EnvRefs()))
	for _, name := range cfg.EnvRefs() {
		if value := os.Getenv(name); value != "" {
			secrets = append(secrets, value)
		}
	}

	seed := opts.Seed
	if seed == 0 && cfg.Load.Seed != nil {
		seed = *cfg.Load.Seed
	}
	if seed == 0 {
		seed = opts.Now().UnixNano()
	}

	planned := time.Duration(0)
	for _, stage := range cfg.Load.Stages {
		planned += stage.Duration.Value()
	}

	e := &Engine{
		cfg:       cfg,
		opts:      opts,
		plan:      plan,
		transport: transport,
		globals:   globals,
		redactor:  redact.NewExplicit(secrets),
		analysis:  httpx.NewAnalysisBudget(cfg.Limits.AnalysisBudgetMB),
		seed:      seed,
		stopCh:    make(chan struct{}),
	}
	e.status = Status{
		Stages:  len(cfg.Load.Stages),
		Planned: planned,
	}
	return e, nil
}

// Start launches the run and returns its live channels. It is single-use.
func (e *Engine) Start(ctx context.Context) *Run {
	if !e.started.CompareAndSwap(false, true) {
		e.runMu.Lock()
		defer e.runMu.Unlock()
		return e.run
	}

	start := e.opts.Now()
	runCtx, cancel := context.WithCancel(ctx)
	e.cancel = cancel

	batches := make(chan metrics.Batch, 1024)
	aggregator := metrics.NewAggregator(start, e.opts.Now)
	aggregatorOut := aggregator.Run(runCtx, batches, e.opts.SnapshotEvery)

	public := make(chan metrics.Snapshot, 16)
	finalCh := make(chan metrics.Snapshot, 1)
	go func() {
		var last metrics.Snapshot
		for snapshot := range aggregatorOut {
			last = snapshot
			if e.opts.OnSnapshot != nil {
				e.opts.OnSnapshot(snapshot)
			}
			select {
			case public <- snapshot:
			default:
			}
		}
		finalCh <- last
		close(public)
	}()

	doneCh := make(chan Result, 1)
	run := &Run{Snapshots: public, Done: doneCh}
	e.runMu.Lock()
	e.run = run
	e.runMu.Unlock()

	if e.cfg.Load.Mode == config.ModeOpen {
		go e.superviseOpen(runCtx, start, batches, finalCh, doneCh)
	} else {
		go e.supervise(runCtx, start, batches, finalCh, doneCh)
	}
	return run
}

// Stop requests a graceful stop. It is idempotent.
func (e *Engine) Stop() {
	e.stopOnce.Do(func() { close(e.stopCh) })
}

// Status returns the current engine state.
func (e *Engine) Status() Status {
	e.statusMu.RLock()
	defer e.statusMu.RUnlock()
	return e.status
}

func (e *Engine) supervise(ctx context.Context, start time.Time, batches chan metrics.Batch, finalCh <-chan metrics.Snapshot, done chan<- Result) {
	defer e.cancel()
	defer func() { _ = e.transport.Close() }()

	sched := newSchedule(e.cfg.Load.Stages)
	ticker := time.NewTicker(e.opts.TickEvery)
	defer ticker.Stop()

	active := make(map[int]*vuHandle, maxInt(1, e.cfg.Load.MaxVUs))
	doneVU := make(chan int, maxInt(1, e.cfg.Load.MaxVUs+1))
	nextID := 0
	peak := 0
	aborted := false
	stopping := false

	for !stopping {
		elapsed := e.opts.Now().Sub(start)
		if elapsed < 0 {
			elapsed = 0
		}
		if elapsed >= sched.total {
			break
		}

		desired, stage, _ := sched.At(elapsed)
		e.setStatus(len(active), desired, peak, nextID, stage, len(sched.spans), elapsed, sched.total, false)

		for len(active) < desired {
			id := nextID
			nextID++
			handle := &vuHandle{retire: make(chan struct{})}
			active[id] = handle
			if len(active) > peak {
				peak = len(active)
			}
			e.spawn(ctx, id, handle.retire, batches, doneVU)
		}

		surplus := len(active) - desired
		retired := 0
		if surplus > 0 {
			for _, handle := range active {
				if handle.retiring {
					continue
				}
				close(handle.retire)
				handle.retiring = true
				retired++
				if retired >= surplus {
					break
				}
			}
		}

		select {
		case <-ctx.Done():
			aborted = true
			stopping = true
		case <-e.stopCh:
			aborted = true
			stopping = true
		case id := <-doneVU:
			delete(active, id)
		case <-ticker.C:
		}
		if stopping {
			break
		}
	}

	elapsed := e.opts.Now().Sub(start)
	e.setStatus(len(active), 0, peak, nextID, len(sched.spans), len(sched.spans), elapsed, sched.total, true)
	for _, handle := range active {
		if !handle.retiring {
			close(handle.retire)
			handle.retiring = true
		}
	}

	waited := make(chan struct{})
	go func() {
		e.wg.Wait()
		close(waited)
	}()
	select {
	case <-waited:
	case <-time.After(e.opts.StopGrace):
		e.cancel()
		<-waited
	}

	close(batches)
	final := <-finalCh
	e.setStatus(0, 0, peak, nextID, len(sched.spans), len(sched.spans), final.Elapsed, sched.total, true)
	done <- Result{Snapshot: final, Aborted: aborted, Seed: e.seed}
}

func (e *Engine) spawn(ctx context.Context, id int, retire <-chan struct{}, batches chan<- metrics.Batch, doneVU chan<- int) {
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		defer func() {
			select {
			case doneVU <- id:
			default:
			}
		}()

		client, err := httpx.NewClient(e.cfg.Target, e.transport, e.redactor)
		if err != nil {
			e.opts.Logger.Error(fmt.Sprintf("VU %d: %v", id+1, err))
			return
		}
		client.SetAnalysisBudget(e.analysis)
		scenario := e.plan.Scenarios[id%len(e.plan.Scenarios)]
		worker := newVU(id, scenario, client, e.globals, batches, e.opts.Logger, e.seed)
		worker.cfg = e.cfg.Load
		worker.maxBatch = e.cfg.Limits.MaxBatchSamples
		worker.run(ctx, retire)
	}()
}

func (e *Engine) setStatus(active, target, peak, spawned, stage, stages int, elapsed, planned time.Duration, finishing bool) {
	e.statusMu.Lock()
	e.status = Status{
		Mode:       config.ModeClosed,
		ActiveVUs:  active,
		TargetVUs:  target,
		PeakVUs:    peak,
		SpawnedVUs: spawned,
		Stage:      stage,
		Stages:     stages,
		Elapsed:    elapsed,
		Planned:    planned,
		Finishing:  finishing,
	}
	e.statusMu.Unlock()
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
