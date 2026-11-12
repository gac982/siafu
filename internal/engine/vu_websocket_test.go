// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/coder/websocket"

	"github.com/gac982/siafu/internal/config"
	"github.com/gac982/siafu/internal/metrics"
	"github.com/gac982/siafu/internal/vars"
)

func webSocketEcho(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		for {
			typ, data, err := conn.Read(r.Context())
			if err != nil {
				return
			}
			if err := conn.Write(r.Context(), typ, data); err != nil {
				return
			}
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func webSocketScenario() config.Scenario {
	return config.Scenario{Name: "ws", Steps: []config.Step{
		{Name: "connect", WebSocket: &config.WebSocketStep{Action: "connect", URL: "/ws", Connection: "default"}},
		{Name: "send", WebSocket: &config.WebSocketStep{Action: "send", Connection: "default", Text: "hola"}},
		{Name: "receive", WebSocket: &config.WebSocketStep{
			Action: "receive", Connection: "default",
			Expect: config.WebSocketExpect{MessageContains: "hola"},
		}},
	}}
}

func wsScope() vars.Scope {
	return vars.Scope{
		VU:        1,
		Iter:      1,
		Extracted: map[string]string{},
		Globals:   map[string]vars.Template{},
		LookupEnv: os.LookupEnv,
	}
}

func TestVUWebSocketIterations(t *testing.T) {
	srv := webSocketEcho(t)
	target := testTarget(srv.URL)
	batches := make(chan metrics.Batch, 100)
	v := newTestVU(t, target, webSocketScenario(), batches)
	flusher := newFlusher(batches, 100)
	scope := wsScope()

	for i := 0; i < 2; i++ {
		ok, stats := v.runIteration(context.Background(), &scope, flusher)
		if !ok || stats.Failed != 0 {
			t.Fatalf("iteración %d: ok=%v stats=%+v", i, ok, stats)
		}
	}
	flusher.flush(context.Background())

	samples, iterations := collectBatches(t, batches)
	if len(iterations) != 2 || len(samples) != 6 {
		t.Fatalf("samples/iterations = %d/%d", len(samples), len(iterations))
	}
	for _, sample := range samples {
		if sample.Failed {
			t.Fatalf("sample failed: %+v", sample)
		}
		if sample.Protocol != "WebSocket" {
			t.Errorf("protocol = %q; quiero WebSocket", sample.Protocol)
		}
		if sample.Step == "connect" && sample.Status != 101 {
			t.Errorf("connect status = %d; quiero 101", sample.Status)
		}
	}
	for _, iteration := range iterations {
		if iteration.Failed {
			t.Fatalf("iteration failed: %+v", iteration)
		}
	}
	if v.client.OpenWebSockets() != 0 {
		t.Errorf("conexiones sin cerrar: %d", v.client.OpenWebSockets())
	}
}

func TestVUWebSocketErrorStopsIteration(t *testing.T) {
	target := testTarget("http://127.0.0.1:1")
	scenario := config.Scenario{Name: "ws", Steps: []config.Step{
		{Name: "connect", WebSocket: &config.WebSocketStep{Action: "connect", URL: "/ws", Connection: "default"}},
		{Name: "send", WebSocket: &config.WebSocketStep{Action: "send", Connection: "default", Text: "hola"}},
	}}
	batches := make(chan metrics.Batch, 100)
	v := newTestVU(t, target, scenario, batches)
	flusher := newFlusher(batches, 100)
	scope := wsScope()

	_, stats := v.runIteration(context.Background(), &scope, flusher)
	flusher.flush(context.Background())

	samples, iterations := collectBatches(t, batches)
	if len(samples) != 1 || !samples[0].Failed || samples[0].ErrorKind != "ws_dial" {
		t.Fatalf("samples = %+v", samples)
	}
	if len(iterations) != 1 || !iterations[0].Failed || stats.Failed != 1 {
		t.Fatalf("iterations = %+v stats=%+v", iterations, stats)
	}
	if v.client.OpenWebSockets() != 0 {
		t.Errorf("conexiones sin cerrar: %d", v.client.OpenWebSockets())
	}
}

func TestVUWebSocketConcurrent(t *testing.T) {
	srv := webSocketEcho(t)
	target := testTarget(srv.URL)
	batches := make(chan metrics.Batch, 1000)
	var wg sync.WaitGroup

	vus := make([]*vu, 4)
	for i := range vus {
		vus[i] = newTestVU(t, target, webSocketScenario(), batches)
		wg.Add(1)
		go func(v *vu) {
			defer wg.Done()
			flusher := newFlusher(batches, 100)
			scope := wsScope()
			scope.VU = v.id + 1
			if _, stats := v.runIteration(context.Background(), &scope, flusher); stats.Failed != 0 {
				t.Errorf("iteración fallida: %+v", stats)
			}
			flusher.flush(context.Background())
		}(vus[i])
	}
	wg.Wait()

	samples, iterations := collectBatches(t, batches)
	if len(iterations) != 4 || len(samples) != 12 {
		t.Fatalf("samples/iterations = %d/%d", len(samples), len(iterations))
	}
	for _, sample := range samples {
		if sample.Failed {
			t.Fatalf("sample failed: %+v", sample)
		}
	}
	for _, v := range vus {
		if v.client.OpenWebSockets() != 0 {
			t.Errorf("conexiones sin cerrar en un VU: %d", v.client.OpenWebSockets())
		}
	}
}
