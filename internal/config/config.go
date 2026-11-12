// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

// Package config defines the YAML schema, its defaults and validation.
package config

// Config is the root of a siafu test file.
type Config struct {
	Version    int               `yaml:"version"`
	Name       string            `yaml:"name"`
	Target     Target            `yaml:"target"`
	Load       LoadConfig        `yaml:"load"`
	Vars       map[string]string `yaml:"vars"`
	Scenarios  []Scenario        `yaml:"scenarios"`
	Thresholds Thresholds        `yaml:"thresholds"`
	Limits     Limits            `yaml:"limits"`

	sourceDir string
}

// Target describes the destination shared by every scenario.
type Target struct {
	BaseURL     string            `yaml:"base_url"`
	Timeout     Duration          `yaml:"timeout"`
	HTTPVersion string            `yaml:"http_version"`
	Headers     map[string]string `yaml:"headers"`
	TLS         TLSConfig         `yaml:"tls"`
}

// HTTP versions accepted by target.http_version.
const (
	HTTPVersionAuto = "auto"
	HTTPVersion11   = "1.1"
	HTTPVersion2    = "2"
	HTTPVersion3    = "3"
)

// TLSConfig controls certificate verification for the target.
type TLSConfig struct {
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
	CAFile             string `yaml:"ca_file"`
}

// LoadConfig describes stages, think time and safety limits.
type LoadConfig struct {
	Mode      string         `yaml:"mode"`
	Stages    []Stage        `yaml:"stages"`
	ThinkTime ThinkTime      `yaml:"think_time"`
	Seed      *int64         `yaml:"seed"`
	MaxVUs    int            `yaml:"max_vus"`
	OpenLoop  OpenLoopConfig `yaml:"open_loop"`
}

// Load modes.
const (
	ModeClosed = "closed"
	ModeOpen   = "open"
)

// Overload policies.
const (
	OverloadDrop = "drop"
)

// Analysis exhaustion policies.
const (
	AnalysisSkip = "skip"
	AnalysisFail = "fail"
)

// OpenLoopConfig describes the arrival-rate scheduler.
type OpenLoopConfig struct {
	Stages              []RateStage `yaml:"stages"`
	MaxInFlight         int         `yaml:"max_in_flight"`
	OnOverload          string      `yaml:"on_overload"`
	OnAnalysisExhausted string      `yaml:"on_analysis_exhausted"`
}

// RateStage is one segment of the linear arrival-rate ramp.
type RateStage struct {
	Duration   Duration `yaml:"duration"`
	TargetRate float64  `yaml:"target_rate"`
}

// Stage is one segment of the linear virtual-user ramp.
type Stage struct {
	Duration  Duration `yaml:"duration"`
	TargetVUs int      `yaml:"target_vus"`
}

// Scenario is an ordered flow executed by a virtual user.
type Scenario struct {
	Name  string `yaml:"name"`
	Steps []Step `yaml:"steps"`
}

// Step is one HTTP request or WebSocket action in a scenario.
type Step struct {
	Name      string            `yaml:"name"`
	Request   Request           `yaml:"request"`
	WebSocket *WebSocketStep    `yaml:"websocket"`
	Expect    Expect            `yaml:"expect"`
	Extract   map[string]string `yaml:"extract"`
}

// WebSocket actions.
const (
	WSActionConnect = "connect"
	WSActionSend    = "send"
	WSActionReceive = "receive"
	WSActionClose   = "close"
)

// WebSocketStep describes one action on a named per-VU connection.
type WebSocketStep struct {
	Action       string            `yaml:"action"`
	URL          string            `yaml:"url"`
	Connection   string            `yaml:"connection"`
	Headers      map[string]string `yaml:"headers"`
	Subprotocols []string          `yaml:"subprotocols"`
	Text         string            `yaml:"text"`
	BinaryBase64 string            `yaml:"binary_base64"`
	Timeout      Duration          `yaml:"timeout"`
	Expect       WebSocketExpect   `yaml:"expect"`
	CloseCode    int               `yaml:"close_code"`
	CloseReason  string            `yaml:"close_reason"`
}

// WebSocketExpect holds the checks of a WebSocket action.
type WebSocketExpect struct {
	Status          StatusCodes `yaml:"status"`
	MessageContains string      `yaml:"message_contains"`
	MaxDuration     Duration    `yaml:"max_duration"`
}

// Request describes the HTTP request to execute.
type Request struct {
	Method  string            `yaml:"method"`
	URL     string            `yaml:"url"`
	Headers map[string]string `yaml:"headers"`
	Body    string            `yaml:"body"`
}

// Expect holds the checks evaluated for one response.
type Expect struct {
	Status       StatusCodes `yaml:"status"`
	BodyContains string      `yaml:"body_contains"`
	MaxDuration  Duration    `yaml:"max_duration"`
}

// Limits are the process-wide safety budgets.
type Limits struct {
	MemoryBudgetMB   int `yaml:"memory_budget_mb"`
	WarnAtPercent    int `yaml:"warn_at_percent"`
	AnalysisBudgetMB int `yaml:"analysis_budget_mb"`
	MaxLogLines      int `yaml:"max_log_lines"`
	MaxBatchSamples  int `yaml:"max_batch_samples"`
	MaxIdleConns     int `yaml:"max_idle_conns"`
}

// Thresholds are the acceptance criteria of the test. Nil means "not set".
type Thresholds struct {
	LatencyP95 *Duration `yaml:"latency_p95"`
	LatencyP99 *Duration `yaml:"latency_p99"`
	ErrorRate  *float64  `yaml:"error_rate"`
	MinRPS     *float64  `yaml:"min_rps"`
}
