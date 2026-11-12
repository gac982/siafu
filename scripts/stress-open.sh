#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Siafu contributors
#
# Ejecuta el perfil open-loop contra el demo local y mide RSS y descartes.
set -euo pipefail

cd "$(dirname "$0")/.."

PORT="${SIAFU_STRESS_PORT:-18081}"
BASE="http://127.0.0.1:${PORT}"
TMP="$(mktemp -d)"
DEMO_PID=""
SIAFU_PID=""
RSS_MAX=0
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

sed "s#http://127.0.0.1:8080#${BASE}#" examples/stress-api-open.yaml >"$TMP/open.yaml"
./bin/siafu validate "$TMP/open.yaml" >/dev/null

./bin/siafu run --no-tui --no-save "$TMP/open.yaml" >"$TMP/run.out" 2>"$TMP/run.err" &
SIAFU_PID=$!

while kill -0 "$SIAFU_PID" 2>/dev/null; do
  if [ -r "/proc/${SIAFU_PID}/status" ]; then
    hwm_kb=$(awk '/^VmHWM:/{print $2}' "/proc/${SIAFU_PID}/status" 2>/dev/null || true)
    rss_mb=$(( ${hwm_kb:-0} / 1024 ))
    if [ "$rss_mb" -gt "$RSS_MAX" ]; then
      RSS_MAX=$rss_mb
    fi
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
failures=$(awk -F: '/Fallidas:/{print $2; exit}' "$TMP/run.out" | awk '{print $1}')
failures=${failures:-0}
rps=$(awk '{for (i = 1; i < NF; i++) if ($i == "promedio") { print $(i + 1); exit }}' "$TMP/run.out")
rps=${rps:-0}
p95=$(awk -F'p95 ' '/p95 /{print $2; exit}' "$TMP/run.out" | awk '{print $1}')
p95=${p95:-?}
target=$(awk -F: '/Tasa objetivo:/{print $2; exit}' "$TMP/run.out" | awk '{print $1}')
target=${target:-?}
achieved=$(awk -F: '/Tasa media objetivo:/{print $2; exit}' "$TMP/run.out" | awk '{print $1}')
if [ -z "$achieved" ] || [ "$achieved" = "?" ]; then
  achieved=$(awk -F: '/Tasa alcanzada:/{print $2; exit}' "$TMP/run.out" | awk '{print $1}')
fi
achieved=${achieved:-?}
inflight=$(awk -F: '/In-flight pico:/{print $2; exit}' "$TMP/run.out" | awk '{print $1}')
inflight=${inflight:-?}
dropped=$(awk -F: '/Descartes:/{print $2; exit}' "$TMP/run.out" | awk '{print $1}')
dropped=${dropped:-?}

printf '\nRESUMEN OPEN LOOP\n'
printf '  Tasa objetivo:     %s iteraciones/s\n' "$target"
printf '  Tasa alcanzada:    %s iteraciones/s\n' "$achieved"
printf '  In-flight pico:    %s\n' "$inflight"
printf '  Descartes:         %s\n' "$dropped"
printf '  RSS pico:          %s MiB\n' "$RSS_MAX"
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
