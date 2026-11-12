// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package cli

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gac982/siafu/internal/results"
	"github.com/gac982/siafu/internal/web"
)

func TestServeHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := runServe([]string{"--help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d (stderr: %s)", code, stderr.String())
	}
}

func TestServeInvalidAddr(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runServe([]string{"--addr", "bad"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (stderr: %s)", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "dirección inválida") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestServeNonLoopbackWarning(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runServe([]string{"--addr", "0.0.0.0:bad"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (stderr: %s)", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "aviso") || !strings.Contains(stderr.String(), "autenticación") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestServeInvalidWebDir(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runServe([]string{"--web-dir", "/no/existe", "--addr", "127.0.0.1:0"}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "web-dir") {
		t.Fatalf("exit/stderr = %d/%q", code, stderr.String())
	}
}

func TestLoopbackHost(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "::1", "localhost", "LOCALHOST"} {
		if !isLoopbackHost(host) {
			t.Errorf("isLoopbackHost(%q) = false", host)
		}
	}
	for _, host := range []string{"0.0.0.0", "::", "", "192.168.1.10"} {
		if isLoopbackHost(host) {
			t.Errorf("isLoopbackHost(%q) = true", host)
		}
	}
}

func TestServeCommandHelp(t *testing.T) {
	code, _, _ := runCLI(t, "serve", "--help")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
}

func TestServeListener(t *testing.T) {
	store, err := results.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	var stdout bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- serveListener(ctx, listener, web.NewServer(store, web.AssetsFS()).Handler(), &stdout, store.Dir)
	}()

	var response *http.Response
	deadline := time.Now().Add(2 * time.Second)
	for {
		response, err = http.Get("http://" + listener.Addr().String() + "/api/health")
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("server did not start: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
	response.Body.Close()

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serveListener() = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("serveListener did not stop")
	}
	if !strings.Contains(stdout.String(), "escuchando") {
		t.Errorf("stdout = %q", stdout.String())
	}
}

func TestServeListenerWithReady(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	ready := make(chan string, 1)
	done := make(chan error, 1)
	go func() {
		done <- serveListenerWithReady(ctx, listener, http.NotFoundHandler(), io.Discard, "dir", func(url string) {
			ready <- url
		})
	}()

	url := <-ready
	if !strings.HasPrefix(url, "http://127.0.0.1:") {
		t.Errorf("url = %q", url)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("serveListenerWithReady() = %v", err)
	}
}
