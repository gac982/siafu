// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package engine

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gac982/siafu/internal/config"
	"github.com/gac982/siafu/internal/httpx"
	"github.com/gac982/siafu/internal/metrics"
	"github.com/gac982/siafu/internal/redact"
	"github.com/gac982/siafu/internal/vars"
)

func testTarget(base string) config.Target {
	return config.Target{BaseURL: base, Timeout: config.Duration{Duration: 2 * time.Second}}
}

func newTestVU(t *testing.T, target config.Target, scenario config.Scenario, batches chan metrics.Batch) *vu {
	t.Helper()
	plan, err := httpx.Prepare(&config.Config{Target: target, Scenarios: []config.Scenario{scenario}})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	tr, err := httpx.NewTransport(target, 10)
	if err != nil {
		t.Fatalf("NewTransport() error = %v", err)
	}
	t.Cleanup(tr.CloseIdleConnections)
	client, err := httpx.NewClient(target, tr, redact.New(nil))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return newVU(0, plan.Scenarios[0], client, map[string]vars.Template{}, batches, NopLogger{}, 1)
}

func collectBatches(t *testing.T, ch chan metrics.Batch) ([]metrics.Sample, []metrics.Iteration) {
	t.Helper()
	close(ch)
	var samples []metrics.Sample
	var iterations []metrics.Iteration
	for batch := range ch {
		samples = append(samples, batch.Requests...)
		iterations = append(iterations, batch.Iterations...)
	}
	return samples, iterations
}

func TestVUMultiStepCookiesAndToken(t *testing.T) {
	var logins, profiles atomic.Int64
	var currentToken atomic.Value
	currentToken.Store("")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			n := logins.Add(1)
			token := "token-" + strings.Repeat("x", int(n))
			currentToken.Store(token)
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "1", Path: "/"})
			_, _ = io.WriteString(w, `{"token":"`+token+`"}`)
		case "/me":
			profiles.Add(1)
			cookie, err := r.Cookie("session")
			if err != nil || cookie.Value != "1" {
				w.WriteHeader(401)
				return
			}
			if r.Header.Get("Authorization") != "Bearer "+currentToken.Load().(string) {
				w.WriteHeader(403)
				return
			}
			w.WriteHeader(200)
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	target := testTarget(srv.URL)
	scenario := config.Scenario{Name: "s", Steps: []config.Step{
		{
			Name:    "login",
			Request: config.Request{Method: "GET", URL: "/login"},
			Expect:  config.Expect{Status: config.StatusCodes{200}},
			Extract: map[string]string{"token": "json:token"},
		},
		{
			Name:    "me",
			Request: config.Request{Method: "GET", URL: "/me", Headers: map[string]string{"Authorization": "Bearer ${token}"}},
			Expect:  config.Expect{Status: config.StatusCodes{200}},
		},
	}}

	batches := make(chan metrics.Batch, 100)
	v := newTestVU(t, target, scenario, batches)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	v.run(ctx, make(chan struct{}))

	samples, iterations := collectBatches(t, batches)
	if logins.Load() < 3 || profiles.Load() < 3 {
		t.Fatalf("logins/profiles = %d/%d, want at least 3", logins.Load(), profiles.Load())
	}
	if len(samples) < 6 || len(iterations) < 3 {
		t.Fatalf("samples/iterations = %d/%d", len(samples), len(iterations))
	}
	for _, s := range samples {
		if s.Failed {
			t.Fatalf("sample failed: %+v", s)
		}
	}
	for _, it := range iterations {
		if it.Failed {
			t.Fatalf("iteration failed: %+v", it)
		}
	}
}

func TestVUExtractionClearedEachIteration(t *testing.T) {
	var tokenCounter atomic.Int64
	var profiles, failedProfiles atomic.Int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			n := tokenCounter.Add(1)
			_, _ = io.WriteString(w, `{"token":"tok-`+strings.Repeat("y", int(n))+`"}`)
		case "/me":
			profiles.Add(1)
			if r.Header.Get("Authorization") != "Bearer tok-"+strings.Repeat("y", int(tokenCounter.Load())) {
				failedProfiles.Add(1)
				w.WriteHeader(403)
				return
			}
			w.WriteHeader(200)
		}
	}))
	defer srv.Close()

	target := testTarget(srv.URL)
	scenario := config.Scenario{Name: "s", Steps: []config.Step{
		{Name: "login", Request: config.Request{Method: "GET", URL: "/login"}, Expect: config.Expect{Status: config.StatusCodes{200}}, Extract: map[string]string{"token": "json:token"}},
		{Name: "me", Request: config.Request{Method: "GET", URL: "/me", Headers: map[string]string{"Authorization": "Bearer ${token}"}}, Expect: config.Expect{Status: config.StatusCodes{200}}},
	}}

	batches := make(chan metrics.Batch, 100)
	v := newTestVU(t, target, scenario, batches)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	v.run(ctx, make(chan struct{}))
	collectBatches(t, batches)

	if profiles.Load() < 3 {
		t.Fatalf("profiles = %d, want at least 3", profiles.Load())
	}
	if failedProfiles.Load() != 0 {
		t.Fatalf("stale token used %d times", failedProfiles.Load())
	}
}

func TestVUCheckFailureContinuesStep(t *testing.T) {
	var failedStep, nextStep atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/fail":
			failedStep.Add(1)
			w.WriteHeader(500)
		case "/next":
			nextStep.Add(1)
			w.WriteHeader(200)
		}
	}))
	defer srv.Close()

	target := testTarget(srv.URL)
	scenario := config.Scenario{Name: "s", Steps: []config.Step{
		{Name: "fail", Request: config.Request{Method: "GET", URL: "/fail"}, Expect: config.Expect{Status: config.StatusCodes{200}}},
		{Name: "next", Request: config.Request{Method: "GET", URL: "/next"}, Expect: config.Expect{Status: config.StatusCodes{200}}},
	}}

	batches := make(chan metrics.Batch, 100)
	v := newTestVU(t, target, scenario, batches)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()
	v.run(ctx, make(chan struct{}))
	samples, iterations := collectBatches(t, batches)

	if failedStep.Load() == 0 || nextStep.Load() == 0 {
		t.Fatalf("steps = fail %d, next %d; both must run", failedStep.Load(), nextStep.Load())
	}
	if len(samples) == 0 || !samples[0].Failed || samples[0].CheckReason != "status" {
		t.Errorf("samples = %+v", samples)
	}
	if len(iterations) == 0 || !iterations[0].Failed {
		t.Errorf("iterations = %+v", iterations)
	}
}

func TestVUTransportErrorAbortsIteration(t *testing.T) {
	var nextStep atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/fail":
			conn, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				_ = conn.Close()
			}
		case "/next":
			nextStep.Add(1)
			w.WriteHeader(200)
		}
	}))
	defer srv.Close()

	target := testTarget(srv.URL)
	scenario := config.Scenario{Name: "s", Steps: []config.Step{
		{Name: "fail", Request: config.Request{Method: "GET", URL: "/fail"}},
		{Name: "next", Request: config.Request{Method: "GET", URL: "/next"}},
	}}

	batches := make(chan metrics.Batch, 100)
	v := newTestVU(t, target, scenario, batches)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()
	v.run(ctx, make(chan struct{}))
	collectBatches(t, batches)

	if nextStep.Load() != 0 {
		t.Fatalf("next step ran %d times after transport error", nextStep.Load())
	}
}

func TestVUThinkTimeRespected(t *testing.T) {
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	target := testTarget(srv.URL)
	scenario := config.Scenario{Name: "s", Steps: []config.Step{
		{Name: "one", Request: config.Request{Method: "GET", URL: "/"}, Expect: config.Expect{Status: config.StatusCodes{200}}},
	}}
	batches := make(chan metrics.Batch, 100)
	v := newTestVU(t, target, scenario, batches)
	v.cfg.ThinkTime = config.ThinkTime{Min: config.Duration{Duration: 60 * time.Millisecond}, Max: config.Duration{Duration: 60 * time.Millisecond}}

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	v.run(ctx, make(chan struct{}))
	collectBatches(t, batches)

	if hits.Load() > 6 {
		t.Fatalf("hits = %d; think time was not respected", hits.Load())
	}
}

func TestFlusherFlushesAtSampleLimit(t *testing.T) {
	out := make(chan metrics.Batch, 1)
	f := newFlusher(out, 2)
	f.addSample(metrics.Sample{Step: "one"})
	if f.full() {
		t.Fatal("flusher full after one sample")
	}
	f.addSample(metrics.Sample{Step: "two"})
	if !f.full() {
		t.Fatal("flusher not full after two samples")
	}
	f.flush(context.Background())
	select {
	case batch := <-out:
		if len(batch.Requests) != 2 {
			t.Fatalf("batch samples = %d, want 2", len(batch.Requests))
		}
	default:
		t.Fatal("flush did not emit a batch")
	}
}

func TestResetExtractedReusesMap(t *testing.T) {
	original := map[string]string{"token": "old", "id": "42"}
	scope := &vars.Scope{Extracted: original}

	resetExtracted(scope)

	if len(scope.Extracted) != 0 {
		t.Fatalf("extracted = %v, want empty", scope.Extracted)
	}
	scope.Extracted["token"] = "new"
	if original["token"] != "new" {
		t.Fatal("resetExtracted replaced the map instead of clearing it")
	}
}

var benchSinkMap map[string]string

func BenchmarkResetExtracted(b *testing.B) {
	b.Run("reuse", func(b *testing.B) {
		scope := &vars.Scope{Extracted: map[string]string{"token": "x", "id": "y"}}
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			scope.Extracted["token"] = "x"
			scope.Extracted["id"] = "y"
			resetExtracted(scope)
		}
	})
	b.Run("allocate", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			benchSinkMap = map[string]string{"token": "x", "id": "y"}
		}
	})
}
