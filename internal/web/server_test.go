// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package web

import (
	"crypto/rand"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gac982/siafu/internal/results"
)

func seedWebStore(t *testing.T) (*results.Store, []string) {
	t.Helper()
	store, err := results.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	store.Rand = rand.Reader
	base := time.Date(2026, 11, 9, 9, 0, 0, 0, time.UTC)

	ids := make([]string, 0, 2)
	for i, name := range []string{"alpha", "beta"} {
		rec, err := store.Save(results.RunRecord{
			Name:      name,
			Mode:      []string{"closed", "open"}[i],
			Verdict:   []string{"PASS", "FAIL"}[i],
			CreatedAt: base.Add(time.Duration(i) * time.Minute),
			Global:    results.Global{Requests: int64(100 + i*100), P95MS: float64(40 + i*10), RPSAvg: float64(100 + i*10)},
			OpenLoop:  nil,
		}, []results.SeriesPoint{{TMS: 1000, RPS: float64(10 + i)}}, "summary "+name)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, rec.RunID)
	}
	return store, ids
}

func newHandler(t *testing.T, store *results.Store) http.Handler {
	t.Helper()
	return NewServer(store, AssetsFS()).Handler()
}

func doRequest(t *testing.T, handler http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestLogoServed(t *testing.T) {
	store, _ := seedWebStore(t)
	handler := newHandler(t, store)

	for _, path := range []string{"/logo.png", "/mark.png", "/favicon.png"} {
		rec := doRequest(t, handler, http.MethodGet, path)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d", path, rec.Code)
		}
		if got := rec.Header().Get("Content-Type"); got != "image/png" {
			t.Errorf("%s content-type = %q", path, got)
		}
		if rec.Body.Len() == 0 {
			t.Errorf("%s body is empty", path)
		}
	}
}

func TestAPIList(t *testing.T) {
	store, ids := seedWebStore(t)
	handler := newHandler(t, store)

	rec := doRequest(t, handler, http.MethodGet, "/api/runs")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (body: %s)", rec.Code, rec.Body.String())
	}
	var entries []results.IndexEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].RunID != ids[1] || entries[1].RunID != ids[0] {
		t.Fatalf("entries = %+v", entries)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("cache-control = %q", rec.Header().Get("Cache-Control"))
	}

	cases := []struct {
		query string
		want  int
	}{
		{"?mode=open", 1},
		{"?verdict=PASS", 1},
		{"?q=beta", 1},
		{"?limit=1", 1},
		{"?mode=closed&verdict=FAIL", 0},
	}
	for _, tc := range cases {
		rec := doRequest(t, handler, http.MethodGet, "/api/runs"+tc.query)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d", tc.query, rec.Code)
		}
		var filtered []results.IndexEntry
		if err := json.Unmarshal(rec.Body.Bytes(), &filtered); err != nil {
			t.Fatal(err)
		}
		if len(filtered) != tc.want {
			t.Errorf("%s returned %d, want %d", tc.query, len(filtered), tc.want)
		}
	}
}

func TestAPIShow(t *testing.T) {
	store, ids := seedWebStore(t)
	handler := newHandler(t, store)

	rec := doRequest(t, handler, http.MethodGet, "/api/runs/"+ids[0])
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (body: %s)", rec.Code, rec.Body.String())
	}
	var record results.RunRecord
	if err := json.Unmarshal(rec.Body.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if record.Name != "alpha" || record.RunID != ids[0] {
		t.Errorf("record = %+v", record)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("content-type = %q", ct)
	}
}

func TestAPISeries(t *testing.T) {
	store, ids := seedWebStore(t)
	handler := newHandler(t, store)

	rec := doRequest(t, handler, http.MethodGet, "/api/runs/"+ids[0]+"/series")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (body: %s)", rec.Code, rec.Body.String())
	}
	var points []results.SeriesPoint
	if err := json.Unmarshal(rec.Body.Bytes(), &points); err != nil {
		t.Fatal(err)
	}
	if len(points) != 1 || points[0].TMS != 1000 {
		t.Errorf("points = %+v", points)
	}
}

func TestAPIHealth(t *testing.T) {
	store, _ := seedWebStore(t)
	rec := doRequest(t, newHandler(t, store), http.MethodGet, "/api/health")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Errorf("body = %+v", body)
	}
}

func TestAPIPathTraversal(t *testing.T) {
	store, _ := seedWebStore(t)
	handler := newHandler(t, store)

	for _, path := range []string{
		"/api/runs/../../etc",
		"/api/runs/a%2Fb",
		"/api/runs/..",
		"/api/runs/",
	} {
		rec := doRequest(t, handler, http.MethodGet, path)
		if rec.Code != http.StatusBadRequest && rec.Code != http.StatusNotFound {
			t.Errorf("%s status = %d, want 400/404", path, rec.Code)
		}
	}
}

func TestAPINotFound(t *testing.T) {
	store, _ := seedWebStore(t)
	handler := newHandler(t, store)

	for _, path := range []string{"/api/nope", "/api/runs/no-existe"} {
		rec := doRequest(t, handler, http.MethodGet, path)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s status = %d, want 404", path, rec.Code)
		}
	}
}

func TestAPIMethodNotAllowed(t *testing.T) {
	store, _ := seedWebStore(t)
	rec := doRequest(t, newHandler(t, store), http.MethodPost, "/api/runs")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

func TestStaticAssets(t *testing.T) {
	store, _ := seedWebStore(t)
	handler := newHandler(t, store)

	cases := []struct {
		path        string
		contentType string
		body        string
	}{
		{"/", "text/html; charset=utf-8", `id="app"`},
		{"/app.js", "text/javascript; charset=utf-8", "Siafu"},
		{"/style.css", "text/css; charset=utf-8", "--"},
		{"/charts.js", "text/javascript; charset=utf-8", "lineChart"},
		{"/format.js", "text/javascript; charset=utf-8", "format"},
	}
	for _, tc := range cases {
		rec := doRequest(t, handler, http.MethodGet, tc.path)
		if rec.Code != http.StatusOK {
			t.Errorf("%s status = %d", tc.path, rec.Code)
			continue
		}
		if ct := rec.Header().Get("Content-Type"); ct != tc.contentType {
			t.Errorf("%s content-type = %q, want %q", tc.path, ct, tc.contentType)
		}
		if !strings.Contains(rec.Body.String(), tc.body) {
			t.Errorf("%s body = %q, want %q", tc.path, rec.Body.String(), tc.body)
		}
	}
}

func TestAssetsNoExternalURLs(t *testing.T) {
	fsys := AssetsFS()
	if err := walkAssets(fsys, func(path string, data []byte) error {
		// The SVG namespace is a standard identifier, not a network request.
		text := strings.ReplaceAll(string(data), "http://www.w3.org/2000/svg", "")
		for _, bad := range []string{"http://", "https://", "//cdn", "@import url(http"} {
			if strings.Contains(text, bad) {
				t.Errorf("%s contains external dependency %q", path, bad)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func walkAssets(fsys fs.FS, fn func(string, []byte) error) error {
	return fs.WalkDir(fsys, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		return fn(path, data)
	})
}

func TestServeMissingDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "missing")
	store, err := results.OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	rec := doRequest(t, NewServer(store, AssetsFS()).Handler(), http.MethodGet, "/api/runs")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (body: %s)", rec.Code, rec.Body.String())
	}
	var entries []results.IndexEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("entries = %+v, want empty", entries)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("serve created the directory: %v", err)
	}
}

func TestAPIHead(t *testing.T) {
	store, _ := seedWebStore(t)
	srv := httptest.NewServer(newHandler(t, store))
	defer srv.Close()

	cases := []struct {
		path        string
		contentType string
	}{
		{"/api/health", "application/json; charset=utf-8"},
		{"/", "text/html; charset=utf-8"},
	}
	for _, tc := range cases {
		resp, err := http.Head(srv.URL + tc.path)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("HEAD %s status = %d", tc.path, resp.StatusCode)
		}
		if got := resp.Header.Get("Content-Type"); got != tc.contentType {
			t.Errorf("HEAD %s content-type = %q, want %q", tc.path, got, tc.contentType)
		}
		resp.Body.Close()
	}
}

func TestAPICorruptRun(t *testing.T) {
	store, _ := seedWebStore(t)
	bad := filepath.Join(store.Dir, "bad")
	if err := os.Mkdir(bad, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bad, "run.json"), []byte("{not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bad, "series.jsonl"), []byte("{not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := newHandler(t, store)

	for _, path := range []string{"/api/runs/bad", "/api/runs/bad/series"} {
		rec := doRequest(t, handler, http.MethodGet, path)
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("%s status = %d, want 500", path, rec.Code)
		}
		if strings.Contains(rec.Body.String(), store.Dir) {
			t.Errorf("%s leaks the results path: %q", path, rec.Body.String())
		}
	}
}
