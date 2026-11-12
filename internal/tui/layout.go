// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/gac982/siafu/internal/metrics"

	"github.com/gac982/siafu/internal/safetext"
)

func (m Model) dashboard(width, height int) string {
	frame := m.frame

	header := []string{
		spread("SIAFU · "+safetext.Line(frame.Name), "destino "+safetext.Line(frame.Target), width),
		m.statusLine(width),
	}

	hostCards := []card{
		{"CPU", fmt.Sprintf("%.0f%%", frame.Host.CPUPercent), "del generador"},
		{"RAM", fmt.Sprintf("%s / %s", formatBytes(frame.Host.MemUsed), formatBytes(frame.Host.MemTotal)), fmt.Sprintf("%.0f%% usado", frame.Host.MemPercent)},
		{"DISCO", fmt.Sprintf("%.0f%%", frame.Host.DiskUsedPercent), formatBytes(frame.Host.DiskFree) + " libres"},
		{"RED", "↓ " + formatRate(frame.Host.NetRxBps), "↑ " + formatRate(frame.Host.NetTxBps)},
	}
	loadCards := []card{
		{"VUs", fmt.Sprintf("%d/%d", frame.VUsActive, frame.VUsTarget), fmt.Sprintf("pico %d", frame.VUsPeak)},
		{"RPS", fmt.Sprintf("%.1f", frame.Metrics.RPSRecent), fmt.Sprintf("medio %.1f", frame.Metrics.RPSAvg)},
		{"Latencia", fmt.Sprintf("p95 %s", shortDuration(frame.Metrics.Latency.P95)), fmt.Sprintf("p99 %s", shortDuration(frame.Metrics.Latency.P99))},
		{"Errores", fmt.Sprintf("%.1f%%", frame.Metrics.ErrorRate*100), fmt.Sprintf("checks %.1f%%", checkRate(frame.Metrics))},
	}
	if frame.Mode == "open" {
		loadCards = []card{
			{"Target/s", fmt.Sprintf("%.0f", frame.TargetRate), fmt.Sprintf("in-flight %d/%d", frame.InFlight, frame.MaxInFlight)},
			{"Achieved/s", fmt.Sprintf("%.0f", frame.AchievedRate), fmt.Sprintf("RPS %.1f", frame.Metrics.RPSRecent)},
			{"Dropped", fmt.Sprintf("%d", frame.Dropped), "descartes del generador"},
			{"Latencia", fmt.Sprintf("p95 %s", shortDuration(frame.Metrics.Latency.P95)), fmt.Sprintf("p99 %s", shortDuration(frame.Metrics.Latency.P99))},
		}
	}

	hostGrid := renderCardGrid(hostCards, columnWidth(width))
	loadGrid := renderCardGrid(loadCards, columnWidth(width))
	var body string
	if width >= 56 {
		body = lipgloss.JoinHorizontal(lipgloss.Top, hostGrid, " ", loadGrid)
	} else {
		body = lipgloss.JoinVertical(lipgloss.Left, hostGrid, loadGrid)
	}

	totals := secondaryStyle.Render(fmt.Sprintf("Peticiones %d · Iteraciones %d · Fallidas %d", frame.Metrics.Total, frame.Metrics.Iterations, frame.Metrics.Failed))

	var parts []string
	parts = append(parts, header...)
	parts = append(parts, body, truncate(totals, width))

	_, _, trend, logPanel := dashboardHeights(width, height)
	if trend > 0 {
		parts = append(parts, m.trendLine(width))
	}
	if logPanel >= 3 {
		logLines := strings.Split(m.logViewport.View(), "\n")
		parts = append(parts, renderPanel("LOG", logLines, width, logPanel))
	}

	parts = append(parts, secondaryStyle.Render(truncate("Ctrl+D detener   X logs   ? ayuda   Ctrl+C salir   ↑↓ desplazar", width)))
	return strings.Join(parts, "\n")
}

func (m Model) statusLine(width int) string {
	frame := m.frame
	state := "EJECUTANDO"
	if frame.State == StateFinishing {
		state = "DETENIENDO"
	}
	fraction := 0.0
	if frame.Planned > 0 {
		fraction = float64(frame.Elapsed) / float64(frame.Planned)
	}
	var line string
	if frame.Mode == "open" {
		line = fmt.Sprintf("%s  %s / %s · etapa %d/%d · target %.0f/s · achieved %.0f/s · in-flight %d  %s",
			state,
			formatDuration(frame.Elapsed),
			formatDuration(frame.Planned),
			frame.Stage,
			frame.Stages,
			frame.TargetRate,
			frame.AchievedRate,
			frame.InFlight,
			progressBar(fraction, 10),
		)
	} else {
		line = fmt.Sprintf("%s  %s / %s · etapa %d/%d · VUs %d/%d  %s",
			state,
			formatDuration(frame.Elapsed),
			formatDuration(frame.Planned),
			frame.Stage,
			frame.Stages,
			frame.VUsActive,
			frame.VUsTarget,
			progressBar(fraction, 10),
		)
	}
	return truncate(line, width)
}

func (m Model) trendLine(width int) string {
	sparkWidth := max(8, (width-34)/2)
	rps := accentStyle.Render(RenderSparkline(m.rpsHistory, sparkWidth))
	p95 := accentStyle.Render(RenderSparkline(m.p95History, sparkWidth))
	line := titleStyle.Render("RPS ") + rps + " " + valueStyle.Render(fmt.Sprintf("%.1f", m.frame.Metrics.RPSRecent)) +
		"    " + titleStyle.Render("p95 ") + p95 + " " + valueStyle.Render(shortDuration(m.frame.Metrics.Latency.P95))
	return truncate(line, width)
}

func (m Model) logsView(width, height int) string {
	lines := strings.Split(m.logViewport.View(), "\n")
	panelHeight := max(3, height-2)
	return strings.Join([]string{
		titleStyle.Render(truncate("LOG · "+safetext.Line(m.frame.Name), width)),
		renderPanel("EVENTOS", lines, width, panelHeight),
		secondaryStyle.Render(truncate("X o ESC volver   ↑↓ desplazar   Ctrl+C salir", width)),
	}, "\n")
}

func helpView(width int) string {
	lines := []string{
		titleStyle.Render("SIAFU · AYUDA"),
		"",
		titleStyle.Render("Ctrl+D") + secondaryStyle.Render("  detener la prueba y mostrar el reporte"),
		titleStyle.Render("Ctrl+C") + secondaryStyle.Render("  salir inmediatamente"),
		titleStyle.Render("X") + secondaryStyle.Render("       ver los logs a pantalla completa"),
		titleStyle.Render("?") + secondaryStyle.Render("       mostrar u ocultar esta ayuda"),
		titleStyle.Render("↑/↓") + secondaryStyle.Render("     desplazar los logs"),
		titleStyle.Render("ESC") + secondaryStyle.Render("     cerrar la ayuda o los logs"),
	}
	output := make([]string, 0, len(lines))
	for _, line := range lines {
		output = append(output, truncate(line, width))
	}
	return strings.Join(output, "\n")
}

func dashboardHeights(width, height int) (columns, gridHeight, trend, logPanel int) {
	columns = columnWidth(width)
	gridHeight = 10
	if width < 56 {
		gridHeight = 20
	}
	if height >= 33 && width >= 80 {
		trend = 2
	}
	logPanel = height - 2 - gridHeight - 1 - trend - 1
	if logPanel < 0 {
		logPanel = 0
	}
	return columns, gridHeight, trend, logPanel
}

func columnWidth(width int) int {
	if width < 40 {
		return width
	}
	return (width - 1) / 2
}

func renderPanel(title string, content []string, width, height int) string {
	if width < 4 || height < 3 {
		return truncate(title, width)
	}
	innerWidth := width - 2
	innerHeight := height - 2

	content = append([]string{titleStyle.Render(title)}, content...)
	if len(content) > innerHeight {
		content = content[:innerHeight]
	}
	for len(content) < innerHeight {
		content = append(content, "")
	}

	top := borderStyle.Render("┌" + strings.Repeat("─", innerWidth) + "┐")
	bottom := borderStyle.Render("└" + strings.Repeat("─", innerWidth) + "┘")
	lines := make([]string, 0, height)
	lines = append(lines, top)
	for _, line := range content {
		lines = append(lines, borderStyle.Render("│")+padRight(truncate(line, innerWidth), innerWidth)+borderStyle.Render("│"))
	}
	lines = append(lines, bottom)
	return strings.Join(lines, "\n")
}

func spread(left, right string, width int) string {
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	if leftWidth+rightWidth+1 > width {
		return truncate(left, width)
	}
	return left + strings.Repeat(" ", width-leftWidth-rightWidth) + right
}

func fitView(view string, width, height int) string {
	lines := strings.Split(view, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for i, line := range lines {
		lines[i] = truncate(line, width)
	}
	return strings.Join(lines, "\n")
}

func shortDuration(d time.Duration) string {
	if d <= 0 {
		return "—"
	}
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

func checkRate(snapshot metrics.Snapshot) float64 {
	if snapshot.ChecksTotal == 0 {
		return 100
	}
	return float64(snapshot.ChecksPassed) / float64(snapshot.ChecksTotal) * 100
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
