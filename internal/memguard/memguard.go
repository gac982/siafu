// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

// Package memguard watches the process RSS and aborts a run before the OS
// OOM killer can take down the machine.
package memguard

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

// Logger is the subset of logging used by the watchdog.
type Logger interface {
	Warn(msg string)
	Error(msg string)
}

// Watcher samples RSS and enforces warning and hard limits.
type Watcher struct {
	budgetMB  int
	warnPct   int
	logger    Logger
	onHard    func()
	interval  time.Duration
	sample    func() (int, error)
	warned    bool
	hardCount int
}

// New creates a watcher. A nil onHard is allowed; a nil logger is ignored.
func New(budgetMB, warnPct int, logger Logger, onHard func()) *Watcher {
	if budgetMB <= 0 {
		budgetMB = 450
	}
	if warnPct <= 0 || warnPct >= 100 {
		warnPct = 80
	}
	return &Watcher{
		budgetMB: budgetMB,
		warnPct:  warnPct,
		logger:   logger,
		onHard:   onHard,
		interval: time.Second,
		sample:   SampleRSSMB,
	}
}

// Check evaluates one RSS reading. It reports whether the warning threshold
// was crossed for the first time and whether the hard limit was reached twice
// consecutively.
func (w *Watcher) Check(rssMB int) (warn, hard bool) {
	warnAt := w.budgetMB * w.warnPct / 100
	if !w.warned && rssMB >= warnAt {
		w.warned = true
		warn = true
	}
	if rssMB > w.budgetMB {
		w.hardCount++
	} else {
		w.hardCount = 0
	}
	if w.hardCount >= 2 {
		w.hardCount = 0
		hard = true
	}
	return warn, hard
}

// Run samples until ctx is done or the hard limit is reached.
func (w *Watcher) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rss, err := w.sample()
			if err != nil {
				if w.logger != nil {
					w.logger.Warn(fmt.Sprintf("no se pudo leer el RSS: %v", err))
				}
				continue
			}
			warn, hard := w.Check(rss)
			if warn && w.logger != nil {
				w.logger.Warn(fmt.Sprintf("RSS %d MiB supera el umbral de aviso (%d%% de %d MiB)", rss, w.warnPct, w.budgetMB))
			}
			if hard {
				if w.logger != nil {
					w.logger.Error(fmt.Sprintf("límite de memoria alcanzado: RSS %d MiB > %d MiB; deteniendo la prueba", rss, w.budgetMB))
				}
				if w.onHard != nil {
					w.onHard()
				}
				return
			}
		}
	}
}

// SampleRSSMB returns the current process resident set size in MiB.
func SampleRSSMB() (int, error) {
	proc, err := process.NewProcess(int32(os.Getpid()))
	if err != nil {
		return 0, err
	}
	info, err := proc.MemoryInfo()
	if err != nil {
		return 0, err
	}
	return int(info.RSS / 1024 / 1024), nil
}
