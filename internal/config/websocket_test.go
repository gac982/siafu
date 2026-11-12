// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package config

import (
	"strings"
	"testing"
	"time"
)

const webSocketYAML = `
version: 1
name: websocket
target:
  base_url: "http://localhost:8080"
load:
  stages:
    - duration: 1s
      target_vus: 1
scenarios:
  - name: ws
    steps:
      - name: conectar
        websocket:
          action: connect
          url: /ws
      - name: enviar
        websocket:
          action: send
          text: hola
      - name: recibir
        websocket:
          action: receive
          expect:
            message_contains: "ok"
      - name: cerrar
        websocket:
          action: close
`

func webSocketConfig(t *testing.T) *Config {
	t.Helper()
	cfg, err := Decode(strings.NewReader(webSocketYAML))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	return cfg
}

func TestWebSocketDefaults(t *testing.T) {
	cfg := webSocketConfig(t)
	connect := cfg.Scenarios[0].Steps[0].WebSocket
	if connect.Connection != "default" {
		t.Errorf("connection = %q, want default", connect.Connection)
	}
	if len(connect.Expect.Status) != 1 || connect.Expect.Status[0] != 101 {
		t.Errorf("status = %v, want [101]", connect.Expect.Status)
	}
	closeStep := cfg.Scenarios[0].Steps[3].WebSocket
	if closeStep.CloseCode != 1000 {
		t.Errorf("close_code = %d, want 1000", closeStep.CloseCode)
	}
}

func TestWebSocketValidation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{"valid", func(*Config) {}, ""},
		{"request and websocket", func(c *Config) {
			c.Scenarios[0].Steps[0].Request.URL = "/also"
		}, "mutuamente"},
		{"unknown action", func(c *Config) {
			c.Scenarios[0].Steps[0].WebSocket.Action = "wat"
		}, "action"},
		{"connect without url", func(c *Config) {
			c.Scenarios[0].Steps[0].WebSocket.URL = ""
		}, "url"},
		{"send both payloads", func(c *Config) {
			c.Scenarios[0].Steps[1].WebSocket.BinaryBase64 = "eA=="
		}, "text"},
		{"send neither payload", func(c *Config) {
			c.Scenarios[0].Steps[1].WebSocket.Text = ""
		}, "text"},
		{"invalid base64", func(c *Config) {
			c.Scenarios[0].Steps[1].WebSocket.Text = ""
			c.Scenarios[0].Steps[1].WebSocket.BinaryBase64 = "%%%"
		}, "base64"},
		{"invalid connection", func(c *Config) {
			c.Scenarios[0].Steps[0].WebSocket.Connection = "bad name"
		}, "connection"},
		{"connect message_contains", func(c *Config) {
			c.Scenarios[0].Steps[0].WebSocket.Expect.MessageContains = "x"
		}, "message_contains"},
		{"non connect url", func(c *Config) {
			c.Scenarios[0].Steps[1].WebSocket.URL = "/x"
		}, "url"},
		{"http expect on websocket", func(c *Config) {
			c.Scenarios[0].Steps[0].Expect.Status = StatusCodes{200}
		}, "expect"},
		{"extract on websocket", func(c *Config) {
			c.Scenarios[0].Steps[0].Extract = map[string]string{"x": "json:x"}
		}, "extract"},
		{"timeout too small", func(c *Config) {
			c.Scenarios[0].Steps[0].WebSocket.Timeout = Duration{Duration: 500 * time.Millisecond}
		}, "timeout"},
		{"invalid close code", func(c *Config) {
			c.Scenarios[0].Steps[3].WebSocket.CloseCode = 99
		}, "close_code"},
		{"subprotocol on send", func(c *Config) {
			c.Scenarios[0].Steps[1].WebSocket.Subprotocols = []string{"x"}
		}, "subprotocols"},
		{"headers on send", func(c *Config) {
			c.Scenarios[0].Steps[1].WebSocket.Headers = map[string]string{"X": "y"}
		}, "headers"},
		{"status on receive", func(c *Config) {
			c.Scenarios[0].Steps[2].WebSocket.Expect.Status = StatusCodes{200}
		}, "status"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := webSocketConfig(t)
			tc.mutate(cfg)
			err := cfg.Validate()
			if tc.want == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), tc.want) {
				t.Fatalf("Validate() = %v, want error containing %q", err, tc.want)
			}
		})
	}
}
