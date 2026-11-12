#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Siafu contributors
#
# Ejecuta el perfil de estrés contra el demo local y mide RSS y conexiones.
# Uso: VUS=10000 bash scripts/stress-15k.sh
set -euo pipefail

cd "$(dirname "$0")/.."

VUS="${VUS:-15000}"
PORT="${SIAFU_STRESS_PORT:-18080}"
BASE="http://127.0.0.1:${PORT}"
TMP="$(mktemp -d)"
DEMO_PID=""
SIAFU_PID=""
RSS_MAX=0
CONN_MAX=0
EXIT_CODE=0

cleanup() {
  if [ -n "$SIAFU_PID" ]; then
    kill "$SIAFU_PID" 2>/dev/null || true
    wait "$SIAFU_PID" 2>/dev/null || true
    SIAFU_PID=""
  fi
  if [ -n "$DEMO_PID" ]; then
    kill "$DEMO_PID" 2>/dev/null || true
    wait "$DEMO_PID" 2>/dev/null || true
    DEMO_PID=""
  fi
  rm -rf "$TMP"
}
trap cleanup EXIT
trap 'exit 130' INT TERM

make build demo-build >/dev/null

if command -v ss >/dev/null 2>&1 && ss -ltn "sport = :${PORT}" 2>/dev/null | grep -q LISTEN; then
  echo "el puerto ${PORT} ya está en uso; usa SIAFU_STRESS_PORT=<otro>" >&2
  exit 1
fi

./bin/siafu-demo -addr "127.0.0.1:${PORT}" >"$TMP/demo.log" 2>&1 &
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
if [ "$ready" -ne 1 ] || ! kill -0 "$DEMO_PID" 2>/dev/null; then
  echo "el servidor demo no arrancó (¿puerto ${PORT} ocupado?)" >&2
  cat "$TMP/demo.log" >&2 || true
  exit 1
fi

sed -e "s#http://127.0.0.1:8080#${BASE}#" \
    -e "s/15000/${VUS}/g" \
    examples/stress-15000.yaml >"$TMP/stress.yaml"

./bin/siafu validate "$TMP/stress.yaml" >/dev/null

./bin/siafu run --no-tui --no-save "$TMP/stress.yaml" >"$TMP/run.out" 2>"$TMP/run.err" &
SIAFU_PID=$!

while kill -0 "$SIAFU_PID" 2>/dev/null; do
  if [ -r "/proc/${SIAFU_PID}/status" ]; then
    hwm_kb=$(awk '/^VmHWM:/{print $2}' "/proc/${SIAFU_PID}/status" 2>/dev/null || true)
    rss_mb=$(( ${hwm_kb:-0} / 1024 ))
    if [ "$rss_mb" -gt "$RSS_MAX" ]; then
      RSS_MAX=$rss_mb
    fi
  fi

  conns=$(ss -Htan state established "dport = :${PORT}" 2>/dev/null | wc -l || true)
  conns=$((conns + 0))
  if [ "$conns" -gt "$CONN_MAX" ]; then
    CONN_MAX=$conns
  fi
  sleep 0.2
done

set +e
wait "$SIAFU_PID"
EXIT_CODE=$?
set -e
SIAFU_PID=""

requests=$(awk -F: '/Peticiones:/{gsub(/[^0-9]/,"",$2); print $2; exit}' "$TMP/run.out")
requests=${requests:-0}
failures=$(awk -F: '/Fallidas:/{print $2; exit}' "$TMP/run.out")
failures=${failures:-0}
rps=$(awk '{for (i = 1; i < NF; i++) if ($i == "promedio") { print $(i + 1); exit }}' "$TMP/run.out")
rps=${rps:-0}
p95=$(awk -F'p95 ' '/p95 /{print $2; exit}' "$TMP/run.out" | awk '{print $1}')
p95=${p95:-?}

printf '\nRESUMEN ESTRÉS\n'
printf '  VUs objetivo:      %s\n' "$VUS"
printf '  RSS pico:          %s MiB\n' "$RSS_MAX"
printf '  Conexiones pico:   %s\n' "$CONN_MAX"
printf '  Peticiones:        %s\n' "$requests"
printf '  Fallidas:          %s\n' "$failures"
printf '  RPS promedio:      %s\n' "$rps"
printf '  p95:               %s\n' "$p95"
printf '  Exit:              %s\n' "$EXIT_CODE"

if [ "$EXIT_CODE" -ne 0 ]; then
  printf '\n--- stderr ---\n'
  tail -20 "$TMP/run.err" || true
  printf '\n--- stdout (cola) ---\n'
  tail -40 "$TMP/run.out" || true
fi

exit "$EXIT_CODE"
