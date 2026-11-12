// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package tui

import (
	"strings"

	"github.com/gac982/siafu/internal/report"

	"github.com/gac982/siafu/internal/safetext"
)

func (m *Model) applyReport(rep report.Report) {
	m.report = &rep

	lines := strings.Split(report.Text(rep), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "Veredicto:") {
			switch rep.Verdict {
			case report.VerdictPass, report.VerdictNotEvaluated:
				lines[i] = titleStyle.Render(line)
			default:
				lines[i] = accentStyle.Render(line)
			}
		}
	}
	m.reportViewport.SetContent(strings.Join(lines, "\n"))
	m.reportViewport.GotoTop()
	m.resizeViewport()
}

func (m Model) reportView(width, height int) string {
	header := titleStyle.Render(truncate("REPORTE · "+safetext.Line(m.report.Name), width))
	footer := secondaryStyle.Render(truncate("Q/ESC salir   X logs   ↑↓ desplazar   ? ayuda", width))
	panelHeight := max(3, height-2)
	lines := strings.Split(m.reportViewport.View(), "\n")
	return strings.Join([]string{
		header,
		renderPanel("RESULTADO", lines, width, panelHeight),
		footer,
	}, "\n")
}
