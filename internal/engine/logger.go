// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package engine

// Logger is the minimal logging surface used by the load engine.
type Logger interface {
	Info(msg string)
	Warn(msg string)
	Error(msg string)
	Debug(msg string)
}

// NopLogger discards every message.
type NopLogger struct{}

func (NopLogger) Info(string)  {}
func (NopLogger) Warn(string)  {}
func (NopLogger) Error(string) {}
func (NopLogger) Debug(string) {}
