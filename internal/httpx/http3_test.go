// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package httpx

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/gac982/siafu/internal/config"
	"github.com/gac982/siafu/internal/vars"
)

func http3Target(t *testing.T, addr string, version string) config.Target {
	t.Helper()
	return config.Target{
		BaseURL:     "https://" + addr,
		Timeout:     config.Duration{Duration: 5 * time.Second},
		HTTPVersion: version,
		TLS:         config.TLSConfig{InsecureSkipVerify: true},
	}
}

func TestHTTP3TransportType(t *testing.T) {
	tr, err := NewTransport(config.Target{BaseURL: "https://localhost", HTTPVersion: "3"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	defer tr.CloseIdleConnections()
	if got := requiredProtocol("3"); got != 3 {
		t.Fatalf("requiredProtocol(3) = %d", got)
	}
	if _, ok := tr.HTTP.(interface{ CloseIdleConnections() }); !ok {
		t.Fatalf("HTTP transport %T lacks CloseIdleConnections", tr.HTTP)
	}
	if tr.WS.ForceAttemptHTTP2 {
		t.Error("WS transport must stay HTTP/1.1 under http_version 3")
	}
}

func TestHTTP3Negotiates(t *testing.T) {
	addr, _, _ := startHTTP3Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.Proto)
	}))
	target := http3Target(t, addr, config.HTTPVersion3)

	step := prepareOne(t, target, config.Step{
		Name:    "h3",
		Request: config.Request{Method: "GET", URL: "/"},
		Expect:  config.Expect{Status: config.StatusCodes{200}, BodyContains: "HTTP/3.0"},
	})
	res := clientFor(t, target).Do(context.Background(), step, vars.Scope{})
	if res.Failed() {
		t.Fatalf("h3 falló: %+v (err: %v)", res, res.Err)
	}
	if res.Protocol != "HTTP/3.0" {
		t.Errorf("Protocol = %q; quiero HTTP/3.0", res.Protocol)
	}
	if res.Status != 200 {
		t.Errorf("status = %d; quiero 200", res.Status)
	}
}

func TestHTTP3WithCAFile(t *testing.T) {
	addr, certPEM, _ := startHTTP3Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))
	caPath := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caPath, certPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	target := http3Target(t, addr, config.HTTPVersion3)
	target.TLS = config.TLSConfig{CAFile: caPath}

	step := prepareOne(t, target, config.Step{Name: "h3", Request: config.Request{Method: "GET", URL: "/"}})
	res := clientFor(t, target).Do(context.Background(), step, vars.Scope{})
	if res.Failed() {
		t.Fatalf("h3 con ca_file falló: %+v (err: %v)", res, res.Err)
	}
	if res.Protocol != "HTTP/3.0" {
		t.Errorf("Protocol = %q; quiero HTTP/3.0", res.Protocol)
	}
}

func TestHTTP3ChecksAndExtract(t *testing.T) {
	addr, _, _ := startHTTP3Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Session", "abc123")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":{"id":42}}`)
	}))
	target := http3Target(t, addr, config.HTTPVersion3)

	step := prepareOne(t, target, config.Step{
		Name:    "h3",
		Request: config.Request{Method: "GET", URL: "/"},
		Expect:  config.Expect{Status: config.StatusCodes{200}, BodyContains: `"id":42`},
		Extract: map[string]string{
			"id":      "json:data.id",
			"session": "header:X-Session",
		},
	})
	res := clientFor(t, target).Do(context.Background(), step, vars.Scope{})
	if res.Failed() {
		t.Fatalf("h3 checks/extract falló: %+v (err: %v)", res, res.Err)
	}
	if res.Extracted["id"] != "42" || res.Extracted["session"] != "abc123" {
		t.Errorf("Extracted = %+v", res.Extracted)
	}
}

func TestHTTP3StrictFailsAgainstHTTP2Server(t *testing.T) {
	srv := newHTTP2Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	target := http3Target(t, srv.Listener.Addr().String(), config.HTTPVersion3)
	target.Timeout = config.Duration{Duration: 700 * time.Millisecond}

	step := prepareOne(t, target, config.Step{Name: "h3", Request: config.Request{Method: "GET", URL: "/"}})
	res := clientFor(t, target).Do(context.Background(), step, vars.Scope{})
	if !res.Failed() {
		t.Fatalf("h3 contra servidor h2 debe fallar: %+v", res)
	}
}

func TestHTTP3WebSocketStillHTTP1(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor != 1 {
			t.Errorf("WebSocket handshake proto = %s; quiero HTTP/1.1", r.Proto)
		}
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
	srv.Config.ErrorLog = nil

	target := http3Target(t, srv.Listener.Addr().String(), config.HTTPVersion3)
	scenario := config.Scenario{Name: "ws", Steps: []config.Step{
		{Name: "connect", WebSocket: &config.WebSocketStep{Action: "connect", URL: "/ws", Connection: "c"}},
		{Name: "send", WebSocket: &config.WebSocketStep{Action: "send", Connection: "c", Text: "hola"}},
		{Name: "receive", WebSocket: &config.WebSocketStep{
			Action: "receive", Connection: "c",
			Expect: config.WebSocketExpect{MessageContains: "hola"},
		}},
	}}
	plan, err := Prepare(&config.Config{Target: target, Scenarios: []config.Scenario{scenario}})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	tr, err := NewTransport(target, 4)
	if err != nil {
		t.Fatalf("NewTransport: %v", err)
	}
	t.Cleanup(tr.CloseIdleConnections)
	client, err := NewClient(target, tr, nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.CloseWebSockets()
	for _, step := range plan.Scenarios[0].Steps {
		res := client.Do(context.Background(), step, vars.Scope{})
		if res.Failed() {
			t.Fatalf("paso %s falló: %+v (err: %v)", step.WS.Action, res, res.Err)
		}
	}
}

func TestHTTP3Concurrent(t *testing.T) {
	addr, _, _ := startHTTP3Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.Proto)
	}))
	target := http3Target(t, addr, config.HTTPVersion3)
	step := prepareOne(t, target, config.Step{Name: "h3", Request: config.Request{Method: "GET", URL: "/"}})

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res := clientFor(t, target).Do(context.Background(), step, vars.Scope{})
			if res.Failed() || res.Protocol != "HTTP/3.0" {
				t.Errorf("resultado = %+v (err: %v)", res, res.Err)
			}
		}()
	}
	wg.Wait()
}

func TestTransportCloseReleasesHTTP3(t *testing.T) {
	addr, _, _ := startHTTP3Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))
	target := http3Target(t, addr, config.HTTPVersion3)

	tr, err := NewTransport(target, 4)
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(target, tr, nil)
	if err != nil {
		t.Fatal(err)
	}
	step := prepareOne(t, target, config.Step{Name: "h3", Request: config.Request{Method: "GET", URL: "/"}})
	if res := client.Do(context.Background(), step, vars.Scope{}); res.Failed() {
		t.Fatalf("antes de Close: %+v (err: %v)", res, res.Err)
	}
	if err := tr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if res := client.Do(context.Background(), step, vars.Scope{}); !res.Failed() {
		t.Fatalf("después de Close debía fallar: %+v", res)
	}
	if err := tr.Close(); err != nil {
		t.Fatalf("segundo Close: %v", err)
	}
}
