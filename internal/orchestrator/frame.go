// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package orchestrator

import (
	"github.com/gac982/siafu/internal/config"
	"github.com/gac982/siafu/internal/engine"
	"github.com/gac982/siafu/internal/logbuf"
	"github.com/gac982/siafu/internal/metrics"
	"github.com/gac982/siafu/internal/redact"
	"github.com/gac982/siafu/internal/sysmetrics"
	"github.com/gac982/siafu/internal/tui"
)

func frameFrom(cfg *config.Config, latest metrics.Snapshot, host sysmetrics.Snapshot, logs *logbuf.Buffer, status engine.Status) tui.Frame {
	state := tui.StateRunning
	if status.Finishing {
		state = tui.StateFinishing
	}
	return tui.Frame{
		Name:         cfg.Name,
		Target:       redact.New(nil).URL(cfg.Target.BaseURL),
		Elapsed:      status.Elapsed,
		Planned:      status.Planned,
		Stage:        status.Stage,
		Stages:       status.Stages,
		VUsActive:    status.ActiveVUs,
		VUsTarget:    status.TargetVUs,
		VUsPeak:      status.PeakVUs,
		Mode:         status.Mode,
		TargetRate:   status.TargetRate,
		AchievedRate: status.AchievedRate,
		InFlight:     status.InFlight,
		MaxInFlight:  status.MaxInFlight,
		Dropped:      status.Dropped,
		Metrics:      latest,
		Host:         host,
		Logs:         logs.Lines(),
		State:        state,
	}
}
