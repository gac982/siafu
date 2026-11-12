// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package report

import (
	"fmt"
	"strings"

	"github.com/gac982/siafu/internal/httpx"
)

// Diagnose applies the deterministic rule set to a report.
func Diagnose(r Report) []Finding {
	var findings []Finding
	add := func(severity Severity, format string, args ...any) {
		findings = append(findings, Finding{Severity: severity, Message: fmt.Sprintf(format, args...)})
	}

	if r.MemoryExceeded {
		if r.OpenLoop != nil {
			add(SeverityCritical, "Se alcanzó el límite de memoria del generador; la prueba se detuvo para evitar un OOM. Reducir max_in_flight o target_rate, o aumentar limits.memory_budget_mb.")
		} else {
			add(SeverityCritical, "Se alcanzó el límite de memoria del generador; la prueba se detuvo para evitar un OOM. Reducir VUs o aumentar limits.memory_budget_mb.")
		}
	}
	if r.Global.AnalysisSkipped > 0 {
		add(SeverityWarning, "Análisis de cuerpo omitido en %d pasos por presupuesto agotado; las comprobaciones body_contains/extract no se evaluaron en esos pasos.", r.Global.AnalysisSkipped)
	}
	if r.OpenLoop != nil && r.OpenLoop.Dropped > 0 {
		add(SeverityWarning, "Saturación del generador: %d iteraciones descartadas; la tasa objetivo no se sostuvo. Reducir target_rate o aumentar max_in_flight.", r.OpenLoop.Dropped)
	}
	for _, errCount := range r.Global.Errors {
		if errCount.Kind == string(httpx.ErrAnalysisBudget) {
			add(SeverityWarning, "Presupuesto de análisis agotado en %d pasos; reduce el tamaño de las respuestas, aumenta limits.analysis_budget_mb o usa on_analysis_exhausted: skip.", errCount.Count)
		}
	}
	if r.Global.Total == 0 {
		add(SeverityCritical, "No se completó ninguna petición. Revisar la URL, la conectividad y los logs.")
		markPartial(findings, r.Aborted)
		return findings
	}

	errorCounts := map[string]int64{}
	for _, errCount := range r.Global.Errors {
		errorCounts[errCount.Kind] = errCount.Count
	}

	if errorCounts["template"] > 0 {
		add(SeverityCritical, "Hay variables de entorno o referencias sin resolver (%d). Revisar ${ENV:...} y los logs.", errorCounts["template"])
	}

	transport := errorCounts["dns"] + errorCounts["dial"] + errorCounts["tls"] +
		errorCounts["timeout"] + errorCounts["read"] + errorCounts["redirect"]
	if transport > 0 && (transport*10 >= r.Global.Total || r.Global.Total < 10) {
		add(SeverityCritical,
			"El %.1f%% de las peticiones falló por transporte (dns %d, dial %d, tls %d, timeout %d, read %d, redirect %d). Revisar DNS, TLS, firewall o timeouts.",
			float64(transport)/float64(r.Global.Total)*100,
			errorCounts["dns"], errorCounts["dial"], errorCounts["tls"],
			errorCounts["timeout"], errorCounts["read"], errorCounts["redirect"])
	}

	var statusFailures, s4xx, s5xx, s429 int64
	for code, count := range r.Global.FailedStatusCodes {
		switch {
		case code == 429:
			s429 += count
			s4xx += count
			statusFailures += count
		case code >= 500 && code < 600:
			s5xx += count
			statusFailures += count
		case code >= 400 && code < 500:
			s4xx += count
			statusFailures += count
		}
	}

	switch {
	case statusFailures > 0 && s429*2 >= statusFailures:
		add(SeverityWarning, "El destino aplicó rate limiting (429): %d de %d fallos con estado. Reducir VUs o ajustar los límites.", s429, statusFailures)
	case statusFailures > 0 && s5xx*2 >= statusFailures:
		add(SeverityCritical, "Errores 5xx dominantes (%d de %d fallos con estado). Posible saturación del servidor.", s5xx, statusFailures)
	case statusFailures > 0 && (s4xx-s429)*2 >= statusFailures:
		add(SeverityWarning, "Errores 4xx dominantes (%d de %d fallos). Revisar expect.status, los datos de entrada o la autenticación.", s4xx-s429, statusFailures)
	}

	p95, hasP95 := thresholdByName(r.Thresholds, "latencia p95")
	errorRate, hasErrorRate := thresholdByName(r.Thresholds, "tasa de error")
	minRPS, hasMinRPS := thresholdByName(r.Thresholds, "RPS promedio")

	if !r.Aborted {
		trendDown := hasP95 && !p95.Pass && r.Global.RPSPeak > 0 &&
			r.Global.RPSRecent < 0.8*r.Global.RPSPeak &&
			r.Actual > 0 && r.Global.RPSPeakAt < r.Actual*8/10
		switch {
		case trendDown:
			add(SeverityCritical,
				"Throughput decreciente con latencia alta: el pico de RPS fue %.1f y el reciente es %.1f. Señal de saturación del destino.",
				r.Global.RPSPeak, r.Global.RPSRecent)
		case hasP95 && !p95.Pass:
			add(SeverityWarning, "Latencia alta: p95 %s supera el umbral. Sin errores elevados; revisar red o procesamiento.", p95.Observed)
		}

		if hasMinRPS && !minRPS.Pass && (!hasErrorRate || errorRate.Pass) && (!hasP95 || p95.Pass) {
			if r.OpenLoop != nil {
				add(SeverityWarning, "No se alcanzó el RPS objetivo. Revisar target_rate, max_in_flight o red.")
			} else {
				add(SeverityWarning, "No se alcanzó el RPS objetivo. La prueba no generó suficiente carga: revisar VUs, think time o red.")
			}
		}

		if hasErrorRate && !errorRate.Pass && statusFailures == 0 && transport == 0 && errorCounts["template"] == 0 {
			add(SeverityWarning, "La tasa de error %s supera el umbral; revisar los checks fallidos en el detalle.", errorRate.Observed)
		}
	}

	if r.Host.CPUAvg >= 85 || r.Host.CPUMax >= 95 {
		add(SeverityWarning, "El generador estuvo cerca del límite de CPU (promedio %.1f%%, máximo %.1f%%). Los resultados pueden subestimar al destino.",
			r.Host.CPUAvg, r.Host.CPUMax)
	}

	if errorCounts["body_too_large"] > 0 {
		add(SeverityWarning, "Hay respuestas que superaron el límite de lectura (%d). Los resultados de esas peticiones están truncados.", errorCounts["body_too_large"])
	}

	if !r.Aborted && len(r.Thresholds) > 0 {
		allPass := true
		for _, threshold := range r.Thresholds {
			if !threshold.Pass {
				allPass = false
				break
			}
		}
		if allPass {
			add(SeverityInfo, "Todos los umbrales se cumplen.")
		}
	}

	if r.Aborted {
		if len(findings) == 0 {
			add(SeverityInfo, "Ejecución detenida antes de terminar; diagnóstico parcial.")
		}
		markPartial(findings, true)
	}

	return findings
}

func markPartial(findings []Finding, partial bool) {
	if !partial {
		return
	}
	for i := range findings {
		findings[i].Partial = true
	}
}

func thresholdByName(thresholds []ThresholdResult, name string) (ThresholdResult, bool) {
	for _, threshold := range thresholds {
		if strings.EqualFold(threshold.Name, name) {
			return threshold, true
		}
	}
	return ThresholdResult{}, false
}
