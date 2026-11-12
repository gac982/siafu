// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

// Command siafu-demo is a tiny deterministic HTTP target for examples and E2E.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/coder/websocket"
)

type product struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Price int    `json:"price"`
}

type demo struct {
	counter atomic.Int64
	tokens  sync.Map // token -> email
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "dirección de escucha")
	flag.Parse()

	server := &http.Server{
		Addr:              *addr,
		Handler:           newMux(&demo{}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	log.Printf("siafu-demo escuchando en http://%s", *addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func newMux(d *demo) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", d.home)
	mux.HandleFunc("/api/products", d.products)
	mux.HandleFunc("/api/products/", d.product)
	mux.HandleFunc("/api/login", d.login)
	mux.HandleFunc("/api/me", d.me)
	mux.HandleFunc("/api/slow", d.slow)
	mux.HandleFunc("/api/error", d.failure)
	mux.HandleFunc("/ws", d.webSocket)
	return mux
}

func (d *demo) home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte("<!doctype html><html><head><title>Siafu demo</title></head><body><h1>Siafu demo</h1><p>Objetivo de pruebas de carga.</p></body></html>"))
}

func (d *demo) products(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	items := []product{
		{ID: 1, Name: "teclado", Price: 45},
		{ID: 2, Name: "ratón", Price: 25},
		{ID: 3, Name: "monitor", Price: 220},
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"products": items,
		"items":    items,
	})
}

func (d *demo) product(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/api/products/"))
	if err != nil || id < 1 || id > 3 {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, product{ID: id, Name: fmt.Sprintf("producto-%d", id), Price: id * 10})
}

func (d *demo) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Email == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email requerido"})
		return
	}
	token := fmt.Sprintf("token-%d", d.counter.Add(1))
	d.tokens.Store(token, body.Email)
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

func (d *demo) me(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "falta token"})
		return
	}
	token := strings.TrimPrefix(header, "Bearer ")
	email, ok := d.tokens.Load(token)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "token inválido"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"email": email.(string)})
}

func (d *demo) slow(w http.ResponseWriter, r *http.Request) {
	ms := 100
	if raw := r.URL.Query().Get("ms"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 && parsed <= 5000 {
			ms = parsed
		}
	}
	time.Sleep(time.Duration(ms) * time.Millisecond)
	writeJSON(w, http.StatusOK, map[string]int{"slept_ms": ms})
}

func (d *demo) failure(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "fallo simulado"})
}

func (d *demo) webSocket(w http.ResponseWriter, r *http.Request) {
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
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
