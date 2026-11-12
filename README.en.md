# Siafu

<p><img src="assets/logo.png" width="180" alt="Siafu logo"></p>

**English** · [Español](README.md)

![Go version](https://img.shields.io/badge/go-1.27%2B-00ADD8?logo=go&logoColor=white)
![License](https://img.shields.io/badge/license-AGPL--3.0--or--later-blue)
![Platforms](https://img.shields.io/badge/platforms-linux%20%7C%20macOS-lightgrey)
![Status](https://img.shields.io/badge/status-v0.4.1-orange)

**Terminal-based load testing, configured with YAML and written in Go.**

Siafu simulates concurrent users against web pages and HTTP/HTTPS APIs, shows progress in a live TUI, and produces a report with thresholds and diagnosis. It includes a **closed mode** (VUs that wait for the response) and an optional **open-loop mode** (controlled arrival rate) for finding a backend's limit. No JavaScript, Node, browser, or external services required: one binary, one YAML file, and your target.

```bash
./bin/siafu run examples/demo.yaml
```

> **Responsible use notice:** only run Siafu against systems you own or have explicit permission to test. A load test can take a real service down.

> **Language note:** in v0.1 the CLI help, TUI, and report strings ship in Spanish. Commands, flags, YAML keys, and exit codes are language-neutral; this document is an English guide to the same behavior.

---

## Table of contents

- [What Siafu is](#what-siafu-is)
- [Why it exists](#why-it-exists)
- [Features](#features)
- [TUI preview](#tui-preview)
- [Requirements](#requirements)
- [Build from source](#build-from-source)
- [Install](#install)
- [Quick start](#quick-start)
- [CLI usage](#cli-usage)
- [Interactive menu](#interactive-menu)
- [TUI shortcuts](#tui-shortcuts)
- [Exit codes](#exit-codes)
- [YAML configuration](#yaml-configuration)
- [Templates and variables](#templates-and-variables)
- [Metrics and verdict](#metrics-and-verdict)
- [Diagnosis](#diagnosis)
- [CI mode](#ci-mode)
- [Results and local web](#results-and-local-web)
- [Demo server](#demo-server)
- [Architecture](#architecture)
- [Development and testing](#development-and-testing)
- [Known limitations](#known-limitations)
- [Troubleshooting](#troubleshooting)
- [Security](#security)
- [Roadmap](#roadmap)
- [Contributing](#contributing)
- [License](#license)

---

## What Siafu is

Siafu is a command-line tool for measuring how a website or API responds under concurrent load.

You describe the test in YAML:

- the target host;
- how many virtual users you want and how the load ramps up and down (closed mode), or what arrival rate you want (open-loop mode);
- which steps each user executes (login, query, detail, etc.);
- which responses count as successful;
- which latency, error rate, and throughput you accept.

Siafu runs the test, shows live metrics, and ends with a verdict:

- **PASSED** if every threshold is met;
- **FAILED** if any threshold is missed;
- **NO THRESHOLDS** if the YAML defines no acceptance criteria;
- **ABORTED** if you stop the run;
- **ERROR** if no requests completed.

It is meant to answer questions such as:

- How many requests per second can my API handle?
- Does p95 latency stay below 300 ms with 50 users?
- Do 5xx or 429 responses appear as load increases?
- Is the bottleneck in the server or in my load generator?
- Did the latest change degrade login or product listing?

## Why it exists

Tools such as k6 are very powerful, but modeling scenarios requires writing JavaScript. Siafu targets a narrower, common case:

- versionable, reviewable YAML configuration;
- strict validation before running;
- terminal-first execution with no infrastructure;
- explainable results with deterministic thresholds and diagnosis;
- one binary for Linux and macOS.

It does not aim to replace k6, JMeter, Gatling, or a distributed load-testing service. It aims to be the fastest way to run a smoke or capacity test against a web page or API without writing code.

## Features

- **Multi-step scenarios:** login → query → detail, with per-virtual-user cookies.
- **HTTP/3 over QUIC:** explicit `target.http_version: 3`, with TLS 1.3 and protocol metrics.
- **WebSocket:** per-VU connections with `connect`, `send`, `receive`, and `close`, message checks, and protocol metrics.
- **Virtual users with ramps:** ramp-up, steady, and ramp-down stages, plus think time.
- **Optional open-loop mode:** arrival rate in iterations/s, a pool bounded by `max_in_flight`, controlled drops, and per-stage failure reporting.
- **Memory guards:** preflight admission, analysis budget, RSS watchdog, and log/batch/connection limits.
- **Persistence and local web:** per-run JSON history and a mobile-first UI with SVG charts, no frontend dependencies.
- **Variables and secrets:** `${ENV:TOKEN}`, `${VU}`, `${ITER}`, and token extraction from JSON or headers.
- **Per-response checks:** HTTP status, body substring, and maximum duration.
- **Live metrics:** VUs, RPS, p50/p90/p95/p99, errors, checks, iterations, and bytes.
- **Generator metrics:** CPU, RAM, disk, and network of the machine running Siafu.
- **Thresholds:** p95, p99, error rate, and minimum RPS with a verdict and exit code.
- **Diagnosis:** deterministic rules for saturation, rate limiting, network/TLS errors, generator CPU, and more.
- **Four-color TUI:** live dashboard and scrollable final report.
- **Plain-text mode:** `--no-tui` for CI, cron, or non-interactive terminals.
- **No JavaScript:** no DOM rendering and no page assets are downloaded.
- **One binary:** no runtime, no Node, no cloud services.
- **Continuous tests:** unit, `httptest` integration, TUI without a PTY, demo-server E2E, and `-race`.

## TUI preview

Schematic dashboard while running (values are illustrative):

```text
┌ SIAFU · Demo local ───────────────────────────── http://127.0.0.1:8080 ┐
│ EJECUTANDO  00:12 / 00:12 · etapa 2/3 · VUs 20/20  ██████████            │
├─ GENERADOR ─────────────────────────┬─ CARGA ────────────────────────────┤
│ CPU    34%                          │ VUs        20/20     RPS   142.3   │
│ RAM    6.2 GB / 16.0 GB · 39%       │ p95        210 ms    p99   480 ms  │
│ DISCO  61% · 142 GB libres          │ Errores    0.8%      Checks 99.2%  │
│ RED    ↓ 12.4 MB/s · ↑ 3.1 MB/s     │ Peticiones 18 420    Iter. 3 905   │
├─ TENDENCIA ─────────────────────────┴───────────────────────────────────┤
│ RPS  ▁▂▃▄▅▆▇█▇▆▅▄▃▂ 142      p95  ▁▂▂▃▄▅▆▇▇▆▅▄▃ 210 ms                  │
├─ LOG ──────────────────────────────────────────────────────────────────┤
│ 10:42:01 INFO  etapa 2/3                                   │
│ 10:42:03 WARN  products/listado → 500 (status)              │
└ Ctrl+D detener   X logs   ? ayuda   Ctrl+C salir   ↑↓ desplazar ────────┘
```

The palette uses exactly four colors:

| Color | Hex | Use |
|---|---|---|
| `Ink` | `#0E1116` | Panel background |
| `Bone` | `#E6E1D6` | Primary text |
| `Ember` | `#FC4C02` | Accent, progress, failures |
| `Ash` | `#7A8394` | Secondary text and borders |

Designed for dark terminal themes with truecolor support; Lipgloss falls back to 256 colors when needed.

## Requirements

| Requirement | Detail |
|---|---|
| Go | **1.27 or newer** (pinned in `go.mod`) |
| System | Linux or macOS, amd64 or arm64 |
| Tooling | `git`; `make` is optional but recommended |
| Network | Access to the HTTP/HTTPS target you want to test |
| Terminal | Interactive TUI is optional; `--no-tui` does not need a TTY |

Siafu builds with `CGO_ENABLED=0`: the binary has no system library dependency.

> Windows is not supported in v0.1: host metrics and build targets are designed for Linux and macOS.

## Build from source

### 1. Clone the repository

```bash
git clone https://github.com/gac982/siafu.git
cd siafu
```

### 2. Check your Go version

```bash
go version
# go version go1.27.x ...
```

If your version is older than 1.27, upgrade Go before continuing.

### 3. Build with Make

```bash
make build
```

The binary is written to:

```text
bin/siafu
```

The build injects version, commit, and build date into the binary through `-ldflags`.

### 4. Verify the binary

```bash
./bin/siafu version
./bin/siafu --help
```

Expected output (values may vary):

```text
[SIAFU logo]
siafu v0.4.1 (e6e08f4) built 2026-11-12T18:55:50Z · linux/amd64
```

On a terminal, `version` prints the logo in color; when the output is not a TTY or `NO_COLOR` is set, it is rendered in plain text.

### 5. Build without Make (alternative)

If you do not have `make`:

```bash
mkdir -p bin
CGO_ENABLED=0 go build -trimpath -o bin/siafu ./cmd/siafu
./bin/siafu version
```

That binary works, but its version will be `dev`; use the Makefile to inject metadata, or pass it yourself:

```bash
VERSION=$(git describe --tags --always --dirty)
COMMIT=$(git rev-parse --short HEAD)
DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)

CGO_ENABLED=0 go build -trimpath \
  -ldflags "-s -w \
    -X github.com/gac982/siafu/internal/version.Version=$VERSION \
    -X github.com/gac982/siafu/internal/version.Commit=$COMMIT \
    -X github.com/gac982/siafu/internal/version.Date=$DATE" \
  -o bin/siafu ./cmd/siafu
```

### 6. Cross-compile

```bash
make build-all
```

Produces:

```text
bin/siafu-linux-amd64
bin/siafu-linux-arm64
bin/siafu-darwin-amd64
bin/siafu-darwin-arm64
```

### 7. Run the test suite

```bash
make check
```

This runs `gofmt`, `go vet`, and `go test -race ./...`. You can also run the full E2E suite:

```bash
make e2e
```

The E2E build compiles the binary, starts the demo server, runs a test that must pass, and runs another that must fail.

## Install

### Install into `GOPATH`/`GOBIN`

```bash
make install
```

or directly:

```bash
CGO_ENABLED=0 go install ./cmd/siafu
```

Make sure `$(go env GOPATH)/bin` is on your `PATH`.

### Install from a published repository

Once the repository is published and tagged:

```bash
go install github.com/gac982/siafu/cmd/siafu@latest
```

> If your fork uses a different module path, replace `github.com/gac982/siafu` with yours.

## Quick start

The fastest way to see Siafu working requires no external service: it ships with a local HTTP demo server.

### Terminal 1: demo server

```bash
make demo
```

The server listens on `http://127.0.0.1:8080` and exposes pages, products, token login, and slow/error endpoints.

### Terminal 2: load test

Validate the YAML first:

```bash
./bin/siafu validate examples/demo.yaml
```

Run the test with the TUI:

```bash
./bin/siafu run examples/demo.yaml
```

When it finishes you get a report with metrics, thresholds, and diagnosis. The demo test is designed to pass (`PASSED`).

Run without the TUI:

```bash
./bin/siafu run --no-tui examples/demo.yaml
```

To see a controlled failure:

```bash
./bin/siafu run --no-tui examples/failing.yaml
```

`examples/failing.yaml` sets an impossible `min_rps`; the output should be `FAILED` and the exit code `1`.

## CLI usage

```text
siafu <command> [options]

Commands:
  run <file.yaml>       run a load test
  validate <file.yaml>  validate the configuration without running
  runs <subcommand>     run history (list, show, compare, prune, reindex)
  serve                 serve the local results web UI
  menu                  open the interactive menu
  version               print version and platform

Options:
  -h, --help            show help
```

### Interactive menu

For people who would rather not memorize commands and flags:

```bash
siafu          # opens the menu when a terminal is available
siafu menu     # explicit form
```

It is a full-screen TUI using Siafu's palette: navigate with `↑/↓` or `j/k`, choose with `Enter`, go back with `Esc`/`q`, and quit with `Ctrl+C`. The menu lets you:

1. **Run a test:** pick a YAML file, review its summary, adjust seed, tag, directory, no-save, no-TUI, or debug, and run it.
2. **Browse previous runs:** paginated list, filters by mode/verdict/text, detail and comparison with cursor selection.
3. **Open the results web UI:** configurable port, automatic browser opening, and `s`/`Esc` to stop the server and return.
4. **Validate a YAML** without generating load.
5. **Maintenance:** keep only N runs (with preview and `SI` confirmation) and reindex.
6. **Settings and version:** version and the session's results directory.

The menu sanitizes terminal control sequences in displayed text and caps the length of its inputs. The YAML is still authored outside; the menu neither generates nor edits it. The flag-based CLI (`run`, `validate`, `runs`, `serve`) is unchanged. Without a TTY, `siafu` with no arguments keeps the help and exit code 2; `siafu menu` explains that it needs a terminal.

### `siafu run`

```bash
siafu run <file.yaml> [--no-tui] [--seed N] [--debug] [--results-dir DIR] [--no-save] [--tag TAG]
```

| Flag | Description |
|---|---|
| `--no-tui` | Force plain-text output; useful in CI. |
| `--seed N` | Fix the think-time and reproducibility seed. Overrides YAML. |
| `--debug` | Enable orchestrator debug logs. |
| `--results-dir DIR` | Results history directory; defaults to `.siafu/runs`. |
| `--tag TAG` | Free-form tag for filtering the run in the history. |
| `--no-save` | Do not persist the run. |

Flags may appear before or after the file:

```bash
siafu run --no-tui examples/demo.yaml
siafu run examples/demo.yaml --no-tui
```

If stdout is not a TTY, Siafu automatically uses plain-text mode.

### `siafu validate`

Validates syntax, unknown fields, semantic rules, templates, and variable references without generating load:

```bash
siafu validate examples/api.yaml
```

Example output:

```text
OK  API de ejemplo
  1 escenario(s), 2 paso(s)
  duración 1m0s, pico 20 VUs
  umbrales: p95≤400ms · p99≤1.2s · errores≤1.00% · rps≥10.0
```

In open-loop mode the summary switches to rate and pool:

```text
OK  Estrés API open-loop
  1 escenario(s), 1 paso(s)
  modo open · duración 30s · in-flight 8000 · pico 20000 iteraciones/s
  umbrales: p95≤2s · errores≤0.00%
```

Errors are accumulated with the field path:

```text
configuración inválida:
- scenarios[0].steps[1].request.url: referencia desconocida "${token}"
- thresholds.latency_p99: debe ser mayor o igual que latency_p95
```

### `siafu version`

```bash
siafu version
```

Prints version, commit, build date, operating system, and architecture.

### Signals

- In the TUI, `Ctrl+D` stops the test gracefully and shows the partial report.
- In the TUI, `Ctrl+C` exits immediately with code 130.
- In plain mode, `SIGINT`/`SIGTERM` abort and emit a partial `ABORTED` report with code 130.

## TUI shortcuts

| Key | Action |
|---|---|
| `Ctrl+D` | Stop the test and show the report (`ABORTED`). |
| `Ctrl+C` | Exit immediately (130 while the test is running). |
| `X` | Toggle full-screen logs. |
| `?` | Show or hide help. |
| `↑` / `↓` | Scroll logs or report. |
| `PgUp` / `PgDn` | Fast scroll. |
| `Q` / `Esc` | Exit from the report screen. |

The TUI adapts to terminal width:

- two columns on wide terminals;
- two compact columns on medium widths;
- one column below 56 columns;
- hides the trend chart when there is not enough height;
- hides the log panel when fewer than three lines remain and lets `X` open it full-screen.

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Test passed or no thresholds defined. |
| `1` | At least one threshold failed. |
| `2` | Configuration, validation, usage, or safety-limit rejection. |
| `3` | Fatal runtime error, zero completed requests, or memory limit reached. |
| `130` | Test aborted by the user or a signal. |

These codes let CI distinguish a performance failure (`1`) from an invalid YAML (`2`) or a runtime problem (`3`).

## YAML configuration

A minimal file:

```yaml
version: 1
name: "My first test"

target:
  base_url: "https://api.example.com"
  timeout: 10s

load:
  think_time: 300ms
  stages:
    - duration: 30s
      target_vus: 10
    - duration: 1m
      target_vus: 50
    - duration: 30s
      target_vus: 0

scenarios:
  - name: query
    steps:
      - name: list
        request:
          method: GET
          url: /items
        expect:
          status: 200
          body_contains: "items"

thresholds:
  latency_p95: 400ms
  error_rate: 0.01
```

### Root

| Field | Type | Required | Description |
|---|---|---|---|
| `version` | int | yes | Must be `1`. |
| `name` | string | yes | Test name (1–80 characters). |
| `target` | object | yes | HTTP target and client options. |
| `load` | object | yes | Stages, think time, and limits. |
| `vars` | map | no | Immutable global variables. |
| `scenarios` | list | yes | 1–20 scenarios, each with steps. |
| `thresholds` | object | no | Acceptance criteria. |
| `limits` | object | no | Memory, analysis, log, and connection budgets. |

### `target`

| Field | Default | Rules |
|---|---|---|
| `base_url` | — | Required. `http` or `https`, with host, no query or fragment. Does not allow templates. |
| `timeout` | `30s` | Between `1s` and `300s`; applies to the whole request. |
| `http_version` | `auto` | `auto`, `1.1`, `2`, or `3`. Applies to HTTPS; `2` requires HTTP/2 and `3` requires HTTP/3. |
| `headers` | `{}` | Shared headers. Templates allowed; step headers take precedence. |
| `tls.insecure_skip_verify` | `false` | Skip certificate verification. Controlled tests only. |
| `tls.ca_file` | `""` | Path to a CA PEM for the target. Resolved relative to the YAML file. |

Siafu negotiates **HTTP/2 over HTTPS** when `http_version` is `auto` or `2`; if the server does not offer it, `auto` falls back to HTTP/1.1 and `2` fails with a protocol error. `1.1` forces the previous behavior. Plain `http://` uses HTTP/1.1 (h2c is not supported). It keeps keep-alive, environment proxy (`HTTP_PROXY`/`HTTPS_PROXY`), and up to 10 redirects.

With `http_version: 3`, Siafu uses **HTTP/3 over QUIC** (UDP and TLS 1.3) and fails when the server does not negotiate HTTP/3. `3` is not available over `http://` (h3c is not supported), `auto` does not discover HTTP/3 through Alt-Svc yet, and requests bypass `HTTP_PROXY`/`HTTPS_PROXY` because current HTTP proxies do not carry QUIC. WebSocket handshakes still use HTTP/1.1 even when the target uses `3`.

### `load`

| Field | Default | Rules |
|---|---|---|
| `stages` | — | 1–50 stages. Total duration cannot exceed 6 hours. |
| `stages[].duration` | — | Greater than 0. |
| `stages[].target_vus` | — | 0–`max_vus`; at least one stage must be above 0. |
| `max_vus` | `2000` | Between 1 and 20,000. |
| `think_time` | `0` | Scalar (`500ms`) or range `{min,max}`. |
| `seed` | random | 64-bit integer to reproduce think time. |
| `mode` | `closed` | `closed` (VUs) or `open` (arrival rate). |
| `open_loop.stages` | — | Required in `open`: 1–50 `{duration, target_rate}` stages. |
| `open_loop.max_in_flight` | — | Required in `open`: 1–20,000. |
| `open_loop.on_overload` | `drop` | Only `drop`: no queue; drop when saturated. |
| `open_loop.on_analysis_exhausted` | `skip` | `skip` or `fail`. |

The ramp is linear: each stage interpolates from the previous target (0 at the start) to its `target_vus`. A final stage with `target_vus: 0` ramps load down before finishing.

**Scenario distribution:** each VU picks a scenario in round-robin order (`id % number_of_scenarios`). There are no weights in v0.1; each scenario receives roughly the same number of VUs.

### Open-loop mode

`load.mode: open` changes the model: instead of VUs waiting for a response before the next iteration, you define an **arrival rate** in iterations per second and a bounded worker pool. Closed mode remains the default.

```yaml
load:
  mode: open
  open_loop:
    stages:
      - {duration: 1m, target_rate: 1000}
      - {duration: 2m, target_rate: 5000}
    max_in_flight: 8000
    on_overload: drop
    on_analysis_exhausted: skip
```

- `target_rate` is measured in **iterations/s**. With a single step it equals RPS; with multiple steps the actual RPS is higher.
- Each worker keeps its own session and cookie jar. In v1 the session belongs to the worker, not to a unique user across iterations.
- When no worker is free, the iteration is **dropped** (`dropped_iterations`) and reported. There is no unbounded queue.
- `max_in_flight` bounds workers and in-flight iterations. The peak never exceeds it.
- `think_time` is unused in this mode; the rate controls pacing.
- The report shows target rate, average achieved rate, average rate on the target stage, peak in-flight, drops, and the first failure point per stage.
- Backend errors do not abort the test; they are reported separately from generator drops.
- `on_analysis_exhausted`: `skip` omits `body_contains`/`extract` when the analysis budget is full; `fail` marks the step with `analysis_budget`.
- Memory: 32 MiB + 55 KiB per in-flight. With 450 MiB the admitted maximum is 7,782; with 600 MiB, 10,575. Measured: 8,000 in-flight ≈ 467 MiB. Use `memory_budget_mb: 600` for 8,000.

### `limits`

Process safety budgets. Admission runs **before** any connection or VU is created: if `load.max_vus` exceeds the budget capacity, Siafu rejects the YAML with code `2` and executes no request.

| Field | Default | Rules |
|---|---|---|
| `memory_budget_mb` | `450` | Between 1 and 1024. Process RSS budget. |
| `warn_at_percent` | `80` | Between 50 and 95. Percentage of the budget that emits a warning. |
| `analysis_budget_mb` | `64` | Between 1 and 128. Maximum bytes retained for body analysis. |
| `max_log_lines` | `1000` | Between 100 and 10,000. TUI log ring size. |
| `max_batch_samples` | `64` | Between 1 and 512. Samples per batch before sending them to metrics. |
| `max_idle_conns` | `1024` | Between 1 and 4096. Idle keep-alive connections per host. |

VU capacity is estimated with a 32 MiB process reserve and 50 KiB per VU. With `memory_budget_mb: 450` the safe maximum is **8,560 VUs**; with `1024` it is **19,849 VUs**. To run 15,000 VUs use `memory_budget_mb: 1024`, because ramp transients can exceed 800 MiB. The absolute cap is **1024 MiB** and **20,000 VUs**.

Measured on the reference machine: 10,000 VUs use ~290 MiB at steady state, and 15,000 VUs reach **699–967 MiB** peak (HWM) during the ramp. The watchdog samples current RSS, not HWM, and tolerates brief peaks; the budget is not a hard HWM ceiling. Use progressive ramps and do not set the budget to the bare minimum.

If RSS exceeds `memory_budget_mb` for two consecutive samples (≈2 s), Siafu stops the test to avoid an OOM and exits with code `3`.

### Capacity and system limits

Measured against the local demo (small responses, HTTP/1.1, 16 vCPU) with `examples/stress-15000.yaml`:

| VUs | Peak RSS | Established connections | Average RPS | Errors |
|---:|---:|---:|---:|---:|
| 2,000 | 108 MiB | 1,024 | 13,589 | 0 |
| 5,000 | 283 MiB | 4,355 | 33,129 | 0 |
| 10,000 | 550 MiB | 9,040 | 56,572 | 0 |
| 15,000 | 699–967 MiB | 9,971 | 55,623 | 0 |

The ramp matters: transient heap peaks during an aggressive ramp can exceed steady-state RSS. Use progressive ramps and a budget with headroom. For 15,000 VUs the profile declares `memory_budget_mb: 1024`. Connections are counted on the client side; think time and the idle pool reuse connections, so the established peak can be lower than the VU count.

**Ports:** the default ephemeral range (`32768-60999`) provides 28,232 ports per local IP and destination. 15,000 connections fit with margin; 50,000 do not fit on a single IP and require HTTP/2 or distributed execution.

**System limits:** run `bash scripts/preflight.sh` (or `make preflight`) before a large test. It checks:

- `ulimit -n` ≥ 32,768;
- `net.ipv4.ip_local_port_range` with ≥ 20,000 ports;
- `net.core.somaxconn` ≥ 4,096;
- `net.ipv4.tcp_max_syn_backlog` ≥ 2,048;
- `net.netfilter.nf_conntrack_max` ≥ 262,144 when a firewall/NAT is present.

Run the full harness with `make stress` (closed) or `make stress-open` (open-loop); it builds, checks preflight, starts the demo, and measures RSS/connections/drops. For a baseline: `VUS=10000 make stress`.

Large bodies are outside this target: 15,000 VUs with `body_contains` or `extract` over 1 MiB responses multiply memory by the analysis budget. Use small responses or reduce concurrency.

### `scenarios[].steps[]`

| Field | Default | Rules |
|---|---|---|
| `name` | — | Required, unique within the scenario. |
| `request.method` | `GET` | `GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `HEAD`, `OPTIONS`. |
| `request.url` | — | Required. Absolute or relative to `base_url`. |
| `request.headers` | `{}` | Step headers; templates allowed. |
| `request.body` | `""` | Request body; templates allowed. |
| `expect` | 2xx/3xx | Response checks. |
| `extract` | `{}` | Variables to extract from the response. |
| `websocket` | — | WebSocket action; replaces `request`. See below. |

### `expect`

| Field | Default | Description |
|---|---|---|
| `status` | 200–399 | Integer or list of integers (100–599). |
| `body_contains` | `""` | Substring to search in the first 1 MiB of the body. |
| `max_duration` | `0` (off) | Maximum acceptable duration for that step. |

All configured checks are always evaluated. A failed check marks the request as failed, but the scenario continues with the next step. Extraction only runs when every check passed.

### `extract`

```yaml
extract:
  token: "json:data.token"
  product_id: "json:items.0.id"
  session: "header:X-Session-Id"
```

- `json:<path>` navigates objects and arrays with dots (`items.0.id`).
- `header:<Name>` reads a response header.
- Extracted variables are stored as text.
- They are cleared at the start of each iteration: every loop must extract what it needs again.
- If extraction fails, the current iteration is aborted.
- An extracted key cannot collide with a global variable.

### WebSocket

A `websocket` step replaces `request` and runs one action on a named connection owned by each VU:

```yaml
- name: connect
  websocket: {action: connect, url: /ws, connection: market}
- name: send
  websocket: {action: send, connection: market, text: '{"type":"subscribe"}'}
- name: receive
  websocket: {action: receive, connection: market, expect: {message_contains: "subscribed"}}
- name: close
  websocket: {action: close, connection: market}
```

| Field | Default | Rules |
|---|---|---|
| `action` | — | `connect`, `send`, `receive`, or `close`. |
| `connection` | `default` | Per-VU name; `[A-Za-z0-9_-]{1,32}`. |
| `url` | — | Connect only; relative to `base_url` or `ws://`/`wss://`. |
| `headers` | `{}` | Connect only; templates allowed. |
| `subprotocols` | `[]` | Connect only. |
| `text` | `""` | Send only; exactly one of `text`/`binary_base64`. |
| `binary_base64` | `""` | Send only; binary message. |
| `timeout` | `target.timeout` | Handshake or message deadline; 1s–300s. |
| `expect.status` | `101` | Connect only. |
| `expect.message_contains` | `""` | Receive only; text. |
| `expect.max_duration` | `0` (off) | Connect, send, or receive. |
| `close_code` | `1000` | Close only; 1000–4999. |

- The handshake uses **HTTP/1.1**, even when `target.http_version` is `auto`, `2`, or `3`.
- `wss://` uses `target.tls` (`ca_file` or `insecure_skip_verify`).
- Connections close at the end of every iteration, with or without an error.
- `message_contains` only applies to text messages; binary send is supported, but binary checks are future work.
- `extract` is not supported in WebSocket steps v1.

### `vars` and templates

```yaml
vars:
  user_email: "load+${ENV:RUN_ID}@example.com"
  user_agent: "siafu/${ENV:VERSION}"
```

| Syntax | Meaning |
|---|---|
| `${name}` | Global or per-VU extracted variable. |
| `${ENV:NAME}` | Environment variable, resolved at runtime. |
| `${ENV:NAME:-value}` | Environment variable with a default. |
| `${VU}` | Virtual user number (1-based). |
| `${ITER}` | VU iteration number (1-based). |
| `$${` | Escape: emits a literal `${`. |

Global variables are immutable and evaluated on every expansion. They cannot reference other global variables; they may use `ENV`, `VU`, and `ITER`.

### `thresholds`

| Field | Example | Rule |
|---|---|---|
| `latency_p95` | `400ms` | Observed p95 ≤ value. |
| `latency_p99` | `1200ms` | Observed p99 ≤ value. |
| `error_rate` | `0.01` | Failed request fraction ≤ value (0–1). |
| `min_rps` | `20` | Average RPS ≥ value. |

If you define no thresholds, the verdict is `NO THRESHOLDS` and the exit code is `0`.

### Authenticated API example

```yaml
version: 1
name: "Authenticated API"

target:
  base_url: "https://api.example.com"
  timeout: 10s
  headers:
    Accept: "application/json"

load:
  think_time:
    min: 200ms
    max: 600ms
  stages:
    - duration: 15s
      target_vus: 5
    - duration: 30s
      target_vus: 20
    - duration: 15s
      target_vus: 0

vars:
  user_email: "load+${ENV:RUN_ID}@example.com"

scenarios:
  - name: login_and_profile
    steps:
      - name: login
        request:
          method: POST
          url: /login
          headers:
            Content-Type: application/json
          body: |
            {"email": "${user_email}", "password": "${ENV:LOAD_PASSWORD}"}
        expect:
          status: 200
        extract:
          token: "json:token"

      - name: profile
        request:
          method: GET
          url: /me
          headers:
            Authorization: "Bearer ${token}"
        expect:
          status: 200
          body_contains: "email"

thresholds:
  latency_p95: 400ms
  latency_p99: 1200ms
  error_rate: 0.01
  min_rps: 10
```

Run it with:

```bash
export RUN_ID="$(date +%s)"
export LOAD_PASSWORD="your-test-password"
./bin/siafu validate examples/api.yaml
./bin/siafu run examples/api.yaml
```

### Web page example

```yaml
version: 1
name: "Public site"

target:
  base_url: "https://example.com"
  timeout: 15s

load:
  think_time: 800ms
  stages:
    - duration: 20s
      target_vus: 10
    - duration: 40s
      target_vus: 30
    - duration: 10s
      target_vus: 0

scenarios:
  - name: home
    steps:
      - name: landing
        request:
          method: GET
          url: /
        expect:
          status: 200
          body_contains: "<html"

  - name: about
    steps:
      - name: about_page
        request:
          method: GET
          url: /about
        expect:
          status: [200, 304]

thresholds:
  latency_p95: 800ms
  error_rate: 0.02
```

Siafu does not download CSS, images, or execute JavaScript: a “page” is the HTTP request you declare.

## Templates and variables

### Lifecycle

- Each VU has its own `http.Client`, cookie jar, and extracted-variable map.
- Cookies persist for the whole test.
- Extracted variables are cleared at the start of each iteration.
- Global `vars` are immutable and evaluated on every expansion.
- `ITER` starts at 1 and increments after each attempt, even if the iteration aborts.
- A global variable cannot reference another global variable.

### Errors that abort an iteration

- Transport error (DNS, dial, TLS, timeout, read).
- Unresolved template (for example, missing `${ENV:TOKEN}`).
- Failed extraction.

A failed check (status, body, or duration) does **not** abort the iteration; it continues with the next step.

### Secrets

- `${ENV:...}` values are resolved at runtime, not during validation.
- Siafu never writes request or response bodies to logs.
- Sensitive header values (`Authorization`, `Cookie`, `Set-Cookie`, `X-Api-Key`, `X-Auth-Token`, etc.) are never logged.
- Query parameters with sensitive names (`token`, `key`, `secret`, `password`, `signature`, `session`, etc.) are masked.
- Values of referenced environment variables are replaced with `***` in log messages.

## Metrics and verdict

### Requests

- **Completed requests:** requests that finished reading/discarding the response. Requests canceled by a stop are excluded from totals and latency.
- **Failed:** requests with at least one failed check or a transport/template/extraction error.
- **Error rate:** failed / completed.
- **Checks:** passed checks / evaluated checks.
- **Iterations:** complete passes through a scenario; an iteration is failed if any request failed.

### Latency

Siafu measures from before sending the request until the body is fully read/discarded. It includes connection setup, TLS, header wait, and transfer.

Percentiles are calculated with HDR Histogram: p50, p90, p95, p99, min, max, and mean.

### RPS

- **Average:** completed requests / planned duration.
- **Recent:** requests in the last 5 seconds / 5.
- **Peak:** best 5-second window and the time it occurred.

### Open-loop metrics

- **Target rate:** iterations/s requested by the scheduler at the current instant (or the stage peak).
- **Achieved rate:** average iterations/s over the run.
- **Target-stage average:** average of the stage with the highest `target_rate`; this is the figure comparable to the target on a plateau.
- **Peak in-flight:** maximum simultaneous iterations; never exceeds `max_in_flight`.
- **Drops:** iterations the generator could not dispatch because no worker was free. These are not backend errors; diagnosis reports them as generator saturation.
- **Failure point:** first stage whose error rate exceeds 1%; drops are shown separately.
- **Skipped analysis:** steps where `body_contains`/`extract` were not evaluated because the analysis budget was exhausted with `on_analysis_exhausted: skip`.

### Response body

- With `body_contains` or `extract`, up to **1 MiB** is analyzed.
- Without body checks or extraction, up to **10 MiB** is discarded.
- Exceeding the limit marks `body_too_large` and closes the connection.
- Bodies are read decompressed when the server uses gzip (Go behavior).

### Generator metrics

CPU, RAM, disk, and network cards describe the **machine running Siafu**, not the target. The report states this explicitly. They help detect whether the test machine ran out of CPU before the server did.

### Verdict

| State | Condition | Exit |
|---|---|---|
| `PASSED` | Thresholds exist and all pass. | 0 |
| `FAILED` | At least one threshold fails. | 1 |
| `NO THRESHOLDS` | No thresholds and requests completed. | 0 |
| `ABORTED` | The user stopped the test. | 130 |
| `ERROR` | Zero completed requests or fatal error. | 3 |

### Example report

```text
SIAFU · Demo local
Destino: http://127.0.0.1:8080
Inicio: 2026-11-07 13:05:01
Duración: planificada 12.00s / real 12.04s
Modo: cerrado · pico 20 VUs · Semilla: 405
Veredicto: APROBADA

RESUMEN GLOBAL
  Peticiones:      18420
  Fallidas:        148 (0.80%)
  Iteraciones:     6140 (fallidas 148)
  RPS:             promedio 1535.0 · pico 1890.2
  Latencia:        p50 12ms · p90 35ms · p95 48ms · p99 90ms
  Checks:          18420/18420 (100.00%)
  Canceladas:      0

POR ESCENARIO
  home
    peticiones 6000 · fallidas 0 (0.00%) · iteraciones 6000
  products
    peticiones 6000 · fallidas 0 (0.00%) · iteraciones 3000

CÓDIGOS DE ESTADO
  200: 18420

UMBRALES
  ✓ latencia p95: 48ms (esperado ≤ 500ms)
  ✓ tasa de error: 0.80% (esperado ≤ 1.00%)

DIAGNÓSTICO
  · Todos los umbrales se cumplen.

GENERADOR
  CPU:  promedio 34.2% · máximo 51.0%
  RAM:  máximo 22.1%
  RED:  pico ↓ 12.4 MB/s · ↑ 3.1 MB/s
  (métricas del equipo que ejecutó siafu, no del destino)
```

In open-loop mode the report adds rate and drop metrics:

```text
MODO OPEN LOOP
  Tasa objetivo:   20000.0 iteraciones/s (pico)
  Tasa alcanzada:  19999.1 iteraciones/s (media) · 19999.1 peticiones/s
  Tasa media objetivo: 19998.8 iteraciones/s (objetivo 20000.0/s)
  In-flight pico:  221
  Descartes:       0
  Punto de fallo:  sin fallo registrado
  ETAPAS
    etapa 1 · objetivo 1000.0/s · alcanzado 999.9/s · peticiones 5000 · fallidas 0 · descartes 0
    etapa 2 · objetivo 5000.0/s · alcanzado 4999.8/s · peticiones 25000 · fallidas 0 · descartes 0
```

## Diagnosis

At the end, Siafu applies deterministic rules and explains likely causes. No opaque AI or arbitrary scores.

| Signal | Interpretation |
|---|---|
| Zero requests | Wrong URL, connectivity, TLS, or configuration. |
| Template errors | Missing `${ENV:...}` or an unresolved reference. |
| DNS/dial/TLS/timeout | Network, certificate, firewall, or timeout issue. |
| Dominant 429 | The target is rate limiting. |
| Dominant 5xx | Possible server saturation or server bug. |
| Dominant 4xx | Review `expect.status`, request data, or authentication. |
| High p95 + falling RPS | Target saturation. |
| High p95 without errors | Slow network or slow processing. |
| `min_rps` not reached | The test did not generate enough load; in open-loop check `target_rate` and `max_in_flight`. |
| Open-loop drops | The generator could not sustain the rate; reduce `target_rate` or raise `max_in_flight`. |
| Skipped analysis | The analysis budget was exhausted; `body_contains`/`extract` were not evaluated on those steps. |
| High generator CPU | The bottleneck may be the test machine. |
| `body_too_large` | Responses exceed the analysis limits. |
| Memory limit reached | RSS exceeded `limits.memory_budget_mb`; the test stopped to avoid an OOM. |

## CI mode

`--no-tui` or non-interactive stdout uses plain text:

```bash
siafu run --no-tui loadtest.yaml
```

- Periodic status goes to **stdout**.
- Logs go to **stderr**.
- The final report goes to **stdout**.
- The exit code can fail the pipeline.

GitHub Actions example:

```yaml
name: load-test

on:
  workflow_dispatch:

jobs:
  load:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: "1.27.x"

      - name: Build
        run: |
          go build -o siafu ./cmd/siafu

      - name: Run load test
        env:
          RUN_ID: ${{ github.run_id }}
          LOAD_PASSWORD: ${{ secrets.LOAD_PASSWORD }}
        run: |
          ./siafu validate examples/api.yaml
          ./siafu run --no-tui examples/api.yaml
```

The same pattern works in GitLab CI, CircleCI, Jenkins, or cron: run the binary with `--no-tui` and honor the exit code.

## Results and local web

Each run is stored by default in `.siafu/runs/` (ignored by Git):

```text
.siafu/runs/
  index.jsonl
  20261109-093354-demo-a1b2/
    run.json        # versioned summary (schema_version 1)
    series.jsonl    # one point per second: rps, p95/p99, errors, in-flight, target/achieved, dropped, cpu/mem
    summary.txt     # plain-text report
```

`run` flags:

- `--results-dir DIR` (or `SIAFU_RESULTS_DIR`) changes the directory.
- `--tag TAG` tags the run.
- `--no-save` does not persist (useful in CI or quick tests).

History:

```bash
siafu runs list
siafu runs list --json --limit 10 --mode open --verdict PASS
siafu runs show <id>
siafu runs compare <a> <b>
siafu runs prune --keep 50 --dry-run
siafu runs reindex
```

Local web UI:

```bash
siafu serve --addr 127.0.0.1:8080 --results-dir .siafu/runs
```

`--web-dir DIR` serves assets from disk during development. Open `http://127.0.0.1:8080`. It includes a run list with filters and search, a detail view with RPS/p95/p99/errors/host/open-loop, and a two-run comparison with deltas and overlaid charts. Navigation uses links (`#/runs/<id>`, `#/compare?a=...&b=...`).

Writes are atomic with best-effort `fsync` of files and directories; this is not a transactional database. `run` may create `.siafu/runs`; read-only commands (`runs list`, `serve`) do not. The API and assets accept `HEAD`.

The UI is dependency-free HTML/CSS/JS, mobile first, with custom SVG charts and no external requests. The server listens on localhost by default and is read-only. JSON is the source of truth; **v1 does not use SQLite**. Headers, bodies, sensitive query values, and secrets are not stored; the target is redacted and the YAML is identified by name and hash. Aborted-run thresholds render as “No evaluados”; the list has a **Refrescar** button; SVGs include `<desc>` and reference lines when the YAML defines p95/error/RPS (p99 is a future improvement).

## Demo server

`examples/demo` is a small, deterministic HTTP target for examples, development, and E2E. It has no dependencies.

```bash
make demo
# or
make demo-build
./bin/siafu-demo -addr 127.0.0.1:8080
```

| Method | Path | Response |
|---|---|---|
| `GET` | `/` | HTML containing `<html`. |
| `GET` | `/api/products` | JSON list with `products` and `items`. |
| `GET` | `/api/products/{id}` | Product 1–3. |
| `POST` | `/api/login` | JSON `{"email":...}` → `{"token":"token-N"}`. |
| `GET` | `/api/me` | Requires `Authorization: Bearer token-N`. |
| `GET` | `/api/slow?ms=200` | Sleeps up to 5 s. |
| `GET` | `/api/error` | Controlled `500`. |

The demo server is not production-ready: it keeps tokens in memory and has no real authentication.

## Architecture

```text
cmd/siafu                  main binary
internal/cli               subcommands, flags, signals, exit codes
internal/config            YAML schema, defaults, validation
internal/admission         preflight rejection outside the safety budget
internal/vars              ${...} compilation/rendering
internal/redact            secret masking
internal/httpx             HTTP/1.1/HTTP/2/HTTP/3, WebSocket client, checks, extraction
internal/metrics           samples, HDR, RPS, snapshots
internal/memguard          RSS watchdog and memory-limit shutdown
internal/results           schema, atomic store, and run index
internal/web               local server, JSON API, and embedded UI
internal/sysmetrics        generator CPU/RAM/disk/network
internal/engine            VUs, stage scheduler, shutdown
internal/report            thresholds, verdict, diagnosis, text
internal/tui               Bubble Tea dashboard and report
internal/logbuf            redacted, rate-limited log ring
internal/orchestrator      wires engine + metrics + TUI/plain mode
examples/demo              example target server
scripts/e2e.sh             end-to-end test
```

Simplified flow:

```text
YAML → validation → compiled plan → concurrent VUs → sample batches
     → aggregator → snapshots → TUI / plain mode → report + verdict
```

Design decisions:

- One goroutine per VU, with its own cookie jar and variables.
- A shared transport for keep-alive: `http.Transport` for HTTP/1.1/HTTP/2 and `http3.Transport` for HTTP/3, plus a separate HTTP/1.1 transport for WebSocket.
- A single aggregator owns metric state (no per-request locks).
- The UI consumes immutable snapshots; it does not know the engine.
- The report is serialized to versioned JSON and persisted per run.
- Diagnosis uses explicit, testable rules.

## Development and testing

### Commands

| Command | What it does |
|---|---|
| `make build` | Build `bin/siafu`. |
| `make build-all` | Build Linux/macOS × amd64/arm64. |
| `make install` | Install with `go install`. |
| `make test` | `go test ./...`. |
| `make race` | `go test -race ./...`. |
| `make lint` | `go vet` + `gofmt` check. |
| `make check` | lint + race; the quality gate. |
| `make vuln` | `govulncheck` over source and binary. |
| `make coverage` | Coverage and total. |
| `make demo` | Build and run the demo server. |
| `make demo-build` | Build only `bin/siafu-demo`. |
| `make e2e` | Demo + binary + end-to-end PASS/FAIL (closed and open-loop). |
| `make tui-smoke` | TUI PTY test: early keys, `Ctrl+D` and `Q`. |
| `make web-smoke` | Save a run, start `serve`, and validate the API/assets. |
| `make preflight` | Check `ulimit`, ephemeral ports, backlog, and conntrack. |
| `make stress` | Closed 15,000 VU stress against the demo. |
| `make stress-open` | Open-loop stress from 1k to 20k iterations/s against the demo. |
| `make clean` | Remove `bin/` and `coverage.out`. |

**Dev-only** browser check (not part of `make check`):

```bash
npm install playwright
npx playwright install firefox
make build
node scripts/web-browser-check.js
```

### Test strategy

- **Unit:** configuration, templates, checks, extraction, redaction, histograms, thresholds, and diagnosis.
- **HTTP integration:** `httptest` with latency, 429/500, timeouts, TLS, redirects, truncated bodies, and cookies.
- **Engine:** ramps, graceful stop, abort, 500 VUs, and no deadlocks.
- **Open-loop:** rate interpolation, ±5% accuracy on a plateau, in-flight cap, drops, bounded stop, and analysis budget.
- **Opt-in stress:** 15,000 closed VUs, 1M open-loop requests, and 8,000 in-flight (`SIAFU_STRESS=1`).
- **Persistence:** JSON schema, atomic save, index, queries, and path traversal.
- **Web:** `httptest` API, assets without external URLs, and a `serve` smoke test.
- **TUI:** `Update`/`View` without a PTY, at 120×40, 80×24, and 60×20.
- **E2E:** real binary against the demo server, with passing and failing configurations.

### Adding code

- Keep `internal/` boundaries and dependency direction.
- Add a failing test before implementing.
- Run `make check` before committing.
- Use SPDX headers in new files.
- Commits follow Conventional Commits (`feat:`, `fix:`, `docs:`, `test:`, `chore:`).

## Known limitations

- **HTTP/1.1, HTTP/2, and HTTP/3 over HTTPS, plus WebSocket over HTTP/1.1.** No h2c, gRPC, or WebSocket over extended HTTP/2 yet.
- **No browser.** JavaScript is not executed and DOM assets are not downloaded.
- **Local web, no authentication.** `siafu serve` listens on loopback and is read-only; it is not meant for public exposure.
- **No SQLite in v1.** History uses `index.jsonl`; SQLite is a future improvement.
- **No distributed load.** All VUs originate from the local machine.
- **Session per worker in open-loop.** In v1 each worker keeps its own session; there is no unique-user session store across iterations.
- **Open-loop failure point uses error rate.** Per-stage p95 is not broken down; the report's p95 is global.
- **`on_analysis_exhausted: skip`** can omit `body_contains`/`extract`; the report shows skipped steps and a warning.
- **No automatic retries.** A network failure counts as a failure; it is not retried.
- **No scenario weights.** Distribution is round-robin.
- **No mTLS.** Only a custom CA and `insecure_skip_verify`.
- **Windows is not supported** in v0.1.
- **Body limits:** 1 MiB for analysis, 10 MiB for discard.
- **Load limits:** 2,000 VUs by default, 20,000 maximum, 6 hours; open-loop 8,000 in-flight in the profile, 20,000 maximum, and a 1024 MiB budget.
- **`ca_file`** defines the target trust pool; in v0.1 it is not merged with the system pool.
- Host metrics render as `0` when a metric is unavailable; they never abort the test.

## Troubleshooting

### The test exits with code 130 and `ABORTED`

You stopped the test with `Ctrl+D` or a signal, or the context was canceled. This is expected for an incomplete run.

### Code 3 and zero requests

Siafu could not complete any request. Check:

- `base_url` is correct and reachable;
- DNS, firewall, and proxy;
- the server accepts the protocol configured in `target.http_version`;
- `timeout` is not too low;
- stderr logs.

### Code 2 and rejected configuration

The admission message shows the configured value, the allowed maximum, the excess, and the corrective action. Adjust `load.max_vus` or raise `limits.memory_budget_mb`; no request was executed.

### Code 3 due to the memory limit

Siafu stopped the test because RSS exceeded `limits.memory_budget_mb` for two consecutive samples. Reduce `load.max_vus`, lower `limits.analysis_budget_mb`, or raise `limits.memory_budget_mb` (maximum 1024). For 15,000 VUs use `memory_budget_mb: 1024`.

### TLS error with a self-signed certificate

For a controlled environment:

```yaml
target:
  tls:
    insecure_skip_verify: true
```

Or provide the CA:

```yaml
target:
  tls:
    ca_file: "./certificates/ca.pem"
```

### “variable de entorno ... no definida”

Export the variable before running:

```bash
export LOAD_PASSWORD="..."
```

`validate` does not resolve environment variables; the error appears at runtime.

### The target returns 429

It is rate limiting. Reduce VUs or the ramp slope, or coordinate with the service team. Diagnosis reports it as a likely cause.

### Latency is high but the server looks healthy

Check the **GENERATOR** card and the host summary. If local CPU is above 85–95%, the test machine may be the bottleneck. Run from another machine or reduce logging.

### `body_too_large`

The response exceeds the analysis limit (1 MiB) or discard limit (10 MiB). Siafu closes the connection and marks the request as failed. Adjust the endpoint or avoid `body_contains`/`extract` on large responses.

### I cannot see the TUI

stdout is not a TTY or you passed `--no-tui`. Use an interactive terminal or check plain-text mode.

### I need more detail

Run with `--debug` and inspect stderr. The TUI keeps the last 1,000 log lines; `X` shows them full-screen.

## Security

- **Authorization:** never test third-party systems without permission. Load can cause outages.
- **Secrets:** use `${ENV:...}` for credentials. Do not commit them in YAML.
- **Logs:** Siafu does not log request/response bodies and masks sensitive headers, sensitive query parameters, and referenced environment values.
- **Network surface:** the binary connects to the URLs you declare (including absolute URLs), redirects, and the environment proxy.
- **Limits:** VU, in-flight, duration, memory, and body caps exist to prevent accidents, not to replace your judgment.
- **Dependencies:** the libraries used are AGPLv3-compatible (MIT/BSD/Apache-2.0). `make vuln` runs `govulncheck` over source and binary; CI does it on every PR and daily.
- **Vulnerability reports:** follow [SECURITY.md](SECURITY.md); do not open a public issue with exploit details.
- **AGPL and networks:** if you modify Siafu and offer it as a network service, section 13 of the AGPL requires you to offer the corresponding source to users.

## Roadmap

Ideas for later versions, with no committed order:

- CSV/Parquet export and advanced run-to-run comparison;
- scenario weights;
- a unique-user session store for open-loop;
- target-rate auto-tuning and per-stage p95;
- gRPC;
- distributed execution;
- mTLS and richer auth flows;
- more checks (JSONPath, headers, schemas);
- reusable profiles and templates;
- Prometheus/Grafana integration and CI artifacts.

## Contributing

Read [CONTRIBUTING.md](CONTRIBUTING.md) and the [code of conduct](CODE_OF_CONDUCT.md). In short:

1. Open an issue describing the use case or bug.
2. Fork and create a branch (`feat/...` or `fix/...`).
3. Follow TDD: write the failing test first, then the fix.
4. Run `make check`, `make e2e`, and `make vuln` when dependencies are touched.
5. Open a Pull Request with a clear description and the reasoning behind your decisions.

Contributions incompatible with AGPL-3.0-or-later are not accepted. New files must include:

```go
// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors
```

## License

Siafu is distributed under the **GNU Affero General Public License, version 3 or later** (`AGPL-3.0-or-later`).

- Full text: [`LICENSE`](LICENSE)
- Official license: <https://www.gnu.org/licenses/agpl-3.0.html>

In short: you may use, study, modify, and distribute Siafu, but if you distribute a modified version or offer it as a network service, you must make the corresponding source available to users under the same license. There is no warranty.

---

Built with Go, Bubble Tea, Lipgloss, gopsutil, and HDR Histogram.
