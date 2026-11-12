// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package config

import (
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"math"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gac982/siafu/internal/vars"
)

var (
	namePattern  = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	identPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

var validMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "PATCH": true,
	"DELETE": true, "HEAD": true, "OPTIONS": true,
}

type validator struct {
	problems []string
}

func (v *validator) addf(format string, args ...any) {
	v.problems = append(v.problems, fmt.Sprintf(format, args...))
}

// Validate checks the configuration and returns every problem it finds.
func (c *Config) Validate() error {
	v := &validator{}
	v.validateRoot(c)
	if len(v.problems) == 0 {
		return nil
	}
	return &ValidationError{Problems: v.problems}
}

func (v *validator) validateRoot(c *Config) {
	if c.Version != 1 {
		v.addf("version: debe ser 1 (recibido %d)", c.Version)
	}
	if c.Name == "" {
		v.addf("name: es obligatorio")
	} else if len(c.Name) > 80 {
		v.addf("name: no puede superar 80 caracteres")
	}

	v.validateTarget(c.Target, c.Vars)
	v.validateLoad(c.Load)
	v.validateVars(c.Vars)
	v.validateScenarios(c)
	v.validateThresholds(c.Thresholds)
	v.validateLimits(c.Limits)
}

func (v *validator) validateTarget(t Target, varsMap map[string]string) {
	baseOK := false
	if t.BaseURL == "" {
		v.addf("target.base_url: es obligatorio")
	} else if strings.Contains(t.BaseURL, "${") {
		v.addf("target.base_url: no admite plantillas")
	} else if u, err := url.Parse(t.BaseURL); err != nil {
		v.addf("target.base_url: URL inválida: %v", err)
	} else if u.Scheme != "http" && u.Scheme != "https" {
		v.addf("target.base_url: el esquema debe ser http o https")
	} else if u.Host == "" {
		v.addf("target.base_url: falta el host")
	} else if u.RawQuery != "" || u.Fragment != "" {
		v.addf("target.base_url: no puede incluir query ni fragmento")
	} else {
		baseOK = true
	}
	_ = baseOK

	if d := t.Timeout.Value(); d < time.Second || d > 300*time.Second {
		v.addf("target.timeout: debe estar entre 1s y 300s (recibido %s)", d)
	}
	version := t.HTTPVersion
	if version == "" {
		version = HTTPVersionAuto
	}
	switch version {
	case HTTPVersionAuto, HTTPVersion11:
	case HTTPVersion2:
		if u, err := url.Parse(t.BaseURL); err == nil && u.Scheme == "http" {
			v.addf("target.http_version: \"2\" requiere https (h2c no soportado)")
		}
	case HTTPVersion3:
		if u, err := url.Parse(t.BaseURL); err == nil && u.Scheme == "http" {
			v.addf("target.http_version: \"3\" requiere https (h3c no soportado)")
		}
	default:
		v.addf("target.http_version: debe ser auto, 1.1, 2 o 3 (recibido %q)", version)
	}
	v.validateHeaders("target.headers", t.Headers)
	available := map[string]bool{"VU": true, "ITER": true}
	for key := range varsMap {
		available[key] = true
	}
	for name, value := range t.Headers {
		v.validateTemplate("target.headers."+name, value, available)
	}
	v.validateCAFile(t.TLS.CAFile)
}

func (v *validator) validateCAFile(path string) {
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		v.addf("target.tls.ca_file: no se puede leer %s: %v", path, err)
		return
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(data) {
		v.addf("target.tls.ca_file: %s no contiene un certificado PEM válido", path)
	}
}

func (v *validator) validateLoad(l LoadConfig) {
	switch l.Mode {
	case ModeClosed:
		v.validateClosedStages(l)
	case ModeOpen:
		v.validateOpenStages(l.OpenLoop)
	default:
		v.addf("load.mode: debe ser closed u open (recibido %q)", l.Mode)
	}

	if l.MaxVUs < 1 || l.MaxVUs > 20000 {
		v.addf("load.max_vus: debe estar entre 1 y 20000 (recibido %d)", l.MaxVUs)
	}

	if l.ThinkTime.Min.Value() < 0 {
		v.addf("load.think_time.min: no puede ser negativo")
	}
	if l.ThinkTime.Max.Value() < l.ThinkTime.Min.Value() {
		v.addf("load.think_time: max debe ser mayor o igual que min")
	}
	if l.ThinkTime.Max.Value() > time.Hour {
		v.addf("load.think_time.max: no puede superar 1h")
	}
}

func (v *validator) validateClosedStages(l LoadConfig) {
	if len(l.Stages) == 0 {
		v.addf("load.stages: se requiere al menos una etapa")
	} else if len(l.Stages) > 50 {
		v.addf("load.stages: no puede haber más de 50 etapas")
	}

	total := time.Duration(0)
	positive := false
	for i, stage := range l.Stages {
		if stage.Duration.Value() <= 0 {
			v.addf("load.stages[%d].duration: debe ser mayor que 0", i)
		}
		total += stage.Duration.Value()
		if stage.TargetVUs < 0 {
			v.addf("load.stages[%d].target_vus: no puede ser negativo", i)
		} else if stage.TargetVUs > l.MaxVUs {
			v.addf("load.stages[%d].target_vus: %d supera load.max_vus (%d)", i, stage.TargetVUs, l.MaxVUs)
		}
		if stage.TargetVUs > 0 {
			positive = true
		}
	}
	if len(l.Stages) > 0 && !positive {
		v.addf("load.stages: al menos una etapa debe tener target_vus mayor que 0")
	}
	if total > 6*time.Hour {
		v.addf("load.stages: la duración total no puede superar 6h")
	}
}

func (v *validator) validateOpenStages(o OpenLoopConfig) {
	if len(o.Stages) == 0 {
		v.addf("load.open_loop.stages: se requiere al menos una etapa")
	} else if len(o.Stages) > 50 {
		v.addf("load.open_loop.stages: no puede haber más de 50 etapas")
	}

	total := time.Duration(0)
	positive := false
	for i, stage := range o.Stages {
		if stage.Duration.Value() <= 0 {
			v.addf("load.open_loop.stages[%d].duration: debe ser mayor que 0", i)
		}
		total += stage.Duration.Value()
		if stage.TargetRate < 0 || math.IsNaN(stage.TargetRate) || math.IsInf(stage.TargetRate, 0) || stage.TargetRate > 1_000_000 {
			v.addf("load.open_loop.stages[%d].target_rate: debe estar entre 0 y 1000000", i)
		}
		if stage.TargetRate > 0 {
			positive = true
		}
	}
	if len(o.Stages) > 0 && !positive {
		v.addf("load.open_loop.stages: al menos una etapa debe tener target_rate mayor que 0")
	}
	if total > 6*time.Hour {
		v.addf("load.open_loop.stages: la duración total no puede superar 6h")
	}
	if o.MaxInFlight < 1 || o.MaxInFlight > 20000 {
		v.addf("load.open_loop.max_in_flight: debe estar entre 1 y 20000 (recibido %d)", o.MaxInFlight)
	}
	if o.OnOverload != OverloadDrop {
		v.addf("load.open_loop.on_overload: solo se admite %q", OverloadDrop)
	}
	if o.OnAnalysisExhausted != AnalysisSkip && o.OnAnalysisExhausted != AnalysisFail {
		v.addf("load.open_loop.on_analysis_exhausted: debe ser %q o %q", AnalysisSkip, AnalysisFail)
	}
}

func (v *validator) validateVars(varsMap map[string]string) {
	for key, raw := range varsMap {
		if !identPattern.MatchString(key) {
			v.addf("vars.%s: nombre inválido", key)
			continue
		}
		if key == "VU" || key == "ITER" || key == "ENV" {
			v.addf("vars.%s: es un nombre reservado", key)
			continue
		}
		tpl, err := vars.Compile(raw)
		if err != nil {
			v.addf("vars.%s: %v", key, err)
			continue
		}
		for _, ref := range tpl.Refs() {
			if ref.Kind == vars.KindVar {
				v.addf("vars.%s: no puede referenciar otras variables (\"${%s}\")", key, ref.Name)
			}
		}
	}
}

func (v *validator) validateScenarios(c *Config) {
	if len(c.Scenarios) == 0 {
		v.addf("scenarios: se requiere al menos un escenario")
		return
	}
	if len(c.Scenarios) > 20 {
		v.addf("scenarios: no puede haber más de 20 escenarios")
	}

	seenScenarios := map[string]bool{}
	for si, scenario := range c.Scenarios {
		field := fmt.Sprintf("scenarios[%d]", si)
		if scenario.Name == "" {
			v.addf("%s.name: es obligatorio", field)
		} else if len(scenario.Name) > 60 || !namePattern.MatchString(scenario.Name) {
			v.addf("%s.name: use entre 1 y 60 caracteres [A-Za-z0-9_-]", field)
		} else if seenScenarios[scenario.Name] {
			v.addf("%s.name: nombre duplicado %q", field, scenario.Name)
		}
		seenScenarios[scenario.Name] = true

		if len(scenario.Steps) == 0 {
			v.addf("%s.steps: se requiere al menos un paso", field)
			continue
		}
		if len(scenario.Steps) > 50 {
			v.addf("%s.steps: no puede haber más de 50 pasos", field)
		}

		available := map[string]bool{"VU": true, "ITER": true}
		for key := range c.Vars {
			available[key] = true
		}

		seenSteps := map[string]bool{}
		seenExtracts := map[string]bool{}
		for pi := range scenario.Steps {
			step := &scenario.Steps[pi]
			stepField := fmt.Sprintf("%s.steps[%d]", field, pi)
			if step.Name == "" {
				v.addf("%s.name: es obligatorio", stepField)
			} else if len(step.Name) > 60 || !namePattern.MatchString(step.Name) {
				v.addf("%s.name: use entre 1 y 60 caracteres [A-Za-z0-9_-]", stepField)
			} else if seenSteps[step.Name] {
				v.addf("%s.name: nombre duplicado %q", stepField, step.Name)
			}
			seenSteps[step.Name] = true

			if step.WebSocket != nil {
				if requestSet(step.Request) {
					v.addf("%s: request y websocket son mutuamente excluyentes", stepField)
				}
				if httpExpectSet(step.Expect) {
					v.addf("%s.expect: usa websocket.expect; request y websocket son mutuamente excluyentes", stepField)
				}
				if len(step.Extract) > 0 {
					v.addf("%s.extract: no soportado en websocket v1", stepField)
				}
				v.validateWebSocket(stepField, *step.WebSocket, available)
				continue
			}

			step.Request.Method = strings.ToUpper(strings.TrimSpace(step.Request.Method))
			if !validMethods[step.Request.Method] {
				v.addf("%s.request.method: método no permitido %q", stepField, step.Request.Method)
			}
			if step.Request.URL == "" {
				v.addf("%s.request.url: es obligatorio", stepField)
			} else {
				v.validateTemplate(stepField+".request.url", step.Request.URL, available)
			}
			v.validateHeaders(stepField+".request.headers", step.Request.Headers)
			for name := range step.Request.Headers {
				v.validateTemplate(stepField+".request.headers."+name, step.Request.Headers[name], available)
			}
			v.validateTemplate(stepField+".request.body", step.Request.Body, available)

			for _, code := range step.Expect.Status {
				if code < 100 || code > 599 {
					v.addf("%s.expect.status: código fuera de rango: %d", stepField, code)
				}
			}
			if step.Expect.MaxDuration.Value() < 0 {
				v.addf("%s.expect.max_duration: no puede ser negativo", stepField)
			}

			for key, spec := range step.Extract {
				if !identPattern.MatchString(key) {
					v.addf("%s.extract.%s: nombre inválido", stepField, key)
					continue
				}
				if _, ok := c.Vars[key]; ok {
					v.addf("%s.extract.%s: colisiona con una variable global", stepField, key)
				}
				if seenExtracts[key] {
					v.addf("%s.extract.%s: extracción duplicada en el escenario", stepField, key)
				}
				if !validExtractSpec(spec) {
					v.addf("%s.extract.%s: use json:<ruta> o header:<Nombre>", stepField, key)
					continue
				}
				seenExtracts[key] = true
				available[key] = true
			}
		}
	}
}

func (v *validator) validateHeaders(field string, headers map[string]string) {
	for name, value := range headers {
		if !validHeaderName(name) {
			v.addf("%s: nombre de cabecera inválido %q", field, name)
		}
		if strings.ContainsAny(value, "\r\n") {
			v.addf("%s.%s: el valor no puede contener CR/LF", field, name)
		}
	}
}

func (v *validator) validateTemplate(field, raw string, available map[string]bool) {
	tpl, err := vars.Compile(raw)
	if err != nil {
		v.addf("%s: %v", field, err)
		return
	}
	for _, ref := range tpl.Refs() {
		if ref.Kind == vars.KindVar && !available[ref.Name] {
			v.addf("%s: referencia desconocida \"${%s}\"", field, ref.Name)
		}
	}
}

func (v *validator) validateLimits(l Limits) {
	if l.MemoryBudgetMB < 1 || l.MemoryBudgetMB > 1024 {
		v.addf("limits.memory_budget_mb: debe estar entre 1 y 1024")
	}
	if l.WarnAtPercent < 50 || l.WarnAtPercent > 95 {
		v.addf("limits.warn_at_percent: debe estar entre 50 y 95")
	}
	if l.AnalysisBudgetMB < 1 || l.AnalysisBudgetMB > 128 {
		v.addf("limits.analysis_budget_mb: debe estar entre 1 y 128")
	}
	if l.MaxLogLines < 100 || l.MaxLogLines > 10000 {
		v.addf("limits.max_log_lines: debe estar entre 100 y 10000")
	}
	if l.MaxBatchSamples < 1 || l.MaxBatchSamples > 512 {
		v.addf("limits.max_batch_samples: debe estar entre 1 y 512")
	}
	if l.MaxIdleConns < 1 || l.MaxIdleConns > 4096 {
		v.addf("limits.max_idle_conns: debe estar entre 1 y 4096")
	}
}

func (v *validator) validateThresholds(t Thresholds) {
	if t.LatencyP95 != nil && t.LatencyP95.Value() <= 0 {
		v.addf("thresholds.latency_p95: debe ser mayor que 0")
	}
	if t.LatencyP99 != nil && t.LatencyP99.Value() <= 0 {
		v.addf("thresholds.latency_p99: debe ser mayor que 0")
	}
	if t.LatencyP95 != nil && t.LatencyP99 != nil && t.LatencyP99.Value() < t.LatencyP95.Value() {
		v.addf("thresholds.latency_p99: debe ser mayor o igual que latency_p95")
	}
	if t.ErrorRate != nil && (*t.ErrorRate < 0 || *t.ErrorRate > 1) {
		v.addf("thresholds.error_rate: debe estar entre 0 y 1")
	}
	if t.MinRPS != nil && *t.MinRPS < 0 {
		v.addf("thresholds.min_rps: no puede ser negativo")
	}
}

func validExtractSpec(spec string) bool {
	if strings.HasPrefix(spec, "json:") {
		return strings.TrimSpace(strings.TrimPrefix(spec, "json:")) != ""
	}
	if strings.HasPrefix(spec, "header:") {
		return validHeaderName(strings.TrimPrefix(spec, "header:"))
	}
	return false
}

func validHeaderName(name string) bool {
	if name == "" {
		return false
	}
	const extra = "!#$%&'*+-.^_`|~"
	for _, r := range name {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			continue
		}
		if !strings.ContainsRune(extra, r) {
			return false
		}
	}
	return true
}

func requestSet(r Request) bool {
	return r.Method != "" || r.URL != "" || r.Body != "" || len(r.Headers) > 0
}

func httpExpectSet(e Expect) bool {
	return len(e.Status) > 0 || e.BodyContains != "" || e.MaxDuration.Value() != 0
}

func (v *validator) validateWebSocket(field string, ws WebSocketStep, available map[string]bool) {
	action := strings.ToLower(strings.TrimSpace(ws.Action))
	switch action {
	case WSActionConnect, WSActionSend, WSActionReceive, WSActionClose:
	default:
		v.addf("%s.websocket.action: debe ser connect, send, receive o close (recibido %q)", field, ws.Action)
		return
	}

	if ws.Connection == "" || len(ws.Connection) > 32 || !namePattern.MatchString(ws.Connection) {
		v.addf("%s.websocket.connection: use entre 1 y 32 caracteres [A-Za-z0-9_-]", field)
	}
	if d := ws.Timeout.Value(); d != 0 && (d < time.Second || d > 300*time.Second) {
		v.addf("%s.websocket.timeout: debe estar entre 1s y 300s (recibido %s)", field, d)
	}

	if action == WSActionConnect {
		if ws.URL == "" {
			v.addf("%s.websocket.url: es obligatorio en connect", field)
		} else {
			v.validateTemplate(field+".websocket.url", ws.URL, available)
		}
		if ws.Expect.MessageContains != "" {
			v.addf("%s.websocket.expect.message_contains: solo se permite en receive", field)
		}
		v.validateHeaders(field+".websocket.headers", ws.Headers)
		for name := range ws.Headers {
			v.validateTemplate(field+".websocket.headers."+name, ws.Headers[name], available)
		}
		for _, proto := range ws.Subprotocols {
			if strings.TrimSpace(proto) == "" {
				v.addf("%s.websocket.subprotocols: no puede contener valores vacíos", field)
			}
		}
	} else {
		if ws.URL != "" {
			v.addf("%s.websocket.url: solo se permite en connect", field)
		}
		if len(ws.Headers) > 0 {
			v.addf("%s.websocket.headers: solo se permiten en connect", field)
		}
		if len(ws.Subprotocols) > 0 {
			v.addf("%s.websocket.subprotocols: solo se permiten en connect", field)
		}
	}

	if len(ws.Expect.Status) > 0 {
		if action != WSActionConnect {
			v.addf("%s.websocket.expect.status: solo se permite en connect", field)
		}
		for _, code := range ws.Expect.Status {
			if code < 100 || code > 599 {
				v.addf("%s.websocket.expect.status: código fuera de rango: %d", field, code)
			}
		}
	}
	if ws.Expect.MessageContains != "" && action != WSActionReceive {
		v.addf("%s.websocket.expect.message_contains: solo se permite en receive", field)
	}
	if ws.Expect.MaxDuration.Value() < 0 {
		v.addf("%s.websocket.expect.max_duration: no puede ser negativo", field)
	}

	if action == WSActionSend {
		textSet := ws.Text != ""
		binarySet := ws.BinaryBase64 != ""
		if textSet == binarySet {
			v.addf("%s.websocket: send requiere exactamente uno de text o binary_base64", field)
		}
		if textSet {
			v.validateTemplate(field+".websocket.text", ws.Text, available)
		}
		if binarySet {
			if _, err := base64.StdEncoding.DecodeString(ws.BinaryBase64); err != nil {
				v.addf("%s.websocket.binary_base64: base64 inválido: %v", field, err)
			}
		}
	} else if ws.Text != "" || ws.BinaryBase64 != "" {
		v.addf("%s.websocket: text/binary_base64 solo se permiten en send", field)
	}

	if action == WSActionClose {
		if ws.CloseCode != 0 && (ws.CloseCode < 1000 || ws.CloseCode > 4999) {
			v.addf("%s.websocket.close_code: debe estar entre 1000 y 4999", field)
		}
	} else if ws.CloseCode != 0 || ws.CloseReason != "" {
		v.addf("%s.websocket.close_code: solo se permite en close", field)
	}
	if ws.CloseReason != "" && action != WSActionClose {
		v.addf("%s.websocket.close_reason: solo se permite en close", field)
	}
}
