// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package httpx

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gac982/siafu/internal/config"
	"github.com/gac982/siafu/internal/redact"
	"github.com/gac982/siafu/internal/vars"
)

func http2Target(base, version string) config.Target {
	return config.Target{
		BaseURL:     base,
		Timeout:     config.Duration{Duration: 5 * time.Second},
		HTTPVersion: version,
		TLS:         config.TLSConfig{InsecureSkipVerify: true},
	}
}

func newHTTP2Server(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	srv := httptest.NewUnstartedServer(handler)
	srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	srv.EnableHTTP2 = true
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return srv
}

func protocolStep(t *testing.T, target config.Target, want string) PreparedStep {
	t.Helper()
	return prepareOne(t, target, config.Step{
		Name:    "protocol",
		Request: config.Request{Method: "GET", URL: "/"},
		Expect:  config.Expect{BodyContains: want},
	})
}

func TestHTTP2AutoNegotiates(t *testing.T) {
	srv := newHTTP2Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.Proto))
	}))
	target := http2Target(srv.URL, config.HTTPVersionAuto)
	res := clientFor(t, target).Do(context.Background(), protocolStep(t, target, "HTTP/2.0"), vars.Scope{})
	if res.Failed() {
		t.Fatalf("auto no negoció h2: %+v", res)
	}
	if res.Protocol != "HTTP/2.0" {
		t.Errorf("Protocol = %q; quiero HTTP/2.0", res.Protocol)
	}
}

func TestHTTP1Forced(t *testing.T) {
	srv := newHTTP2Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.Proto))
	}))
	target := http2Target(srv.URL, config.HTTPVersion11)
	res := clientFor(t, target).Do(context.Background(), protocolStep(t, target, "HTTP/1.1"), vars.Scope{})
	if res.Failed() {
		t.Fatalf("1.1 no forzó HTTP/1.1: %+v", res)
	}
	if res.Protocol != "HTTP/1.1" {
		t.Errorf("Protocol = %q; quiero HTTP/1.1", res.Protocol)
	}
}

func TestHTTP2Required(t *testing.T) {
	srv := newHTTP2Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.Proto))
	}))
	target := http2Target(srv.URL, config.HTTPVersion2)
	res := clientFor(t, target).Do(context.Background(), protocolStep(t, target, "HTTP/2.0"), vars.Scope{})
	if res.Failed() {
		t.Fatalf("2 no aceptó h2: %+v", res)
	}
}

func TestHTTP2RequiredOnHTTP1Fails(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.Proto))
	}))
	defer srv.Close()
	target := http2Target(srv.URL, config.HTTPVersion2)
	res := clientFor(t, target).Do(context.Background(), prepareOne(t, target, config.Step{
		Name: "strict", Request: config.Request{Method: "GET", URL: "/"},
	}), vars.Scope{})
	if res.ErrKind != ErrProtocol {
		t.Fatalf("ErrKind = %q, want %q (res: %+v)", res.ErrKind, ErrProtocol, res)
	}
}

func TestHTTP2OverPlainHTTPIsHTTP1(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.Proto))
	}))
	defer srv.Close()
	target := http2Target(srv.URL, config.HTTPVersionAuto)
	res := clientFor(t, target).Do(context.Background(), protocolStep(t, target, "HTTP/1.1"), vars.Scope{})
	if res.Failed() {
		t.Fatalf("http plano debería ser h1: %+v", res)
	}
}

func TestHTTP2ConcurrentClients(t *testing.T) {
	srv := newHTTP2Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.Proto))
	}))
	target := http2Target(srv.URL, config.HTTPVersionAuto)
	transport, err := NewTransport(target, 20)
	if err != nil {
		t.Fatal(err)
	}
	defer transport.CloseIdleConnections()
	step := protocolStep(t, target, "HTTP/2.0")

	var wg sync.WaitGroup
	errs := make(chan Result, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client, err := NewClient(target, transport, redact.New(nil))
			if err != nil {
				errs <- Result{ErrKind: ErrProtocol, Err: err}
				return
			}
			errs <- client.Do(context.Background(), step, vars.Scope{})
		}()
	}
	wg.Wait()
	close(errs)
	for res := range errs {
		if res.Failed() {
			t.Errorf("cliente concurrente falló: %+v", res)
		}
	}
}
