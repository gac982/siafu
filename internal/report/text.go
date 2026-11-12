// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package report

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/gac982/siafu/internal/metrics"

	"github.com/gac982/siafu/internal/safetext"
)

// Text renders the report as plain text without ANSI escapes.
func Text(r Report) string {
	var b strings.Builder

	fmt.Fprintf(&b, "SIAFU · %s\n", safetext.Line(r.Name))
	fmt.Fprintf(&b, "Destino: %s\n", safetext.Line(r.Target))
	fmt.Fprintf(&b, "Inicio: %s\n", r.StartedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "Duración: planificada %s / real %s\n", formatDuration(r.Planned), formatDuration(r.Actual))
	mode := "cerrado"
	unit := "VUs"
	if r.OpenLoop != nil {
		mode = "abierto"
		unit = "in-flight"
	}
	fmt.Fprintf(&b, "Modo: %s · pico %d %s · Semilla: %d\n", mode, r.PeakVUs, unit, r.Seed)
	fmt.Fprintf(&b, "Veredicto: %s\n", r.Verdict.Label())
	if r.Aborted {
		fmt.Fprintln(&b, "Nota: ejecución detenida antes de terminar; las métricas son parciales y los umbrales no se evaluaron.")
	}
	fmt.Fprintln(&b)

	fmt.Fprintln(&b, "RESUMEN GLOBAL")
	fmt.Fprintf(&b, "  Peticiones:      %d\n", r.Global.Total)
	fmt.Fprintf(&b, "  Fallidas:        %d (%.2f%%)\n", r.Global.Failed, r.Global.ErrorRate*100)
	fmt.Fprintf(&b, "  Iteraciones:     %d (fallidas %d)\n", r.Global.Iterations, r.Global.FailedIterations)
	fmt.Fprintf(&b, "  RPS:             promedio %.1f · pico %.1f\n", r.Global.RPSAvg, r.Global.RPSPeak)
	fmt.Fprintf(&b, "  Protocolos:      %s\n", formatProtocols(r.Global.Protocols))
	fmt.Fprintf(&b, "  Latencia:        p50 %s · p90 %s · p95 %s · p99 %s\n",
		formatDuration(r.Global.Latency.P50), formatDuration(r.Global.Latency.P90),
		formatDuration(r.Global.Latency.P95), formatDuration(r.Global.Latency.P99))
	fmt.Fprintf(&b, "  Latencia:        mín %s · máx %s · media %s\n",
		formatDuration(r.Global.Latency.Min), formatDuration(r.Global.Latency.Max), formatDuration(r.Global.Latency.Mean))
	fmt.Fprintf(&b, "  Checks:          %d/%d (%.2f%%)\n", r.Global.ChecksPassed, r.Global.ChecksTotal, checkRate(r.Global))
	fmt.Fprintf(&b, "  Bytes leídos:    %s\n", formatBytes(uint64(r.Global.TransferredBytes)))
	fmt.Fprintf(&b, "  Canceladas:      %d\n", r.Global.Canceled)
	if r.Global.AnalysisSkipped > 0 {
		fmt.Fprintf(&b, "  Análisis omitido: %d pasos\n", r.Global.AnalysisSkipped)
	}
	fmt.Fprintln(&b)

	if r.OpenLoop != nil {
		renderOpenLoop(&b, r.OpenLoop)
	}

	fmt.Fprintln(&b, "POR ESCENARIO")
	if len(r.Scenarios) == 0 {
		fmt.Fprintln(&b, "  sin datos")
	}
	for _, scenario := range r.Scenarios {
		fmt.Fprintf(&b, "  %s\n", safetext.Line(scenario.Name))
		fmt.Fprintf(&b, "    peticiones %d · fallidas %d (%.2f%%) · iteraciones %d\n",
			scenario.Total, scenario.Failed, scenario.ErrorRate*100, scenario.Iterations)
		fmt.Fprintf(&b, "    rps promedio %.1f · pico %.1f · p95 %s · p99 %s\n",
			scenario.RPSAvg, scenario.RPSPeak,
			formatDuration(scenario.Latency.P95), formatDuration(scenario.Latency.P99))
	}
	fmt.Fprintln(&b)

	fmt.Fprintln(&b, "CÓDIGOS DE ESTADO")
	codes := make([]int, 0, len(r.Global.StatusCodes))
	for code := range r.Global.StatusCodes {
		codes = append(codes, code)
	}
	sort.Ints(codes)
	if len(codes) == 0 {
		fmt.Fprintln(&b, "  sin datos")
	}
	for _, code := range codes {
		fmt.Fprintf(&b, "  %d: %d\n", code, r.Global.StatusCodes[code])
	}
	fmt.Fprintln(&b)

	fmt.Fprintln(&b, "ERRORES Y CHECKS")
	if len(r.Global.Errors) == 0 && len(r.Global.CheckFailures) == 0 {
		fmt.Fprintln(&b, "  sin errores registrados")
	}
	for _, errCount := range r.Global.Errors {
		fmt.Fprintf(&b, "  transporte · %s: %d\n", errCount.Kind, errCount.Count)
	}
	for i, failure := range r.Global.CheckFailures {
		if i >= 10 {
			break
		}
		fmt.Fprintf(&b, "  check · %s/%s · %s: %d\n", safetext.Line(failure.Scenario), safetext.Line(failure.Step), safetext.Line(failure.Reason), failure.Count)
	}
	fmt.Fprintln(&b)

	fmt.Fprintln(&b, "UMBRALES")
	if len(r.Thresholds) == 0 {
		if r.Aborted {
			fmt.Fprintln(&b, "  no evaluados (ejecución abortada)")
		} else {
			fmt.Fprintln(&b, "  sin umbrales definidos")
		}
	}
	for _, threshold := range r.Thresholds {
		mark := "✓"
		if !threshold.Pass {
			mark = "✗"
		}
		fmt.Fprintf(&b, "  %s %s: %s (esperado %s)\n", mark, threshold.Name, threshold.Observed, threshold.Expected)
	}
	fmt.Fprintln(&b)

	if len(r.Diagnosis) > 0 {
		fmt.Fprintln(&b, "DIAGNÓSTICO")
		for _, finding := range r.Diagnosis {
			prefix := "·"
			switch finding.Severity {
			case SeverityWarning:
				prefix = "!"
			case SeverityCritical:
				prefix = "✗"
			}
			fmt.Fprintf(&b, "  %s %s\n", prefix, safetext.Line(finding.Message))
		}
		fmt.Fprintln(&b)
	}

	fmt.Fprintln(&b, "GENERADOR")
	fmt.Fprintf(&b, "  CPU:  promedio %.1f%% · máximo %.1f%%\n", r.Host.CPUAvg, r.Host.CPUMax)
	fmt.Fprintf(&b, "  RAM:  máximo %.1f%%\n", r.Host.MemMaxPercent)
	fmt.Fprintf(&b, "  RED:  pico ↓ %s · ↑ %s\n", formatRate(r.Host.NetRxPeak), formatRate(r.Host.NetTxPeak))
	fmt.Fprintln(&b, "  (métricas del equipo que ejecutó siafu, no del destino)")

	return b.String()
}

// WriteText writes the plain-text report.
func WriteText(w io.Writer, r Report) error {
	_, err := io.WriteString(w, Text(r))
	return err
}

func checkRate(snapshot metrics.Snapshot) float64 {
	if snapshot.ChecksTotal == 0 {
		return 100
	}
	return float64(snapshot.ChecksPassed) / float64(snapshot.ChecksTotal) * 100
}

func formatProtocols(protocols map[string]int64) string {
	if len(protocols) == 0 {
		return "sin datos"
	}
	names := make([]string, 0, len(protocols))
	for name := range protocols {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, fmt.Sprintf("%s: %d", name, protocols[name]))
	}
	return strings.Join(parts, " · ")
}

func formatBytes(value uint64) string {
	const unit = 1024
	if value < unit {
		return fmt.Sprintf("%d B", value)
	}
	div, exp := uint64(unit), 0
	for n := value / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	if exp >= len("KMGTPE") {
		exp = len("KMGTPE") - 1
	}
	return fmt.Sprintf("%.1f %cB", float64(value)/float64(div), "KMGTPE"[exp])
}

func formatRate(value float64) string {
	return formatBytes(uint64(value)) + "/s"
}

func renderOpenLoop(b *strings.Builder, o *OpenLoopSummary) {
	peakTarget := o.TargetRate
	for _, stage := range o.Stages {
		if stage.TargetRate > peakTarget {
			peakTarget = stage.TargetRate
		}
	}
	fmt.Fprintln(b, "MODO OPEN LOOP")
	peakAchieved := 0.0
	for _, stage := range o.Stages {
		if stage.TargetRate == peakTarget && stage.Duration > 0 {
			achieved := float64(stage.Iterations) / stage.Duration.Seconds()
			if achieved > peakAchieved {
				peakAchieved = achieved
			}
		}
	}
	fmt.Fprintf(b, "  Tasa objetivo:   %.1f iteraciones/s (pico)\n", peakTarget)
	fmt.Fprintf(b, "  Tasa alcanzada:  %.1f iteraciones/s (media) · %.1f peticiones/s\n", o.AchievedRate, o.AchievedRPS)
	if peakAchieved > 0 {
		fmt.Fprintf(b, "  Tasa media objetivo: %.1f iteraciones/s (objetivo %.1f/s)\n", peakAchieved, peakTarget)
	}
	fmt.Fprintf(b, "  In-flight pico:  %d\n", o.PeakInFlight)
	fmt.Fprintf(b, "  Descartes:       %d\n", o.Dropped)
	if failure := openLoopFailurePoint(o.Stages); failure != nil {
		fmt.Fprintf(b, "  Punto de fallo:  etapa %d (objetivo %.1f/s) · %d fallidas · %d descartes\n",
			failure.Index+1, failure.TargetRate, failure.Failed, failure.Dropped)
	} else {
		fmt.Fprintln(b, "  Punto de fallo:  sin fallo registrado")
	}
	if len(o.Stages) > 0 {
		fmt.Fprintln(b, "  ETAPAS")
		for _, stage := range o.Stages {
			achieved := 0.0
			if stage.Duration > 0 {
				achieved = float64(stage.Iterations) / stage.Duration.Seconds()
			}
			fmt.Fprintf(b, "    etapa %d · objetivo %.1f/s · alcanzado %.1f/s · peticiones %d · fallidas %d · descartes %d\n",
				stage.Index+1, stage.TargetRate, achieved, stage.Requests, stage.Failed, stage.Dropped)
		}
	}
	fmt.Fprintln(b)
}

const openLoopErrorThreshold = 0.01

func openLoopFailurePoint(stages []OpenLoopStage) *OpenLoopStage {
	for i := range stages {
		stage := &stages[i]
		if stage.Requests > 0 && float64(stage.Failed)/float64(stage.Requests) > openLoopErrorThreshold {
			return stage
		}
	}
	return nil
}
