// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

// Package orchestrator wires the engine, metrics, host sampler and UI.
package orchestrator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/gac982/siafu/internal/admission"
	"github.com/gac982/siafu/internal/config"
	"github.com/gac982/siafu/internal/engine"
	"github.com/gac982/siafu/internal/logbuf"
	"github.com/gac982/siafu/internal/memguard"
	"github.com/gac982/siafu/internal/metrics"
	"github.com/gac982/siafu/internal/redact"
	"github.com/gac982/siafu/internal/report"
	"github.com/gac982/siafu/internal/results"
	"github.com/gac982/siafu/internal/sysmetrics"
	"github.com/gac982/siafu/internal/tui"
)

// Options configures one orchestrated run.
type Options struct {
	NoTUI      bool
	Debug      bool
	Seed       int64
	Stdout     io.Writer
	Stderr     io.Writer
	IsTTY      bool
	Now        func() time.Time
	ResultsDir string
	NoSave     bool
	Tag        string
	ConfigPath string
	Version    string
	Commit     string
}

// Run executes the load test and returns the report and the process exit code.
func Run(ctx context.Context, cfg *config.Config, opts Options) (rep report.Report, code int) {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}

	recorder := NewRecorder()
	defer func() {
		saveRunIfNeeded(opts, rep, recorder, code)
	}()

	logs := logbuf.New(cfg.Limits.MaxLogLines)
	secrets := make([]string, 0, len(cfg.EnvRefs()))
	for _, name := range cfg.EnvRefs() {
		if value := os.Getenv(name); value != "" {
			secrets = append(secrets, value)
		}
	}
	redactor := redact.NewExplicit(secrets)
	logs.SetRedactor(redactor)
	plain := opts.NoTUI || !opts.IsTTY
	if plain {
		logs.SetPlain(opts.Stderr, redactor)
	}

	if err := admission.Check(cfg); err != nil {
		fmt.Fprintln(opts.Stderr, err)
		return report.Report{Verdict: report.VerdictError}, 2
	}

	var loadEngine *engine.Engine
	loadEngine, err := engine.New(cfg, engine.Options{
		Seed:   opts.Seed,
		Logger: logs,
		Now:    opts.Now,
		OnSnapshot: func(snapshot metrics.Snapshot) {
			recorder.AddMetrics(snapshot, loadEngine.Status())
		},
	})
	if err != nil {
		fmt.Fprintln(opts.Stderr, err)
		return report.Report{Verdict: report.VerdictError}, 3
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	run := loadEngine.Start(runCtx)
	hostCh := sysmetrics.NewSampler(time.Second).Run(runCtx)
	host := &sysmetrics.Accumulator{}
	var memoryExceeded atomic.Bool
	guard := memguard.New(cfg.Limits.MemoryBudgetMB, cfg.Limits.WarnAtPercent, logs, func() {
		memoryExceeded.Store(true)
		cancel()
	})
	go guard.Run(runCtx)

	if opts.Debug {
		logs.Debug("modo debug activado")
	}

	if plain {
		return runPlain(runCtx, cfg, loadEngine, run, hostCh, host, &memoryExceeded, recorder, opts)
	}
	return runTUI(runCtx, cancel, cfg, loadEngine, run, hostCh, host, logs, &memoryExceeded, recorder, opts)
}

func runPlain(ctx context.Context, cfg *config.Config, loadEngine *engine.Engine, run *engine.Run, hostCh <-chan sysmetrics.Snapshot, host *sysmetrics.Accumulator, memoryExceeded *atomic.Bool, recorder *Recorder, opts Options) (report.Report, int) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var latest metrics.Snapshot
	for {
		select {
		case snapshot, ok := <-run.Snapshots:
			if ok {
				latest = snapshot
			}
		case sample, ok := <-hostCh:
			if ok {
				host.Add(sample)
				recorder.AddHost(sample)
			}
		case <-ticker.C:
			status := loadEngine.Status()
			if status.Mode == config.ModeOpen {
				fmt.Fprintf(opts.Stdout, "[siafu] %s/%s · etapa %d/%d · target %.0f/s · achieved %.0f/s · in-flight %d · descartes %d · p95 %s · errores %.2f%%\n",
					shortDuration(status.Elapsed), shortDuration(status.Planned),
					status.Stage, status.Stages,
					status.TargetRate, status.AchievedRate,
					status.InFlight, status.Dropped,
					shortDuration(latest.Latency.P95), latest.ErrorRate*100)
			} else {
				fmt.Fprintf(opts.Stdout, "[siafu] %s/%s · etapa %d/%d · VUs %d/%d · RPS %.1f · p95 %s · errores %.2f%%\n",
					shortDuration(status.Elapsed), shortDuration(status.Planned),
					status.Stage, status.Stages,
					status.ActiveVUs, status.TargetVUs,
					latest.RPSRecent, shortDuration(latest.Latency.P95), latest.ErrorRate*100)
			}
		case result := <-run.Done:
			rep := buildReport(cfg, loadEngine, result, host, memoryExceeded.Load())
			_ = report.WriteText(opts.Stdout, rep)
			return rep, rep.ExitCode()
		case <-ctx.Done():
			select {
			case result := <-run.Done:
				rep := buildReport(cfg, loadEngine, result, host, memoryExceeded.Load())
				_ = report.WriteText(opts.Stdout, rep)
				return rep, abortCode(rep)
			case <-time.After(2 * time.Second):
				if memoryExceeded.Load() {
					return report.Report{Verdict: report.VerdictError, MemoryExceeded: true}, 3
				}
				return report.Report{Verdict: report.VerdictAborted, Aborted: true}, 130
			}
		}
	}
}

func runTUI(ctx context.Context, cancel context.CancelFunc, cfg *config.Config, loadEngine *engine.Engine, run *engine.Run, hostCh <-chan sysmetrics.Snapshot, host *sysmetrics.Accumulator, logs *logbuf.Buffer, memoryExceeded *atomic.Bool, recorder *Recorder, opts Options) (report.Report, int) {
	var finished atomic.Bool

	model := tui.New(tui.Actions{
		Stop: loadEngine.Stop,
		Quit: func() {
			if !finished.Load() {
				cancel()
			}
		},
	})
	program := tea.NewProgram(model, tea.WithContext(ctx))

	programDone := make(chan error, 1)
	go func() {
		_, err := program.Run()
		programDone <- err
	}()

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	var latest metrics.Snapshot
	var hostSnapshot sysmetrics.Snapshot
	for {
		select {
		case snapshot, ok := <-run.Snapshots:
			if ok {
				latest = snapshot
			}
		case sample, ok := <-hostCh:
			if ok {
				hostSnapshot = sample
				host.Add(sample)
				recorder.AddHost(sample)
			}
		case <-ticker.C:
			program.Send(tui.FrameMsg{Frame: frameFrom(cfg, latest, hostSnapshot, logs, loadEngine.Status())})
		case result := <-run.Done:
			finished.Store(true)
			rep := buildReport(cfg, loadEngine, result, host, memoryExceeded.Load())
			program.Send(tui.ReportMsg{Report: rep})
			select {
			case err := <-programDone:
				if err != nil {
					fmt.Fprintln(opts.Stderr, err)
					return rep, 3
				}
				return rep, rep.ExitCode()
			case <-ctx.Done():
				program.Quit()
				<-programDone
				return rep, abortCode(rep)
			}
		case <-ctx.Done():
			program.Quit()
			<-programDone
			select {
			case result := <-run.Done:
				rep := buildReport(cfg, loadEngine, result, host, memoryExceeded.Load())
				return rep, abortCode(rep)
			case <-time.After(2 * time.Second):
				if memoryExceeded.Load() {
					return report.Report{Verdict: report.VerdictError, MemoryExceeded: true}, 3
				}
				return report.Report{Verdict: report.VerdictAborted, Aborted: true}, 130
			}
		case err := <-programDone:
			if err != nil {
				fmt.Fprintln(opts.Stderr, err)
			}
			select {
			case <-run.Done:
			case <-time.After(2 * time.Second):
			}
			if memoryExceeded.Load() {
				return report.Report{Verdict: report.VerdictError, MemoryExceeded: true}, 3
			}
			return report.Report{Verdict: report.VerdictAborted, Aborted: true}, 130
		}
	}
}

func buildReport(cfg *config.Config, loadEngine *engine.Engine, result engine.Result, host *sysmetrics.Accumulator, memoryExceeded bool) report.Report {
	status := loadEngine.Status()
	return report.Build(result.Snapshot, cfg, host.Summary(), report.Meta{
		Seed:           result.Seed,
		PeakVUs:        status.PeakVUs,
		Planned:        status.Planned,
		Actual:         result.Snapshot.Elapsed,
		Aborted:        result.Aborted,
		MemoryExceeded: memoryExceeded,
		OpenLoop:       openLoopSummary(result.OpenLoop),
	})
}

func openLoopSummary(result *engine.OpenLoopResult) *report.OpenLoopSummary {
	if result == nil {
		return nil
	}
	stages := make([]report.OpenLoopStage, len(result.Stages))
	for i, stage := range result.Stages {
		stages[i] = report.OpenLoopStage{
			Index:      stage.Index,
			Duration:   stage.Duration,
			TargetRate: stage.TargetRate,
			Requests:   stage.Requests,
			Failed:     stage.Failed,
			Dropped:    stage.Dropped,
			Iterations: stage.Iterations,
		}
	}
	return &report.OpenLoopSummary{
		TargetRate:   result.TargetRate,
		AchievedRate: result.AchievedRate,
		AchievedRPS:  result.AchievedRPS,
		Dropped:      result.Dropped,
		PeakInFlight: result.PeakInFlight,
		Stages:       stages,
	}
}

func abortCode(rep report.Report) int {
	if rep.Verdict == report.VerdictError {
		return 3
	}
	return 130
}

func shortDuration(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	total := int64(d / time.Second)
	minutes := total / 60
	seconds := total % 60
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

func saveRunIfNeeded(opts Options, rep report.Report, recorder *Recorder, code int) {
	if opts.NoSave || rep.StartedAt.IsZero() {
		return
	}
	dir := ResolveResultsDir(opts)
	store, err := results.NewStore(dir)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "aviso: no se pudieron guardar los resultados: %v\n", err)
		return
	}
	store.Warn = func(msg string) { fmt.Fprintf(opts.Stderr, "aviso: %s\n", msg) }
	source := ""
	if opts.ConfigPath != "" {
		source = filepath.Base(opts.ConfigPath)
	}
	record := results.FromReport(rep, results.Meta{
		Tag:        opts.Tag,
		Source:     source,
		ConfigHash: configHash(opts.ConfigPath),
		Version:    opts.Version,
		Commit:     opts.Commit,
		ExitCode:   code,
	})
	saved, err := store.Save(record, recorder.Points(), report.Text(rep))
	if err != nil {
		fmt.Fprintf(opts.Stderr, "aviso: no se pudieron guardar los resultados: %v\n", err)
		return
	}
	fmt.Fprintf(opts.Stderr, "resultados guardados en %s\n", filepath.Join(dir, saved.RunID))
}

// ResolveResultsDir returns the effective results directory.
func ResolveResultsDir(opts Options) string {
	if opts.ResultsDir != "" {
		return opts.ResultsDir
	}
	if dir := os.Getenv("SIAFU_RESULTS_DIR"); dir != "" {
		return dir
	}
	return ".siafu/runs"
}

func configHash(path string) string {
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
