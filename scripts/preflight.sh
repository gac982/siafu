#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Siafu contributors
#
# Preflight de límites del sistema para 15 000 conexiones concurrentes.
set -euo pipefail

OK=0
WARN=0
FAIL=0

ok() { printf 'OK    %s\n' "$*"; OK=$((OK + 1)); }
warn() { printf 'WARN  %s\n' "$*"; WARN=$((WARN + 1)); }
fail() { printf 'FAIL  %s\n' "$*"; FAIL=$((FAIL + 1)); }

need_ulimit=32768
have_ulimit=$(ulimit -n)
if [ "$have_ulimit" -ge "$need_ulimit" ]; then
  ok "ulimit -n = $have_ulimit (>= $need_ulimit)"
else
  fail "ulimit -n = $have_ulimit (< $need_ulimit); ejecuta: ulimit -n $need_ulimit"
fi

need_ports=20000
port_range=/proc/sys/net/ipv4/ip_local_port_range
if [ -r "$port_range" ]; then
  read -r port_min port_max <"$port_range"
  ports=$((port_max - port_min + 1))
  if [ "$ports" -ge "$need_ports" ]; then
    ok "puertos efímeros = $ports ($port_min-$port_max, >= $need_ports)"
  else
    fail "puertos efímeros = $ports (< $need_ports); amplía net.ipv4.ip_local_port_range"
  fi
else
  fail "no se puede leer $port_range"
fi

check_sysctl() {
  local path=$1 need=$2 name=$3
  if [ -r "$path" ]; then
    local value
    value=$(cat "$path")
    if [ "$value" -ge "$need" ]; then
      ok "$name = $value (>= $need)"
    else
      warn "$name = $value (< $need); recomendado: sysctl -w ${path#/proc/sys/}=$need"
    fi
  else
    warn "$name no disponible en $path"
  fi
}

check_sysctl /proc/sys/net/core/somaxconn 4096 net.core.somaxconn
check_sysctl /proc/sys/net/ipv4/tcp_max_syn_backlog 2048 net.ipv4.tcp_max_syn_backlog
check_sysctl /proc/sys/net/netfilter/nf_conntrack_max 262144 net.netfilter.nf_conntrack_max

printf '\nPreflight: %d OK, %d WARN, %d FAIL\n' "$OK" "$WARN" "$FAIL"
if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
