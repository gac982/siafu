// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package sysmetrics

// Summary aggregates host metrics for the final report.
type Summary struct {
	CPUAvg        float64
	CPUMax        float64
	MemMaxPercent float64
	NetRxPeak     float64
	NetTxPeak     float64
	Samples       int
}

// Accumulator folds snapshots into a Summary.
type Accumulator struct {
	cpuSum   float64
	cpuMax   float64
	memMax   float64
	netRxMax float64
	netTxMax float64
	samples  int
}

// Add incorporates one snapshot. Error-only snapshots still count if they
// carry usable values.
func (a *Accumulator) Add(snapshot Snapshot) {
	a.samples++
	a.cpuSum += snapshot.CPUPercent
	if snapshot.CPUPercent > a.cpuMax {
		a.cpuMax = snapshot.CPUPercent
	}
	if snapshot.MemPercent > a.memMax {
		a.memMax = snapshot.MemPercent
	}
	if snapshot.NetRxBps > a.netRxMax {
		a.netRxMax = snapshot.NetRxBps
	}
	if snapshot.NetTxBps > a.netTxMax {
		a.netTxMax = snapshot.NetTxBps
	}
}

// Summary returns the accumulated values.
func (a *Accumulator) Summary() Summary {
	summary := Summary{
		CPUMax:        a.cpuMax,
		MemMaxPercent: a.memMax,
		NetRxPeak:     a.netRxMax,
		NetTxPeak:     a.netTxMax,
		Samples:       a.samples,
	}
	if a.samples > 0 {
		summary.CPUAvg = a.cpuSum / float64(a.samples)
	}
	return summary
}
