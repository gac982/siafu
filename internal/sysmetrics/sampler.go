// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

// Package sysmetrics samples the machine that runs siafu, not the target.
package sysmetrics

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	gnet "github.com/shirou/gopsutil/v4/net"
)

// Snapshot is one host-metrics sample. Err is non-nil when a metric could
// not be read; the other fields remain usable.
type Snapshot struct {
	At              time.Time
	CPUPercent      float64
	MemPercent      float64
	DiskUsedPercent float64
	MemUsed         uint64
	MemTotal        uint64
	DiskFree        uint64
	NetRxBps        float64
	NetTxBps        float64
	Err             error
}

// Sampler periodically samples CPU, memory, disk and network.
type Sampler struct {
	interval time.Duration
}

// NewSampler creates a sampler; a non-positive interval means one second.
func NewSampler(interval time.Duration) *Sampler {
	if interval <= 0 {
		interval = time.Second
	}
	return &Sampler{interval: interval}
}

// Run emits snapshots until ctx is canceled. Partial metric failures do not
// stop the stream.
func (s *Sampler) Run(ctx context.Context) <-chan Snapshot {
	out := make(chan Snapshot, 1)
	go func() {
		defer close(out)

		// Prime the CPU delta and network counters.
		_, _ = cpu.Percent(0, false)
		prevRx, prevTx, _ := networkTotals()
		prevAt := time.Now()

		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				snap := Snapshot{At: now}
				var problems []string

				cpuPercent, err := cpu.Percent(0, false)
				if err != nil {
					problems = append(problems, "cpu: "+err.Error())
				} else if len(cpuPercent) > 0 {
					snap.CPUPercent = cpuPercent[0]
				}

				if vm, err := mem.VirtualMemory(); err != nil {
					problems = append(problems, "mem: "+err.Error())
				} else {
					snap.MemUsed = vm.Used
					snap.MemTotal = vm.Total
					snap.MemPercent = vm.UsedPercent
				}

				if usage, err := disk.Usage("/"); err != nil {
					problems = append(problems, "disk: "+err.Error())
				} else {
					snap.DiskUsedPercent = usage.UsedPercent
					snap.DiskFree = usage.Free
				}

				rx, tx, err := networkTotals()
				if err != nil {
					problems = append(problems, "net: "+err.Error())
				} else {
					elapsed := now.Sub(prevAt).Seconds()
					if elapsed > 0 && prevRx > 0 && rx >= prevRx {
						snap.NetRxBps = float64(rx-prevRx) / elapsed
					}
					if elapsed > 0 && prevTx > 0 && tx >= prevTx {
						snap.NetTxBps = float64(tx-prevTx) / elapsed
					}
					prevRx, prevTx, prevAt = rx, tx, now
				}

				if len(problems) > 0 {
					snap.Err = errors.New(strings.Join(problems, "; "))
				}
				select {
				case out <- snap:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out
}

func networkTotals() (rx, tx uint64, err error) {
	counters, err := gnet.IOCounters(true)
	if err != nil {
		return 0, 0, err
	}
	for _, counter := range counters {
		if strings.HasPrefix(counter.Name, "lo") {
			continue
		}
		rx += counter.BytesRecv
		tx += counter.BytesSent
	}
	if rx == 0 && tx == 0 {
		return 0, 0, fmt.Errorf("no hay interfaces de red utilizables")
	}
	return rx, tx, nil
}
