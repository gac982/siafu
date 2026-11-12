// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

// Package web serves the local results UI and its read-only JSON API.
package web

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gac982/siafu/internal/results"
)

// Server serves the results UI and API.
type Server struct {
	store  *results.Store
	assets fs.FS
}

// NewServer creates a server for store and the given asset filesystem.
func NewServer(store *results.Store, assets fs.FS) *Server {
	return &Server{store: store, assets: assets}
}

// Handler returns the HTTP handler with manual routing.
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "método no permitido", http.StatusMethodNotAllowed)
			return
		}

		switch path := r.URL.Path; {
		case path == "/api/health":
			s.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		case path == "/api/runs":
			s.handleRuns(w, r)
		case strings.HasPrefix(path, "/api/runs/"):
			rest := strings.TrimPrefix(path, "/api/runs/")
			if strings.HasSuffix(rest, "/series") {
				s.handleSeries(w, strings.TrimSuffix(rest, "/series"))
			} else {
				s.handleRun(w, rest)
			}
		case path == "/":
			s.handleAsset(w, "index.html", "text/html; charset=utf-8")
		case path == "/app.js":
			s.handleAsset(w, "app.js", "text/javascript; charset=utf-8")
		case path == "/style.css":
			s.handleAsset(w, "style.css", "text/css; charset=utf-8")
		case path == "/charts.js":
			s.handleAsset(w, "charts.js", "text/javascript; charset=utf-8")
		case path == "/format.js":
			s.handleAsset(w, "format.js", "text/javascript; charset=utf-8")
		case path == "/logo.png":
			s.handleAsset(w, "logo.png", "image/png")
		case path == "/mark.png":
			s.handleAsset(w, "mark.png", "image/png")
		case path == "/favicon.png":
			s.handleAsset(w, "favicon.png", "image/png")
		default:
			http.NotFound(w, r)
		}
	})
}

func (s *Server) handleRuns(w http.ResponseWriter, r *http.Request) {
	entries, err := s.store.List()
	if err != nil {
		http.Error(w, "error leyendo el índice", http.StatusInternalServerError)
		return
	}
	query := r.URL.Query()
	mode := query.Get("mode")
	verdict := query.Get("verdict")
	search := query.Get("q")

	limit := 0
	if raw := query.Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			http.Error(w, "limit inválido", http.StatusBadRequest)
			return
		}
		limit = parsed
	}
	offset := 0
	if raw := query.Get("offset"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			http.Error(w, "offset inválido", http.StatusBadRequest)
			return
		}
		offset = parsed
	}

	filtered := make([]results.IndexEntry, 0, len(entries))
	for _, entry := range entries {
		if mode != "" && !strings.EqualFold(entry.Mode, mode) {
			continue
		}
		if verdict != "" && !strings.EqualFold(entry.Verdict, verdict) {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(entry.Name+" "+entry.Tag+" "+entry.RunID), strings.ToLower(search)) {
			continue
		}
		filtered = append(filtered, entry)
	}
	if offset >= len(filtered) {
		filtered = []results.IndexEntry{}
	} else {
		filtered = filtered[offset:]
	}
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}
	if filtered == nil {
		filtered = []results.IndexEntry{}
	}
	s.writeJSON(w, http.StatusOK, filtered)
}

func (s *Server) handleRun(w http.ResponseWriter, id string) {
	if !results.ValidID(id) {
		http.Error(w, "id inválido", http.StatusBadRequest)
		return
	}
	record, err := s.store.Load(id)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || os.IsNotExist(err) {
			http.Error(w, "corrida no encontrada", http.StatusNotFound)
			return
		}
		http.Error(w, "error leyendo la corrida", http.StatusInternalServerError)
		return
	}
	s.writeJSON(w, http.StatusOK, record)
}

func (s *Server) handleSeries(w http.ResponseWriter, id string) {
	if !results.ValidID(id) {
		http.Error(w, "id inválido", http.StatusBadRequest)
		return
	}
	points, err := s.store.Series(id)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || os.IsNotExist(err) {
			http.Error(w, "corrida no encontrada", http.StatusNotFound)
			return
		}
		http.Error(w, "error leyendo la serie", http.StatusInternalServerError)
		return
	}
	if points == nil {
		points = []results.SeriesPoint{}
	}
	s.writeJSON(w, http.StatusOK, points)
}

func (s *Server) handleAsset(w http.ResponseWriter, name, contentType string) {
	if s.assets == nil {
		http.Error(w, "assets no disponibles", http.StatusInternalServerError)
		return
	}
	data, err := fs.ReadFile(s.assets, name)
	if err != nil {
		http.Error(w, "asset no encontrado", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(data)
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
