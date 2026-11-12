// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package httpx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/gac982/siafu/internal/config"
	"github.com/gac982/siafu/internal/vars"
)

func echoWebSocketServer(t *testing.T, subprotocols []string) (*httptest.Server, *string) {
	t.Helper()
	gotHeader := new(string)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*gotHeader = r.Header.Get("X-Test")
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: subprotocols})
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
	return srv, gotHeader
}

func wsStep(name, action string) *config.WebSocketStep {
	return &config.WebSocketStep{Action: action, Connection: "default"}
}

func TestWebSocketEchoFlow(t *testing.T) {
	srv, gotHeader := echoWebSocketServer(t, []string{"json"})
	target := targetFor(srv.URL)
	client := clientFor(t, target)

	connect := prepareOne(t, target, config.Step{Name: "connect", WebSocket: &config.WebSocketStep{
		Action: config.WSActionConnect, URL: "/ws", Connection: "default",
		Headers: map[string]string{"X-Test": "yes"}, Subprotocols: []string{"json"},
	}})
	res := client.Do(context.Background(), connect, vars.Scope{})
	if res.Failed() || res.Protocol != "WebSocket" || res.Status != 101 {
		t.Fatalf("connect result = %+v", res)
	}
	if *gotHeader != "yes" {
		t.Errorf("handshake header = %q", *gotHeader)
	}

	send := prepareOne(t, target, config.Step{Name: "send", WebSocket: &config.WebSocketStep{
		Action: config.WSActionSend, Connection: "default", Text: "hola",
	}})
	if res := client.Do(context.Background(), send, vars.Scope{}); res.Failed() {
		t.Fatalf("send result = %+v", res)
	}

	receive := prepareOne(t, target, config.Step{Name: "receive", WebSocket: &config.WebSocketStep{
		Action: config.WSActionReceive, Connection: "default",
		Expect: config.WebSocketExpect{MessageContains: "hola"},
	}})
	if res := client.Do(context.Background(), receive, vars.Scope{}); res.Failed() {
		t.Fatalf("receive result = %+v", res)
	}

	closeStep := prepareOne(t, target, config.Step{Name: "close", WebSocket: &config.WebSocketStep{
		Action: config.WSActionClose, Connection: "default", CloseCode: 1000,
	}})
	if res := client.Do(context.Background(), closeStep, vars.Scope{}); res.Failed() {
		t.Fatalf("close result = %+v", res)
	}
}

func TestWebSocketReceiveTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		<-r.Context().Done()
	}))
	defer srv.Close()
	target := targetFor(srv.URL)
	client := clientFor(t, target)

	connect := prepareOne(t, target, config.Step{Name: "connect", WebSocket: wsStep("connect", config.WSActionConnect)})
	connect.WS.URL = mustTemplate(t, "/ws")
	if res := client.Do(context.Background(), connect, vars.Scope{}); res.Failed() {
		t.Fatalf("connect result = %+v", res)
	}
	receive := prepareOne(t, target, config.Step{Name: "receive", WebSocket: &config.WebSocketStep{
		Action: config.WSActionReceive, Connection: "default", Timeout: config.Duration{Duration: 100 * time.Millisecond},
	}})
	res := client.Do(context.Background(), receive, vars.Scope{})
	if res.ErrKind != ErrWebSocketTimeout {
		t.Fatalf("ErrKind = %q, want %q (res: %+v)", res.ErrKind, ErrWebSocketTimeout, res)
	}
}

func TestWebSocketMissingConnection(t *testing.T) {
	target := targetFor("http://127.0.0.1:1")
	client := clientFor(t, target)
	send := prepareOne(t, target, config.Step{Name: "send", WebSocket: &config.WebSocketStep{
		Action: config.WSActionSend, Connection: "missing", Text: "x",
	}})
	res := client.Do(context.Background(), send, vars.Scope{})
	if res.ErrKind != ErrWebSocketProtocol {
		t.Fatalf("ErrKind = %q, want %q (res: %+v)", res.ErrKind, ErrWebSocketProtocol, res)
	}
}

func TestWebSocketBinarySend(t *testing.T) {
	srv, _ := echoWebSocketServer(t, nil)
	target := targetFor(srv.URL)
	client := clientFor(t, target)
	connect := prepareOne(t, target, config.Step{Name: "connect", WebSocket: wsStep("connect", config.WSActionConnect)})
	connect.WS.URL = mustTemplate(t, "/ws")
	if res := client.Do(context.Background(), connect, vars.Scope{}); res.Failed() {
		t.Fatalf("connect result = %+v", res)
	}
	send := prepareOne(t, target, config.Step{Name: "send", WebSocket: &config.WebSocketStep{
		Action: config.WSActionSend, Connection: "default", BinaryBase64: "AAE=",
	}})
	if res := client.Do(context.Background(), send, vars.Scope{}); res.Failed() {
		t.Fatalf("binary send result = %+v", res)
	}
	client.CloseWebSockets()
	if len(client.wsConns) != 0 {
		t.Errorf("conexiones restantes = %d", len(client.wsConns))
	}
}

func mustTemplate(t *testing.T, raw string) vars.Template {
	t.Helper()
	tpl, err := vars.Compile(raw)
	if err != nil {
		t.Fatal(err)
	}
	return tpl
}

func TestWebSocketWSSUsesHTTP1(t *testing.T) {
	var proto string
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proto = r.Proto
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
	srv.EnableHTTP2 = true
	srv.StartTLS()
	defer srv.Close()

	target := targetFor(srv.URL)
	target.HTTPVersion = config.HTTPVersionAuto
	target.TLS = config.TLSConfig{InsecureSkipVerify: true}
	client := clientFor(t, target)
	connect := prepareOne(t, target, config.Step{Name: "connect", WebSocket: &config.WebSocketStep{
		Action: config.WSActionConnect, URL: "/ws", Connection: "default",
	}})
	if res := client.Do(context.Background(), connect, vars.Scope{}); res.Failed() {
		t.Fatalf("connect wss result = %+v", res)
	}
	if proto != "HTTP/1.1" {
		t.Errorf("handshake proto = %q; quiero HTTP/1.1", proto)
	}
}
