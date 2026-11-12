// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Decode parses a YAML document, applies defaults and returns the raw
// configuration. Semantic validation is performed by Validate.
func Decode(r io.Reader) (*Config, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("leer configuración: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, errors.New("el documento YAML está vacío")
	}

	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)

	var cfg Config
	if err := dec.Decode(&cfg); err != nil {
		return nil, err
	}

	var extra any
	if err := dec.Decode(&extra); err == nil {
		return nil, errors.New("se encontró más de un documento YAML; el archivo debe contener uno solo")
	} else if !errors.Is(err, io.EOF) {
		return nil, err
	}

	cfg.applyDefaults()
	return &cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Target.Timeout.Duration == 0 {
		c.Target.Timeout.Duration = 30 * time.Second
	}
	if c.Target.HTTPVersion == "" {
		c.Target.HTTPVersion = HTTPVersionAuto
	}
	if c.Load.MaxVUs == 0 {
		c.Load.MaxVUs = 2000
	}
	if c.Load.Mode == "" {
		c.Load.Mode = ModeClosed
	}
	if c.Load.OpenLoop.OnOverload == "" {
		c.Load.OpenLoop.OnOverload = OverloadDrop
	}
	if c.Load.OpenLoop.OnAnalysisExhausted == "" {
		c.Load.OpenLoop.OnAnalysisExhausted = AnalysisSkip
	}
	if c.Limits.MemoryBudgetMB == 0 {
		c.Limits.MemoryBudgetMB = 450
	}
	if c.Limits.WarnAtPercent == 0 {
		c.Limits.WarnAtPercent = 80
	}
	if c.Limits.AnalysisBudgetMB == 0 {
		c.Limits.AnalysisBudgetMB = 64
	}
	if c.Limits.MaxLogLines == 0 {
		c.Limits.MaxLogLines = 1000
	}
	if c.Limits.MaxBatchSamples == 0 {
		c.Limits.MaxBatchSamples = 64
	}
	if c.Limits.MaxIdleConns == 0 {
		c.Limits.MaxIdleConns = 1024
	}
	for si := range c.Scenarios {
		for pi := range c.Scenarios[si].Steps {
			step := &c.Scenarios[si].Steps[pi]
			if step.WebSocket != nil {
				if step.WebSocket.Connection == "" {
					step.WebSocket.Connection = "default"
				}
				switch strings.ToLower(strings.TrimSpace(step.WebSocket.Action)) {
				case WSActionConnect:
					if len(step.WebSocket.Expect.Status) == 0 {
						step.WebSocket.Expect.Status = StatusCodes{101}
					}
				case WSActionClose:
					if step.WebSocket.CloseCode == 0 {
						step.WebSocket.CloseCode = 1000
					}
				}
				continue
			}
			if step.Request.Method == "" {
				step.Request.Method = "GET"
			}
		}
	}
}
