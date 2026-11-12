#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Siafu contributors
#
# Smoke test for persistence + web API/UI assets.
set -euo pipefail

cd "$(dirname "$0")/.."

DEMO_PORT="${SIAFU_WEB_DEMO_PORT:-18082}"
SERVE_PORT="${SIAFU_WEB_SERVE_PORT:-18083}"
BASE="http://127.0.0.1:${DEMO_PORT}"
WEB="http://127.0.0.1:${SERVE_PORT}"
TMP="$(mktemp -d)"
DEMO_PID=""
SERVE_PID=""

cleanup() {
  if [ -n "$SERVE_PID" ]; then
    kill "$SERVE_PID" 2>/dev/null || true
    wait "$SERVE_PID" 2>/dev/null || true
  fi
  if [ -n "$DEMO_PID" ]; then
    kill "$DEMO_PID" 2>/dev/null || true
    wait "$DEMO_PID" 2>/dev/null || true
  fi
  rm -rf "$TMP"
}
trap cleanup EXIT
trap 'exit 130' INT TERM

make build demo-build >/dev/null

./bin/siafu-demo -addr "127.0.0.1:${DEMO_PORT}" >"$TMP/demo.log" 2>&1 &
DEMO_PID=$!

ready=0
for _ in $(seq 1 50); do
  if ! kill -0 "$DEMO_PID" 2>/dev/null; then
    break
  fi
  if curl -fsS "${BASE}/" >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 0.1
done
if [ "$ready" -ne 1 ]; then
  echo "el servidor demo no arrancó" >&2
  cat "$TMP/demo.log" >&2 || true
  exit 1
fi

cat >"$TMP/smoke.yaml" <<EOF
version: 1
name: web-smoke
target:
  base_url: "${BASE}"
  timeout: 2s
load:
  stages:
    - {duration: 1s, target_vus: 2}
scenarios:
  - name: home
    steps:
      - name: portada
        request: {method: GET, url: /}
        expect: {status: 200}
EOF

./bin/siafu run --no-tui --results-dir "$TMP/results" --tag web-smoke "$TMP/smoke.yaml" >"$TMP/run.out" 2>"$TMP/run.err"

./bin/siafu serve --addr "127.0.0.1:${SERVE_PORT}" --results-dir "$TMP/results" >"$TMP/serve.log" 2>&1 &
SERVE_PID=$!

ready=0
for _ in $(seq 1 50); do
  if ! kill -0 "$SERVE_PID" 2>/dev/null; then
    break
  fi
  if curl -fsS "${WEB}/api/health" >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 0.1
done
if [ "$ready" -ne 1 ]; then
  echo "el servidor web no arrancó" >&2
  cat "$TMP/serve.log" >&2 || true
  exit 1
fi

health=$(curl -fsS "${WEB}/api/health")
case "$health" in
  *'"status":"ok"'*) ;;
  *) echo "health inesperado: $health" >&2; exit 1 ;;
esac

runs=$(curl -fsS "${WEB}/api/runs")
case "$runs" in
  *'web-smoke'*) ;;
  *) echo "la corrida no aparece en /api/runs: $runs" >&2; exit 1 ;;
esac

run_id=$(printf '%s' "$runs" | python3 -c 'import json,sys; print(json.load(sys.stdin)[0]["run_id"])')
curl -fsS "${WEB}/api/runs/${run_id}" | grep -q '"run_id"'
curl -fsS "${WEB}/api/runs/${run_id}/series" | grep -q '"t_ms"'
curl -fsS "${WEB}/" | grep -q 'id="app"'
curl -fsS "${WEB}/app.js" | grep -q 'lineChart'
curl -fsS "${WEB}/charts.js" | grep -q 'lineChart'

echo "WEB SMOKE OK: $run_id"
