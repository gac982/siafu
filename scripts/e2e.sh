#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Siafu contributors
set -euo pipefail

cd "$(dirname "$0")/.."

PORT="${SIAFU_DEMO_PORT:-8080}"
BASE="http://127.0.0.1:${PORT}"
TMPDIR_E2E="$(mktemp -d)"
RESULTS="$TMPDIR_E2E/results"
DEMO_PID=""

cleanup() {
  if [ -n "$DEMO_PID" ]; then
    kill "$DEMO_PID" 2>/dev/null || true
    wait "$DEMO_PID" 2>/dev/null || true
  fi
  rm -rf "$TMPDIR_E2E"
}
trap cleanup EXIT

make build demo-build >/dev/null

./bin/siafu-demo -addr "127.0.0.1:${PORT}" >"$TMPDIR_E2E/demo.log" 2>&1 &
DEMO_PID=$!

# Wait for the demo server without failing the script on the first attempts.
ready=0
for _ in $(seq 1 50); do
  if curl -fsS "${BASE}/" >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 0.1
done
if [ "$ready" -ne 1 ]; then
  echo "demo server did not start" >&2
  cat "$TMPDIR_E2E/demo.log" >&2 || true
  exit 1
fi

sed "s#http://127.0.0.1:8080#${BASE}#" examples/demo.yaml >"$TMPDIR_E2E/demo.yaml"
sed "s#http://127.0.0.1:8080#${BASE}#" examples/failing.yaml >"$TMPDIR_E2E/failing.yaml"

./bin/siafu validate "$TMPDIR_E2E/demo.yaml" >/dev/null

set +e
./bin/siafu run --no-tui --results-dir "$RESULTS" "$TMPDIR_E2E/demo.yaml" >"$TMPDIR_E2E/pass.out" 2>"$TMPDIR_E2E/pass.err"
PASS_CODE=$?
./bin/siafu run --no-tui --results-dir "$RESULTS" "$TMPDIR_E2E/failing.yaml" >"$TMPDIR_E2E/fail.out" 2>"$TMPDIR_E2E/fail.err"
FAIL_CODE=$?
set -e

if [ "$PASS_CODE" -ne 0 ]; then
  echo "expected demo run to pass, got exit $PASS_CODE" >&2
  tail -40 "$TMPDIR_E2E/pass.out" >&2 || true
  cat "$TMPDIR_E2E/pass.err" >&2 || true
  exit 1
fi
if ! grep -q "APROBADA" "$TMPDIR_E2E/pass.out"; then
  echo "passing run did not print APROBADA" >&2
  tail -40 "$TMPDIR_E2E/pass.out" >&2 || true
  exit 1
fi
if [ "$FAIL_CODE" -ne 1 ]; then
  echo "expected failing run to exit 1, got $FAIL_CODE" >&2
  tail -40 "$TMPDIR_E2E/fail.out" >&2 || true
  cat "$TMPDIR_E2E/fail.err" >&2 || true
  exit 1
fi
if ! grep -q "FALLIDA" "$TMPDIR_E2E/fail.out"; then
  echo "failing run did not print FALLIDA" >&2
  tail -40 "$TMPDIR_E2E/fail.out" >&2 || true
  exit 1
fi

cat >"$TMPDIR_E2E/open.yaml" <<EOF
version: 1
name: open-e2e
target:
  base_url: "${BASE}"
  timeout: 5s
load:
  mode: open
  open_loop:
    stages:
      - {duration: 500ms, target_rate: 500}
      - {duration: 500ms, target_rate: 500}
    max_in_flight: 100
    on_overload: drop
    on_analysis_exhausted: skip
scenarios:
  - name: home
    steps:
      - name: portada
        request: {method: GET, url: /}
        expect: {status: 200}
EOF
./bin/siafu validate "$TMPDIR_E2E/open.yaml" >/dev/null

set +e
./bin/siafu run --no-tui --results-dir "$RESULTS" "$TMPDIR_E2E/open.yaml" >"$TMPDIR_E2E/open.out" 2>"$TMPDIR_E2E/open.err"
OPEN_CODE=$?
set -e

if [ "$OPEN_CODE" -ne 0 ]; then
  echo "expected open-loop run to exit 0, got $OPEN_CODE" >&2
  tail -40 "$TMPDIR_E2E/open.out" >&2 || true
  cat "$TMPDIR_E2E/open.err" >&2 || true
  exit 1
fi
if ! grep -q "MODO OPEN LOOP" "$TMPDIR_E2E/open.out"; then
  echo "open-loop run did not print MODO OPEN LOOP" >&2
  tail -40 "$TMPDIR_E2E/open.out" >&2 || true
  exit 1
fi

sed "s#http://127.0.0.1:8080#${BASE}#" examples/websocket.yaml >"$TMPDIR_E2E/websocket.yaml"
./bin/siafu validate "$TMPDIR_E2E/websocket.yaml" >/dev/null
set +e
./bin/siafu run --no-tui --results-dir "$RESULTS" "$TMPDIR_E2E/websocket.yaml" >"$TMPDIR_E2E/websocket.out" 2>"$TMPDIR_E2E/websocket.err"
WS_CODE=$?
set -e
if [ "$WS_CODE" -ne 0 ]; then
  echo "expected WebSocket run to exit 0, got $WS_CODE" >&2
  tail -40 "$TMPDIR_E2E/websocket.out" >&2 || true
  cat "$TMPDIR_E2E/websocket.err" >&2 || true
  exit 1
fi
if ! grep -q "WebSocket" "$TMPDIR_E2E/websocket.out"; then
  echo "WebSocket run did not report the protocol" >&2
  tail -40 "$TMPDIR_E2E/websocket.out" >&2 || true
  exit 1
fi

if [ ! -d "$RESULTS" ]; then
  echo "results directory was not created: $RESULTS" >&2
  exit 1
fi

saved=$(find "$RESULTS" -name run.json | wc -l)
if [ "$saved" -lt 4 ]; then
  echo "expected 4 persisted runs, got $saved" >&2
  find "$RESULTS" -maxdepth 2 -type f >&2 || true
  exit 1
fi

echo "E2E OK: demo PASS (exit 0), failing config FAIL (exit 1), open-loop PASS (exit 0), WebSocket PASS (exit 0), saved $saved runs"
