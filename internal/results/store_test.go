// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package results

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func newTestStore(t *testing.T, dir string) *Store {
	t.Helper()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	store.Rand = rand.Reader
	return store
}

func readIndex(t *testing.T, dir string) []IndexEntry {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "index.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var entries []IndexEntry
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var entry IndexEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("index line %q: %v", line, err)
		}
		entries = append(entries, entry)
	}
	return entries
}

func TestStoreSaveLoadSeries(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore(t, dir)
	when := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	store.Now = func() time.Time { return when }
	store.Rand = bytes.NewReader([]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08})

	rec := RunRecord{
		Name:      "Demo local",
		Mode:      "closed",
		Verdict:   "PASS",
		CreatedAt: when,
		Global:    Global{Requests: 10, P95MS: 12},
	}
	series := []SeriesPoint{
		{TMS: 1000, RPS: 10},
		{TMS: 2000, RPS: 20},
	}

	saved, err := store.Save(rec, series, "resumen")
	if err != nil {
		t.Fatal(err)
	}
	if saved.RunID == "" || !strings.HasSuffix(saved.RunID, "-0102030405060708") {
		t.Fatalf("run id = %q", saved.RunID)
	}
	if saved.SchemaVersion != SchemaVersion || saved.CreatedAt != when {
		t.Fatalf("saved = %+v", saved)
	}

	runDir := filepath.Join(dir, saved.RunID)
	for _, name := range []string{"run.json", "series.jsonl", "summary.txt"} {
		if _, err := os.Stat(filepath.Join(runDir, name)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	summary, err := os.ReadFile(filepath.Join(runDir, "summary.txt"))
	if err != nil || string(summary) != "resumen" {
		t.Errorf("summary = %q, err = %v", summary, err)
	}

	entries := readIndex(t, dir)
	if len(entries) != 1 || entries[0].RunID != saved.RunID {
		t.Fatalf("index = %+v", entries)
	}

	list, err := store.List()
	if err != nil || len(list) != 1 || list[0].RunID != saved.RunID {
		t.Fatalf("list = %+v, err = %v", list, err)
	}

	loaded, err := store.Load(saved.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded, saved) {
		t.Errorf("loaded = %+v, want %+v", loaded, saved)
	}

	gotSeries, err := store.Series(saved.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotSeries, series) {
		t.Errorf("series = %+v, want %+v", gotSeries, series)
	}
}

func TestStoreConcurrentSave(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore(t, dir)
	when := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	store.Now = func() time.Time { return when }

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := store.Save(RunRecord{
				Name:      "concurrent",
				Mode:      "closed",
				Verdict:   "PASS",
				CreatedAt: when.Add(time.Duration(i) * time.Second),
			}, nil, "ok")
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	list, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("list = %+v, want 2", list)
	}
	if entries := readIndex(t, dir); len(entries) != 2 {
		t.Fatalf("index = %+v, want 2", entries)
	}
}

func TestStoreRejectsPathTraversal(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore(t, dir)

	for _, id := range []string{"../x", "a/b", "..", "", ".", `a\b`, "a..b"} {
		if ValidID(id) {
			t.Errorf("ValidID(%q) = true", id)
		}
		if _, err := store.Load(id); err == nil {
			t.Errorf("Load(%q) succeeded", id)
		}
		if _, err := store.Series(id); err == nil {
			t.Errorf("Series(%q) succeeded", id)
		}
	}
}

func TestStoreUnknownSchema(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore(t, dir)
	id := "bad-schema"
	if err := os.Mkdir(filepath.Join(dir, id), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id, "run.json"), []byte(`{"schema_version":99}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := store.Load(id); err == nil || !strings.Contains(err.Error(), "99") {
		t.Fatalf("Load() error = %v, want schema version", err)
	}
}

func TestStoreListRebuildsMissingIndex(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore(t, dir)
	base := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 2; i++ {
		if _, err := store.Save(RunRecord{
			Name:      "run",
			Mode:      "closed",
			Verdict:   "PASS",
			CreatedAt: base.Add(time.Duration(i) * time.Second),
		}, nil, "ok"); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Remove(filepath.Join(dir, "index.jsonl")); err != nil {
		t.Fatal(err)
	}

	list, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("list = %+v, want 2", list)
	}
	if !list[0].CreatedAt.After(list[1].CreatedAt) {
		t.Errorf("list is not newest-first: %+v", list)
	}
}

func TestStorePrune(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore(t, dir)
	base := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	var saved []RunRecord
	for i := 0; i < 3; i++ {
		rec, err := store.Save(RunRecord{
			Name:      "run",
			Mode:      "closed",
			Verdict:   "PASS",
			CreatedAt: base.Add(time.Duration(i) * time.Second),
		}, nil, "ok")
		if err != nil {
			t.Fatal(err)
		}
		saved = append(saved, rec)
	}

	removed, err := store.Prune(2)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	if _, err := os.Stat(filepath.Join(dir, saved[0].RunID)); !os.IsNotExist(err) {
		t.Errorf("oldest run still exists: %v", err)
	}
	list, err := store.List()
	if err != nil || len(list) != 2 {
		t.Fatalf("list = %+v, err = %v", list, err)
	}
	if entries := readIndex(t, dir); len(entries) != 2 {
		t.Fatalf("index = %+v, want 2", entries)
	}
}

func TestStorePruneRejectsInvalidIndexIDs(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore(t, dir)
	if _, err := store.Save(RunRecord{Name: "alpha", Mode: "closed", Verdict: "PASS"}, nil, "ok"); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(t.TempDir(), "victim")
	if err := os.Mkdir(victim, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.jsonl"), []byte(`{"schema_version":1,"run_id":"../../victim"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	removed, err := store.Prune(0)
	if err != nil {
		t.Fatalf("Prune() = %v", err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	if _, err := os.Stat(victim); err != nil {
		t.Fatalf("victim was deleted: %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("results dir was deleted: %v", err)
	}
}

func TestStoreListReconcilesStaleIndex(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore(t, dir)
	base := time.Date(2026, 11, 9, 9, 0, 0, 0, time.UTC)
	if _, err := store.Save(RunRecord{Name: "alpha", Mode: "closed", Verdict: "PASS", CreatedAt: base}, nil, "ok"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Save(RunRecord{Name: "beta", Mode: "closed", Verdict: "PASS", CreatedAt: base.Add(time.Second)}, nil, "ok"); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "index.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if err := os.WriteFile(filepath.Join(dir, "index.jsonl"), []byte(lines[0]+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	list, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("list = %+v, want both runs after reconciliation", list)
	}
}

func TestStoreListRebuildsCorruptIndex(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore(t, dir)
	if _, err := store.Save(RunRecord{Name: "alpha", Mode: "closed", Verdict: "PASS"}, nil, "ok"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.jsonl"), []byte("{not-json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	list, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "alpha" {
		t.Fatalf("list = %+v", list)
	}
}

func TestStoreLoadRejectsMismatchedID(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore(t, dir)
	saved, err := store.Save(RunRecord{Name: "alpha", Mode: "closed", Verdict: "PASS"}, nil, "ok")
	if err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(dir, "other")
	if err := os.Mkdir(other, 0o700); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, saved.RunID, "run.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, "run.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load("other"); err == nil || !strings.Contains(err.Error(), "pertenece") {
		t.Fatalf("Load() error = %v, want mismatched id", err)
	}
}

func TestStoreSaveForcesSchemaVersion(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore(t, dir)
	saved, err := store.Save(RunRecord{SchemaVersion: 99, Name: "alpha", Mode: "closed", Verdict: "PASS"}, nil, "ok")
	if err != nil {
		t.Fatal(err)
	}
	if saved.SchemaVersion != SchemaVersion {
		t.Fatalf("schema = %d, want %d", saved.SchemaVersion, SchemaVersion)
	}
	if _, err := store.Load(saved.RunID); err != nil {
		t.Fatalf("Load() = %v", err)
	}
}

func TestStoreSaveSyncsArtifacts(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore(t, dir)
	var fileSyncs, dirSyncs int
	store.Sync = func(*os.File) error { fileSyncs++; return nil }
	store.SyncDir = func(string) error { dirSyncs++; return nil }

	if _, err := store.Save(RunRecord{Name: "alpha", Mode: "closed", Verdict: "PASS"}, nil, "ok"); err != nil {
		t.Fatal(err)
	}
	if fileSyncs < 4 {
		t.Errorf("file syncs = %d, want >= 4 (three artifacts + index)", fileSyncs)
	}
	if dirSyncs < 2 {
		t.Errorf("dir syncs = %d, want >= 2 (temp + parent)", dirSyncs)
	}
}

func TestStoreReindexSyncs(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore(t, dir)
	if _, err := store.Save(RunRecord{Name: "alpha", Mode: "closed", Verdict: "PASS"}, nil, "ok"); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "index.jsonl")); err != nil {
		t.Fatal(err)
	}
	var fileSyncs, dirSyncs int
	store.Sync = func(*os.File) error { fileSyncs++; return nil }
	store.SyncDir = func(string) error { dirSyncs++; return nil }
	if err := store.Reindex(); err != nil {
		t.Fatal(err)
	}
	if fileSyncs < 1 || dirSyncs < 1 {
		t.Errorf("syncs = %d/%d, want >=1/1", fileSyncs, dirSyncs)
	}
}

func TestSyncDirReal(t *testing.T) {
	if err := syncDir(t.TempDir()); err != nil {
		t.Fatalf("syncDir() = %v", err)
	}
}

func TestStoreSaveSyncError(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore(t, dir)
	store.Sync = func(*os.File) error { return errors.New("sync failed") }
	if _, err := store.Save(RunRecord{Name: "alpha", Mode: "closed", Verdict: "PASS"}, nil, "ok"); err == nil {
		t.Fatal("Save() = nil, want sync error")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("temp dir left behind: %+v", entries)
	}
}

func TestStoreIDCollisionRetriesThenFails(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore(t, dir)
	when := time.Date(2026, 11, 9, 9, 0, 0, 0, time.UTC)
	store.Rand = bytes.NewReader(bytes.Repeat([]byte{1, 2, 3, 4, 5, 6, 7, 8}, 10))
	store.Now = func() time.Time { return when }

	expected := when.Format("20060102-150405") + "-collision-0102030405060708"
	if err := os.Mkdir(filepath.Join(dir, expected), 0o700); err != nil {
		t.Fatal(err)
	}

	_, err := store.Save(RunRecord{Name: "collision", CreatedAt: when, Mode: "closed", Verdict: "PASS"}, nil, "ok")
	if err == nil || !strings.Contains(err.Error(), "id único") {
		t.Fatalf("Save() error = %v, want unique id failure", err)
	}
}

func TestStoreLongIndexLine(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore(t, dir)
	tag := strings.Repeat("x", 200*1024)
	if _, err := store.Save(RunRecord{Name: "long-tag", Tag: tag, Mode: "closed", Verdict: "PASS"}, nil, "ok"); err != nil {
		t.Fatal(err)
	}
	entries, err := store.readIndex()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Tag != tag {
		t.Fatalf("index = %+v", entries)
	}
	list, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Tag != tag {
		t.Fatalf("list = %+v", list)
	}
}

func TestStoreLongSeriesLine(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore(t, dir)
	saved, err := store.Save(RunRecord{Name: "long-series", Mode: "closed", Verdict: "PASS"}, []SeriesPoint{{TMS: 1000, RPS: 10}}, "ok")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, saved.RunID, "series.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, []byte(strings.Repeat(" ", 200*1024)+"\n")...)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	points, err := store.Series(saved.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 1 || points[0].TMS != 1000 {
		t.Fatalf("points = %+v", points)
	}
}

func TestStoreScanWarnsOnCorruptRun(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore(t, dir)
	if _, err := store.Save(RunRecord{Name: "valid", Mode: "closed", Verdict: "PASS"}, nil, "ok"); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(dir, "bad")
	if err := os.Mkdir(bad, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bad, "run.json"), []byte("{not-json"), 0o600); err != nil {
		t.Fatal(err)
	}

	var warnings []string
	store.Warn = func(msg string) { warnings = append(warnings, msg) }
	list, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "valid" {
		t.Fatalf("list = %+v", list)
	}
	if len(warnings) == 0 || !strings.Contains(warnings[0], "bad") {
		t.Fatalf("warnings = %+v, want a corrupt-run warning", warnings)
	}
}

func TestStoreLockSerializes(t *testing.T) {
	dir := t.TempDir()
	first := newTestStore(t, dir)
	second := newTestStore(t, dir)

	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = first.withLock(func() error {
			close(started)
			<-release
			return nil
		})
	}()
	<-started
	go func() {
		time.Sleep(200 * time.Millisecond)
		close(release)
	}()

	begin := time.Now()
	if err := second.withLock(func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(begin)
	<-done
	if elapsed < 100*time.Millisecond {
		t.Fatalf("lock did not serialize: %s", elapsed)
	}
}

func TestStorePruneReindexNoDeadlock(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore(t, dir)
	base := time.Date(2026, 11, 9, 9, 0, 0, 0, time.UTC)
	for i := 0; i < 2; i++ {
		if _, err := store.Save(RunRecord{Name: "run", Mode: "closed", Verdict: "PASS", CreatedAt: base.Add(time.Duration(i) * time.Second)}, nil, "ok"); err != nil {
			t.Fatal(err)
		}
	}
	done := make(chan error, 1)
	go func() {
		_, err := store.Prune(1)
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Prune deadlocked")
	}
}

func TestStoreSavePruneConcurrent(t *testing.T) {
	dir := t.TempDir()
	first := newTestStore(t, dir)
	second := newTestStore(t, dir)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err := first.Save(RunRecord{Name: "concurrent", Mode: "closed", Verdict: "PASS"}, nil, "ok")
		errs <- err
	}()
	go func() {
		defer wg.Done()
		_, err := second.Prune(0)
		errs <- err
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := first.List(); err != nil {
		t.Fatal(err)
	}
}

func TestOpenStoreDoesNotCreate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "missing")
	store, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("OpenStore created the directory: %v", err)
	}
	list, err := store.List()
	if err != nil || len(list) != 0 {
		t.Fatalf("List() = %+v, err = %v", list, err)
	}
}

func TestPruneMissingDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "missing")
	store, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	removed, err := store.Prune(10)
	if err != nil || removed != 0 {
		t.Fatalf("Prune() = %d, %v", removed, err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("Prune created the directory: %v", err)
	}
}

func TestStorePermissions(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore(t, dir)
	saved, err := store.Save(RunRecord{Name: "perms", Mode: "closed", Verdict: "PASS"}, []SeriesPoint{{TMS: 1000, RPS: 1}}, "ok")
	if err != nil {
		t.Fatal(err)
	}
	check := func(path string, want os.FileMode) {
		t.Helper()
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Errorf("%s mode = %o, want %o", path, got, want)
		}
	}
	check(filepath.Join(dir, saved.RunID), 0o700)
	for _, name := range []string{"run.json", "series.jsonl", "summary.txt"} {
		check(filepath.Join(dir, saved.RunID, name), 0o600)
	}
	check(filepath.Join(dir, "index.jsonl"), 0o600)
}

func TestStorePruneIDs(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore(t, dir)
	base := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	var saved []RunRecord
	for i := 0; i < 3; i++ {
		rec, err := store.Save(RunRecord{
			Name:      "run",
			Mode:      "closed",
			Verdict:   "PASS",
			CreatedAt: base.Add(time.Duration(i) * time.Second),
		}, nil, "ok")
		if err != nil {
			t.Fatal(err)
		}
		saved = append(saved, rec)
	}

	removed, err := store.PruneIDs([]string{saved[0].RunID, saved[1].RunID, saved[0].RunID})
	if err != nil {
		t.Fatal(err)
	}
	if removed != 2 {
		t.Errorf("removed = %d; quiero 2", removed)
	}
	entries, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].RunID != saved[2].RunID {
		t.Errorf("entradas = %#v; quiero solo %s", entries, saved[2].RunID)
	}
}

func TestStorePruneIDsRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore(t, dir)
	if _, err := store.Save(RunRecord{Name: "alpha", Mode: "closed", Verdict: "PASS"}, nil, "ok"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.PruneIDs([]string{"../victim"}); err == nil {
		t.Fatal("PruneIDs aceptó un ID con traversal")
	}
}
