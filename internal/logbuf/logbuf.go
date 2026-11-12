// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

// Package logbuf keeps a bounded, redacted log ring for the TUI and plain mode.
package logbuf

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/gac982/siafu/internal/redact"
	"github.com/gac982/siafu/internal/safetext"
)

const maxErrorsPerSecond = 5

// Buffer implements engine.Logger and keeps the last N lines.
type Buffer struct {
	mu       sync.Mutex
	capacity int
	lines    []string
	head     int
	size     int
	redactor *redact.Redactor
	plain    io.Writer

	second     int64
	errorCount int
	dropped    int
}

// New creates a ring with the given capacity (1000 when non-positive).
func New(capacity int) *Buffer {
	if capacity <= 0 {
		capacity = 1000
	}
	return &Buffer{capacity: capacity, lines: make([]string, capacity)}
}

// SetRedactor makes every buffered line pass through the redactor.
func (b *Buffer) SetRedactor(redactor *redact.Redactor) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.redactor = redactor
}

// SetPlain mirrors log lines to w, applying redactor (used in --no-tui).
func (b *Buffer) SetPlain(w io.Writer, redactor *redact.Redactor) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.plain = w
	b.redactor = redactor
}

// Info logs an informational line.
func (b *Buffer) Info(msg string) { b.log("INFO", msg) }

// Warn logs a warning.
func (b *Buffer) Warn(msg string) { b.log("WARN", msg) }

// Error logs an error, rate-limited to five per second.
func (b *Buffer) Error(msg string) { b.log("ERROR", msg) }

// Debug logs a debug line.
func (b *Buffer) Debug(msg string) { b.log("DEBUG", msg) }

// Lines returns a copy of the current buffer.
func (b *Buffer) Lines() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]string, 0, b.size)
	for i := 0; i < b.size; i++ {
		out = append(out, b.lines[(b.head+i)%b.capacity])
	}
	return out
}

func (b *Buffer) log(level, msg string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if level == "ERROR" {
		second := time.Now().Unix()
		if second != b.second {
			if b.dropped > 0 {
				b.appendLocked("WARN", fmt.Sprintf("%d errores similares omitidos", b.dropped))
				b.dropped = 0
			}
			b.second = second
			b.errorCount = 0
		}
		if b.errorCount >= maxErrorsPerSecond {
			b.dropped++
			return
		}
		b.errorCount++
	}

	b.appendLocked(level, msg)
}

func (b *Buffer) appendLocked(level, msg string) {
	if b.redactor != nil {
		msg = b.redactor.Message(msg)
	}
	line := safetext.Line(fmt.Sprintf("%s %-5s %s", time.Now().Format("15:04:05"), level, msg))

	if b.size < b.capacity {
		b.lines[(b.head+b.size)%b.capacity] = line
		b.size++
	} else {
		b.lines[b.head] = line
		b.head = (b.head + 1) % b.capacity
	}
	if b.plain != nil {
		_, _ = fmt.Fprintln(b.plain, line)
	}
}
