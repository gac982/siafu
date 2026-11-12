// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package cli

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gac982/siafu/internal/results"
)

func seedRuns(t *testing.T) (string, []string) {
	t.Helper()
	dir := t.TempDir()
	store, err := results.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	store.Rand = rand.Reader

	base := time.Date(2026, 11, 9, 9, 0, 0, 0, time.UTC)
	names := []string{"alpha", "beta"}
	ids := make([]string, 0, len(names))
	for i, name := range names {
		rec, err := store.Save(results.RunRecord{
			Name:      name,
			Mode:      "closed",
			Verdict:   "PASS",
			CreatedAt: base.Add(time.Duration(i) * time.Minute),
			Global: results.Global{
				Requests:  int64(100 + i*100),
				Failed:    1,
				ErrorRate: 0.01,
				P95MS:     float64(40 + i*10),
				RPSAvg:    float64(100 + i*10),
			},
		}, []results.SeriesPoint{{TMS: 1000, RPS: 100}}, "summary "+name)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, rec.RunID)
	}
	return dir, ids
}

func runCLI(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestRunsListText(t *testing.T) {
	dir, ids := seedRuns(t)
	code, stdout, stderr := runCLI(t, "runs", "list", "--results-dir", dir)
	if code != 0 {
		t.Fatalf("exit = %d (stderr: %s)", code, stderr)
	}
	for _, id := range ids {
		if !strings.Contains(stdout, id) {
			t.Errorf("stdout missing %q:\n%s", id, stdout)
		}
	}
	if !strings.Contains(stdout, "PASS") || !strings.Contains(stdout, "alpha") || !strings.Contains(stdout, "beta") {
		t.Errorf("stdout = %q", stdout)
	}
}

func TestRunsListJSON(t *testing.T) {
	dir, ids := seedRuns(t)
	code, stdout, stderr := runCLI(t, "runs", "list", "--json", "--results-dir", dir)
	if code != 0 {
		t.Fatalf("exit = %d (stderr: %s)", code, stderr)
	}
	var entries []results.IndexEntry
	if err := json.Unmarshal([]byte(stdout), &entries); err != nil {
		t.Fatalf("json: %v\n%s", err, stdout)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(entries))
	}
	if entries[0].RunID != ids[1] || entries[1].RunID != ids[0] {
		t.Errorf("order = %+v, want newest first", entries)
	}
}

func TestRunsListFilters(t *testing.T) {
	dir, ids := seedRuns(t)

	code, stdout, _ := runCLI(t, "runs", "list", "--json", "--q", "alpha", "--results-dir", dir)
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	var entries []results.IndexEntry
	if err := json.Unmarshal([]byte(stdout), &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].RunID != ids[0] {
		t.Fatalf("filtered = %+v", entries)
	}

	code, stdout, _ = runCLI(t, "runs", "list", "--json", "--limit", "1", "--results-dir", dir)
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if err := json.Unmarshal([]byte(stdout), &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].RunID != ids[1] {
		t.Fatalf("limited = %+v", entries)
	}
}

func TestRunsShow(t *testing.T) {
	dir, ids := seedRuns(t)
	code, stdout, stderr := runCLI(t, "runs", "show", ids[0], "--results-dir", dir)
	if code != 0 {
		t.Fatalf("exit = %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "summary alpha") {
		t.Errorf("stdout = %q", stdout)
	}

	code, stdout, _ = runCLI(t, "runs", "show", ids[0], "--json", "--results-dir", dir)
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	var rec results.RunRecord
	if err := json.Unmarshal([]byte(stdout), &rec); err != nil {
		t.Fatal(err)
	}
	if rec.Name != "alpha" || rec.RunID != ids[0] {
		t.Errorf("record = %+v", rec)
	}
}

func TestRunsShowUnknown(t *testing.T) {
	dir, _ := seedRuns(t)
	code, _, stderr := runCLI(t, "runs", "show", "no-existe", "--results-dir", dir)
	if code != 2 || !strings.Contains(stderr, "no existe") {
		t.Fatalf("exit/stderr = %d/%q", code, stderr)
	}
}

func TestRunsCompare(t *testing.T) {
	dir, ids := seedRuns(t)
	code, stdout, stderr := runCLI(t, "runs", "compare", ids[0], ids[1], "--results-dir", dir)
	if code != 0 {
		t.Fatalf("exit = %d (stderr: %s)", code, stderr)
	}
	for _, want := range []string{"p95", "RPS", "error rate", "requests"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}

	code, stdout, _ = runCLI(t, "runs", "compare", ids[0], ids[1], "--json", "--results-dir", dir)
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	var cmp comparison
	if err := json.Unmarshal([]byte(stdout), &cmp); err != nil {
		t.Fatal(err)
	}
	if cmp.Delta.P95MS != 10 || cmp.Delta.Requests != 100 {
		t.Errorf("delta = %+v", cmp.Delta)
	}
	if cmp.A.RunID != ids[0] || cmp.B.RunID != ids[1] {
		t.Errorf("comparison = %+v", cmp)
	}
}

func TestRunsPrune(t *testing.T) {
	dir, ids := seedRuns(t)

	code, stdout, stderr := runCLI(t, "runs", "prune", "--keep", "1", "--dry-run", "--results-dir", dir)
	if code != 0 {
		t.Fatalf("exit = %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "se eliminaría") || !strings.Contains(stdout, ids[0]) {
		t.Errorf("dry run stdout = %q", stdout)
	}
	store, _ := results.NewStore(dir)
	if list, _ := store.List(); len(list) != 2 {
		t.Fatalf("dry run removed runs: %+v", list)
	}

	code, stdout, _ = runCLI(t, "runs", "prune", "--keep", "1", "--results-dir", dir)
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(stdout, "eliminada") {
		t.Errorf("stdout = %q", stdout)
	}
	list, _ := store.List()
	if len(list) != 1 || list[0].RunID != ids[1] {
		t.Fatalf("list after prune = %+v", list)
	}
}

func TestRunsReindex(t *testing.T) {
	dir, _ := seedRuns(t)
	if err := os.Remove(filepath.Join(dir, "index.jsonl")); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runCLI(t, "runs", "reindex", "--results-dir", dir)
	if code != 0 {
		t.Fatalf("exit = %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "reconstruido") {
		t.Errorf("stdout = %q", stdout)
	}
	if _, err := os.Stat(filepath.Join(dir, "index.jsonl")); err != nil {
		t.Fatalf("index missing: %v", err)
	}
}

func TestRunsUnknownSubcommand(t *testing.T) {
	code, _, stderr := runCLI(t, "runs", "nope")
	if code != 2 || !strings.Contains(stderr, "uso: siafu runs") {
		t.Fatalf("exit/stderr = %d/%q", code, stderr)
	}
}

func TestRunsListNegativeLimit(t *testing.T) {
	dir, _ := seedRuns(t)
	code, _, stderr := runCLI(t, "runs", "list", "--limit", "-1", "--results-dir", dir)
	if code != 2 || !strings.Contains(stderr, "limit") {
		t.Fatalf("exit/stderr = %d/%q", code, stderr)
	}
}

func TestRunsShowSchemaError(t *testing.T) {
	dir, _ := seedRuns(t)
	bad := filepath.Join(dir, "bad")
	if err := os.Mkdir(bad, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bad, "run.json"), []byte(`{"schema_version":99,"run_id":"bad"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runCLI(t, "runs", "show", "bad", "--results-dir", dir)
	if code != 2 || !strings.Contains(stderr, "schema_version") {
		t.Fatalf("exit/stderr = %d/%q", code, stderr)
	}
}

func TestRunsListWarnsCorruptRun(t *testing.T) {
	dir, _ := seedRuns(t)
	bad := filepath.Join(dir, "bad")
	if err := os.Mkdir(bad, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bad, "run.json"), []byte("{not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runCLI(t, "runs", "list", "--results-dir", dir)
	if code != 0 {
		t.Fatalf("exit = %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stderr, "aviso") || !strings.Contains(stderr, "bad") {
		t.Fatalf("stderr = %q, want corrupt-run warning", stderr)
	}
}

func TestRunsListMissingDirDoesNotCreate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "missing")
	code, _, stderr := runCLI(t, "runs", "list", "--results-dir", dir)
	if code != 0 {
		t.Fatalf("exit = %d (stderr: %s)", code, stderr)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("runs list created the directory: %v", err)
	}
}
