// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gac982/siafu/internal/results"
)

func writeMenuConfig(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "test.yaml")
	data := `version: 1
name: prueba de menú
target:
  base_url: "http://127.0.0.1:1"
  timeout: 1s
load:
  stages:
    - {duration: 1s, target_vus: 1}
scenarios:
  - name: home
    steps:
      - name: portada
        request: {method: GET, url: /}
`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func seedHistoryRun(t *testing.T, dir, name, mode, verdict string) results.IndexEntry {
	t.Helper()
	store, err := results.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := store.Save(results.RunRecord{
		Name: name, Mode: mode, Verdict: verdict,
		Global: results.Global{P95MS: 42, RPSAvg: 100, Requests: 10},
	}, []results.SeriesPoint{{TMS: 1000, RPS: 1}}, "summary "+name+"\n")
	if err != nil {
		t.Fatal(err)
	}
	return rec.IndexEntry()
}

func TestBuildRunArgs(t *testing.T) {
	got := buildRunArgs("a.yaml", "dir", "42", "smoke", true, true, true)
	want := []string{"a.yaml", "--results-dir", "dir", "--tag", "smoke", "--seed", "42", "--no-save", "--no-tui", "--debug"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("args = %#v; quiero %#v", got, want)
	}
	got = buildRunArgs("a.yaml", "dir", "", "", false, false, false)
	want = []string{"a.yaml", "--results-dir", "dir"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("args = %#v; quiero %#v", got, want)
	}
}

func TestConfigCandidates(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile("a.yaml", []byte("version: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("b.yml", []byte("version: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := configCandidates()
	want := []string{"a.yaml", "b.yml"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("candidatos = %#v; quiero %#v", got, want)
	}
}

func TestFilterEntries(t *testing.T) {
	entries := []results.IndexEntry{
		{RunID: "id1", Name: "demo", Tag: "smoke", Mode: "closed", Verdict: "PASS"},
		{RunID: "id2", Name: "api", Tag: "prod", Mode: "open", Verdict: "FAIL"},
		{RunID: "id3", Name: "otra", Mode: "closed", Verdict: "ABORTED"},
	}
	if got := filterEntries(entries, "open", "FAIL", "api", 0); len(got) != 1 || got[0].RunID != "id2" {
		t.Errorf("open/FAIL/api = %#v", got)
	}
	if got := filterEntries(entries, "", "", "smoke", 0); len(got) != 1 || got[0].RunID != "id1" {
		t.Errorf("smoke = %#v", got)
	}
	if got := filterEntries(entries, "", "", "id3", 0); len(got) != 1 || got[0].RunID != "id3" {
		t.Errorf("id3 = %#v", got)
	}
	if got := filterEntries(entries, "", "", "", 1); len(got) != 1 || got[0].RunID != "id1" {
		t.Errorf("limit 1 = %#v", got)
	}
}

func TestConfigCandidatesNormalizesDash(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile("--evil.yaml", []byte("version: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := configCandidates()
	want := []string{"./--evil.yaml"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("candidatos = %#v; quiero %#v", got, want)
	}
}
