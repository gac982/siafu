// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package httpx

import (
	"encoding/base64"
	"fmt"
	"net/textproto"
	"sort"
	"strings"
	"time"

	"github.com/gac982/siafu/internal/config"
	"github.com/gac982/siafu/internal/vars"
)

// ExtractSource identifies where a variable is read from.
type ExtractSource int

const (
	// ExtractJSON reads a dotted path from a JSON body.
	ExtractJSON ExtractSource = iota
	// ExtractHeader reads a response header.
	ExtractHeader
)

// ExtractSpec is one compiled extraction rule.
type ExtractSpec struct {
	Source ExtractSource
	Path   string
}

// PreparedStep is immutable and shared by every virtual user.
type PreparedStep struct {
	Name    string
	Method  string
	URL     vars.Template
	Headers map[string]vars.Template
	Body    vars.Template
	Expect  config.Expect
	Extract map[string]ExtractSpec
	WS      *PreparedWebSocket
}

// PreparedWebSocket is one compiled WebSocket action.
type PreparedWebSocket struct {
	Name         string
	Action       string
	Connection   string
	URL          vars.Template
	Headers      map[string]vars.Template
	Text         vars.Template
	Binary       []byte
	Timeout      time.Duration
	Subprotocols []string
	Expect       config.WebSocketExpect
	CloseCode    int
	CloseReason  string
}

// Scenario is a compiled scenario.
type Scenario struct {
	Name  string
	Steps []PreparedStep
}

// Plan is the compiled form of the configuration.
type Plan struct {
	Scenarios []Scenario
}

// Prepare compiles every template once, before the load starts.
func Prepare(cfg *config.Config) (*Plan, error) {
	targetHeaders, err := compileHeaders(cfg.Target.Headers)
	if err != nil {
		return nil, fmt.Errorf("target.headers: %w", err)
	}

	plan := &Plan{Scenarios: make([]Scenario, 0, len(cfg.Scenarios))}
	for _, rawScenario := range cfg.Scenarios {
		scenario := Scenario{Name: rawScenario.Name, Steps: make([]PreparedStep, 0, len(rawScenario.Steps))}
		for _, rawStep := range rawScenario.Steps {
			step, err := prepareStep(rawStep, targetHeaders)
			if err != nil {
				return nil, fmt.Errorf("scenario %q step %q: %w", rawScenario.Name, rawStep.Name, err)
			}
			scenario.Steps = append(scenario.Steps, step)
		}
		plan.Scenarios = append(plan.Scenarios, scenario)
	}
	return plan, nil
}

func prepareStep(raw config.Step, targetHeaders map[string]vars.Template) (PreparedStep, error) {
	if raw.WebSocket != nil {
		ws, err := prepareWebSocket(raw, targetHeaders)
		if err != nil {
			return PreparedStep{}, err
		}
		return PreparedStep{Name: raw.Name, WS: ws}, nil
	}

	urlTemplate, err := vars.Compile(raw.Request.URL)
	if err != nil {
		return PreparedStep{}, fmt.Errorf("url: %w", err)
	}
	bodyTemplate, err := vars.Compile(raw.Request.Body)
	if err != nil {
		return PreparedStep{}, fmt.Errorf("body: %w", err)
	}

	merged := cloneHeaders(targetHeaders)
	for name, value := range raw.Request.Headers {
		tpl, err := vars.Compile(value)
		if err != nil {
			return PreparedStep{}, fmt.Errorf("header %s: %w", name, err)
		}
		merged[textproto.CanonicalMIMEHeaderKey(name)] = tpl
	}

	extract, err := compileExtract(raw.Extract)
	if err != nil {
		return PreparedStep{}, err
	}

	return PreparedStep{
		Name:    raw.Name,
		Method:  raw.Request.Method,
		URL:     urlTemplate,
		Headers: merged,
		Body:    bodyTemplate,
		Expect:  raw.Expect,
		Extract: extract,
	}, nil
}

func compileHeaders(headers map[string]string) (map[string]vars.Template, error) {
	compiled := make(map[string]vars.Template, len(headers))
	for name, value := range headers {
		tpl, err := vars.Compile(value)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		compiled[textproto.CanonicalMIMEHeaderKey(name)] = tpl
	}
	return compiled, nil
}

func cloneHeaders(source map[string]vars.Template) map[string]vars.Template {
	clone := make(map[string]vars.Template, len(source))
	for name, tpl := range source {
		clone[name] = tpl
	}
	return clone
}

func compileExtract(raw map[string]string) (map[string]ExtractSpec, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	specs := make(map[string]ExtractSpec, len(raw))
	for key, value := range raw {
		switch {
		case strings.HasPrefix(value, "json:"):
			specs[key] = ExtractSpec{Source: ExtractJSON, Path: strings.TrimPrefix(value, "json:")}
		case strings.HasPrefix(value, "header:"):
			specs[key] = ExtractSpec{Source: ExtractHeader, Path: strings.TrimPrefix(value, "header:")}
		default:
			return nil, fmt.Errorf("extract %s: use json:<ruta> o header:<Nombre>", key)
		}
	}
	return specs, nil
}

// sortedExtractKeys keeps extraction deterministic when more than one
// variable is defined on the same step.
func sortedExtractKeys(specs map[string]ExtractSpec) []string {
	keys := make([]string, 0, len(specs))
	for key := range specs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func prepareWebSocket(raw config.Step, targetHeaders map[string]vars.Template) (*PreparedWebSocket, error) {
	ws := raw.WebSocket
	action := strings.ToLower(strings.TrimSpace(ws.Action))
	urlTemplate, err := vars.Compile(ws.URL)
	if err != nil {
		return nil, fmt.Errorf("websocket.url: %w", err)
	}
	textTemplate, err := vars.Compile(ws.Text)
	if err != nil {
		return nil, fmt.Errorf("websocket.text: %w", err)
	}

	headers := cloneHeaders(targetHeaders)
	for name, value := range ws.Headers {
		tpl, err := vars.Compile(value)
		if err != nil {
			return nil, fmt.Errorf("websocket.header %s: %w", name, err)
		}
		headers[textproto.CanonicalMIMEHeaderKey(name)] = tpl
	}

	var binary []byte
	if ws.BinaryBase64 != "" {
		binary, err = base64.StdEncoding.DecodeString(ws.BinaryBase64)
		if err != nil {
			return nil, fmt.Errorf("websocket.binary_base64: %w", err)
		}
	}

	return &PreparedWebSocket{
		Name:         raw.Name,
		Action:       action,
		Connection:   ws.Connection,
		URL:          urlTemplate,
		Headers:      headers,
		Text:         textTemplate,
		Binary:       binary,
		Timeout:      ws.Timeout.Value(),
		Subprotocols: ws.Subprotocols,
		Expect:       ws.Expect,
		CloseCode:    ws.CloseCode,
		CloseReason:  ws.CloseReason,
	}, nil
}
