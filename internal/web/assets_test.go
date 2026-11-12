// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package web

import (
	"io/fs"
	"strings"
	"testing"
)

func readAsset(t *testing.T, fsys fs.FS, name string) string {
	t.Helper()
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data)
}

func TestAssetsContainShell(t *testing.T) {
	fsys := AssetsFS()

	index := readAsset(t, fsys, "index.html")
	for _, want := range []string{`id="app"`, `type="module"`, "app.js", "style.css", "/logo.png", "/mark.png", "/favicon.png"} {
		if !strings.Contains(index, want) {
			t.Errorf("index.html missing %q", want)
		}
	}

	app := readAsset(t, fsys, "app.js")
	for _, want := range []string{"/logo.png", "#/runs/", "#/compare", "lineChart", "renderDetail", "renderCompare", "comparisonTable", "Comparar…", "No evaluados", "NO EVALUADO", "refresh-runs", "Refrescar", "threshold.evaluated", "thresholdFor", "protocolSummary"} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js missing %q", want)
		}
	}

	charts := readAsset(t, fsys, "charts.js")
	if !strings.Contains(charts, "export function lineChart") {
		t.Errorf("charts.js missing lineChart export")
	}
	if !strings.Contains(charts, "downsample") {
		t.Errorf("charts.js missing downsampling")
	}
	if !strings.Contains(charts, `"desc"`) {
		t.Errorf("charts.js missing desc element")
	}

	for _, name := range []string{"logo.png", "mark.png", "favicon.png"} {
		if len(readAsset(t, fsys, name)) == 0 {
			t.Errorf("%s is empty", name)
		}
	}

	format := readAsset(t, fsys, "format.js")
	for _, want := range []string{"formatDuration", "formatPercent", "escapeHTML"} {
		if !strings.Contains(format, want) {
			t.Errorf("format.js missing %q", want)
		}
	}
}
