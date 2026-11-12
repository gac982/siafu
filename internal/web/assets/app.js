// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

import { lineChart } from "./charts.js";
import {
  escapeHTML,
  formatBytes,
  formatDate,
  formatDuration,
  formatNumber,
  formatPercent,
} from "./format.js";

const app = document.getElementById("app");
const filters = { q: "", mode: "", verdict: "" };
let runsCache = null;

window.addEventListener("hashchange", () => render());
window.addEventListener("DOMContentLoaded", () => render());

async function getJSON(url) {
  const response = await fetch(url, { headers: { Accept: "application/json" } });
  if (!response.ok) {
    throw new Error(`${response.status} ${response.statusText}`);
  }
  return response.json();
}

function parseRoute() {
  const hash = location.hash || "#/";
  if (hash.startsWith("#/runs/")) {
    return { name: "detail", id: decodeURIComponent(hash.slice("#/runs/".length)) };
  }
  if (hash.startsWith("#/compare")) {
    const query = hash.includes("?") ? hash.slice(hash.indexOf("?") + 1) : "";
    const params = new URLSearchParams(query);
    return { name: "compare", a: params.get("a"), b: params.get("b") };
  }
  return { name: "list" };
}

async function render() {
  const loading = document.createElement("div");
  loading.className = "hero";
  loading.innerHTML = `<img src="/logo.png" alt="Siafu"><p class="empty">Cargando…</p>`;
  app.replaceChildren(loading);

  try {
    const route = parseRoute();
    if (route.name === "detail") {
      await renderDetail(route.id);
    } else if (route.name === "compare") {
      await renderCompare(route.a, route.b);
    } else {
      await renderList();
    }
  } catch (error) {
    const message = document.createElement("p");
    message.className = "error";
    message.textContent = `No se pudieron cargar los resultados: ${error.message}`;
    app.replaceChildren(message);
  }
}

async function loadRuns() {
  if (!runsCache) {
    runsCache = await getJSON("/api/runs");
  }
  return runsCache;
}

function verdictClass(verdict) {
  switch (verdict) {
    case "PASS":
      return "ok";
    case "FAIL":
    case "ERROR":
      return "bad";
    case "ABORTED":
      return "warn";
    default:
      return "muted";
  }
}

function badge(verdict) {
  return `<span class="badge ${verdictClass(verdict)}">${escapeHTML(verdict || "—")}</span>`;
}

function backLink() {
  return `<nav class="back"><a href="#/">← Corridas</a></nav>`;
}

function runCard(run) {
  return `
    <a class="run-card" href="#/runs/${encodeURIComponent(run.run_id)}">
      <div class="run-card-head">
        ${badge(run.verdict)}
        <strong>${escapeHTML(run.name)}</strong>
      </div>
      <div class="run-card-meta">
        ${escapeHTML(formatDate(run.created_at))} · ${escapeHTML(run.mode)}${run.tag ? ` · ${escapeHTML(run.tag)}` : ""}
      </div>
      <div class="run-card-stats">
        <span><b>${formatDuration(run.p95_ms)}</b> p95</span>
        <span><b>${formatPercent(run.error_rate)}</b> errores</span>
        <span><b>${formatNumber(run.rps_avg)}</b> RPS</span>
        <span><b>${formatNumber(run.requests, 0)}</b> peticiones</span>
      </div>
    </a>`;
}

function renderRunCards(runs) {
  const list = document.getElementById("run-list");
  if (!list) return;
  const query = filters.q.toLowerCase();
  const visible = runs.filter((run) => {
    if (filters.mode && run.mode !== filters.mode) return false;
    if (filters.verdict && run.verdict !== filters.verdict) return false;
    if (query && !`${run.name} ${run.tag || ""} ${run.run_id}`.toLowerCase().includes(query)) return false;
    return true;
  });
  if (!runs.length) {
    list.innerHTML = `<div class="hero"><img src="/logo.png" alt="Siafu"><p class="empty">Aún no hay corridas guardadas.</p></div>`;
    return;
  }
  if (!visible.length) {
    list.innerHTML = `<p class="empty">No hay corridas que coincidan con los filtros.</p>`;
    return;
  }
  list.innerHTML = `<div class="run-grid">${visible.map(runCard).join("")}</div>`;
}

async function renderList() {
  const runs = await loadRuns();
  app.innerHTML = `
    <div class="page-actions">
      <h1>Corridas guardadas</h1>
      <button id="refresh-runs" class="raw" type="button">Refrescar</button>
    </div>
    <section class="filters" aria-label="Filtros">
      <input id="filter-q" type="search" placeholder="Buscar por nombre, tag o id" value="${escapeHTML(filters.q)}">
      <select id="filter-mode">
        <option value="">Todos los modos</option>
        <option value="closed"${filters.mode === "closed" ? " selected" : ""}>cerrado</option>
        <option value="open"${filters.mode === "open" ? " selected" : ""}>open-loop</option>
      </select>
      <select id="filter-verdict">
        <option value="">Todos los veredictos</option>
        ${["PASS", "FAIL", "ABORTED", "ERROR", "NOT_EVALUATED"].map((verdict) => `<option value="${verdict}"${filters.verdict === verdict ? " selected" : ""}>${verdict}</option>`).join("")}
      </select>
    </section>
    <div id="run-list"></div>`;

  renderRunCards(runs);
  document.getElementById("filter-q").addEventListener("input", (event) => {
    filters.q = event.target.value;
    renderRunCards(runs);
  });
  document.getElementById("filter-mode").addEventListener("change", (event) => {
    filters.mode = event.target.value;
    renderRunCards(runs);
  });
  document.getElementById("filter-verdict").addEventListener("change", (event) => {
    filters.verdict = event.target.value;
    renderRunCards(runs);
  });
  bindRefresh();
}

function bindRefresh() {
  const button = document.getElementById("refresh-runs");
  if (!button) return;
  button.addEventListener("click", () => {
    runsCache = null;
    render();
  });
}

function kpi(label, value, hint = "") {
  return `<div class="kpi"><span>${escapeHTML(label)}</span><strong>${value}</strong>${hint ? `<small>${escapeHTML(hint)}</small>` : ""}</div>`;
}

function protocolSummary(protocols) {
  const entries = Object.entries(protocols || {});
  if (entries.length === 0) return "—";
  return entries.map(([name, count]) => `${escapeHTML(name)}: ${formatNumber(count, 0)}`).join(" · ");
}

function chartCard(title, chart) {
  const article = document.createElement("article");
  article.className = "chart-card";
  const heading = document.createElement("h3");
  heading.textContent = title;
  article.append(heading, chart);
  return article;
}

function points(series, key) {
  return (series || [])
    .map((point) => ({ x: point.t_ms / 1000, y: point[key] }))
    .filter((point) => Number.isFinite(point.y));
}

function thresholdFor(run, name, scale = 1) {
  const threshold = (run.thresholds || []).find((item) => item.name === name);
  if (!threshold || !Number.isFinite(threshold.limit) || threshold.limit <= 0) {
    return null;
  }
  return { value: threshold.limit * scale, label: `${threshold.name} ${threshold.expected}` };
}

function thresholdTable(run) {
  const incomplete = run.aborted || run.memory_exceeded;
  if (!run.thresholds || run.thresholds.length === 0) {
    return `<p class="empty">${incomplete ? "No evaluados (ejecución incompleta)." : "Sin umbrales definidos."}</p>`;
  }
  return `<div class="table-wrap"><table>
    <thead><tr><th>Criterio</th><th>Esperado</th><th>Observado</th><th>Estado</th></tr></thead>
    <tbody>${run.thresholds.map((threshold) => {
      let status;
      if (incomplete || threshold.evaluated === false) {
        status = '<span class="badge muted">NO EVALUADO</span>';
      } else {
        status = threshold.pass ? '<span class="badge ok">OK</span>' : '<span class="badge bad">FALLA</span>';
      }
      return `
      <tr>
        <td>${escapeHTML(threshold.name)}</td>
        <td>${escapeHTML(threshold.expected)}</td>
        <td>${escapeHTML(threshold.observed)}</td>
        <td>${status}</td>
      </tr>`;
    }).join("")}</tbody>
  </table></div>`;
}

function diagnosisList(run) {
  if (!run.diagnosis || run.diagnosis.length === 0) {
    return `<p class="empty">Sin hallazgos.</p>`;
  }
  return `<ul class="diagnosis">${run.diagnosis.map((finding) => `
    <li class="severity ${escapeHTML(finding.severity)}">
      <span>${escapeHTML(finding.severity)}</span>${escapeHTML(finding.message)}
    </li>`).join("")}</ul>`;
}

function scenarioTable(run) {
  if (!run.scenarios || run.scenarios.length === 0) {
    return `<p class="empty">Sin escenarios.</p>`;
  }
  return `<div class="table-wrap"><table>
    <thead><tr><th>Escenario</th><th>Peticiones</th><th>Fallidas</th><th>p95</th><th>p99</th><th>RPS</th></tr></thead>
    <tbody>${run.scenarios.map((scenario) => `
      <tr>
        <td>${escapeHTML(scenario.name)}</td>
        <td>${formatNumber(scenario.requests, 0)}</td>
        <td>${formatNumber(scenario.failed, 0)} (${formatPercent(scenario.error_rate)})</td>
        <td>${formatDuration(scenario.p95_ms)}</td>
        <td>${formatDuration(scenario.p99_ms)}</td>
        <td>${formatNumber(scenario.rps_avg)}</td>
      </tr>`).join("")}</tbody>
  </table></div>`;
}

function statusTable(run) {
  const codes = Object.entries(run.global.status_codes || {}).sort((a, b) => Number(a[0]) - Number(b[0]));
  if (codes.length === 0) {
    return `<p class="empty">Sin códigos de estado.</p>`;
  }
  return `<div class="table-wrap"><table>
    <thead><tr><th>Código</th><th>Peticiones</th></tr></thead>
    <tbody>${codes.map(([code, count]) => `<tr><td>${escapeHTML(code)}</td><td>${formatNumber(count, 0)}</td></tr>`).join("")}</tbody>
  </table></div>`;
}

function stageTable(run) {
  const stages = run.open_loop?.stages || [];
  if (stages.length === 0) {
    return "";
  }
  return `<h2>Etapas open-loop</h2><div class="table-wrap"><table>
    <thead><tr><th>Etapa</th><th>Objetivo</th><th>Alcanzado</th><th>Peticiones</th><th>Fallidas</th><th>Descartes</th></tr></thead>
    <tbody>${stages.map((stage) => {
      const achieved = stage.duration_ms > 0 ? (stage.iterations / stage.duration_ms) * 1000 : 0;
      return `<tr>
        <td>${stage.index + 1}</td>
        <td>${formatNumber(stage.target_rate)}/s</td>
        <td>${formatNumber(achieved)}/s</td>
        <td>${formatNumber(stage.requests, 0)}</td>
        <td>${formatNumber(stage.failed, 0)}</td>
        <td>${formatNumber(stage.dropped, 0)}</td>
      </tr>`;
    }).join("")}</tbody>
  </table></div>`;
}

async function renderDetail(id) {
  const [run, series] = await Promise.all([
    getJSON(`/api/runs/${encodeURIComponent(id)}`),
    getJSON(`/api/runs/${encodeURIComponent(id)}/series`),
  ]);
  const open = run.mode === "open";
  const runId = encodeURIComponent(run.run_id);

  app.innerHTML = `
    ${backLink()}
    <section class="run-header">
      <div>
        ${badge(run.verdict)}
        <h1>${escapeHTML(run.name)}</h1>
        <p class="muted">${escapeHTML(run.run_id)} · ${escapeHTML(run.mode)} · ${escapeHTML(formatDate(run.created_at))}${run.tag ? ` · ${escapeHTML(run.tag)}` : ""} · semilla ${escapeHTML(String(run.seed ?? "—"))}</p>
        <p class="muted">${escapeHTML(run.source || "sin fuente")} · objetivo ${escapeHTML(run.target || "—")}</p>
        ${run.aborted ? '<p class="warn">Ejecución abortada; métricas parciales.</p>' : ""}
        ${run.memory_exceeded ? '<p class="warn">Detenida por límite de memoria.</p>' : ""}
      </div>
      <div class="run-actions">
        <a class="raw" href="#/compare?a=${runId}">Comparar…</a>
        <a class="raw" href="/api/runs/${runId}" target="_blank" rel="noreferrer">JSON</a>
      </div>
    </section>
    <section class="kpis">
      ${kpi("Peticiones", formatNumber(run.global.requests, 0))}
      ${kpi("Errores", formatPercent(run.global.error_rate), `${formatNumber(run.global.failed, 0)} fallidas`)}
      ${kpi("Latencia p95", formatDuration(run.global.p95_ms), `p99 ${formatDuration(run.global.p99_ms)}`)}
      ${kpi("RPS medio", formatNumber(run.global.rps_avg), `pico ${formatNumber(run.global.rps_peak)}`)}
      ${kpi("Iteraciones", formatNumber(run.global.iterations, 0), `${formatNumber(run.global.failed_iterations, 0)} fallidas`)}
      ${kpi("Protocolos", protocolSummary(run.global.protocols))}
      ${kpi("Duración", formatDuration(run.actual_ms), `planificada ${formatDuration(run.planned_ms)}`)}
      ${open ? kpi("In-flight pico", formatNumber(run.open_loop?.peak_in_flight || 0, 0)) : ""}
      ${open ? kpi("Descartes", formatNumber(run.open_loop?.dropped || 0, 0)) : ""}
      ${kpi("Bytes", formatBytes(run.global.transferred_bytes))}
    </section>
    <section id="charts" class="charts-grid"></section>
    <h2>Umbrales</h2>
    ${thresholdTable(run)}
    <h2>Diagnóstico</h2>
    ${diagnosisList(run)}
    <h2>Escenarios</h2>
    ${scenarioTable(run)}
    ${open ? stageTable(run) : ""}
    <h2>Códigos de estado</h2>
    ${statusTable(run)}`;

  const charts = document.createElement("div");
  charts.className = "charts-grid";
  charts.append(chartCard("RPS", lineChart({
    series: [{ name: "RPS", points: points(series, "rps") }],
    xLabel: "tiempo (s)",
    yLabel: "RPS",
    ariaLabel: "RPS en el tiempo",
    description: "RPS en el tiempo",
    threshold: thresholdFor(run, "RPS promedio"),
  })));
  charts.append(chartCard("Latencia", lineChart({
    series: [
      { name: "p95", points: points(series, "p95_ms") },
      { name: "p99", points: points(series, "p99_ms") },
    ],
    xLabel: "tiempo (s)",
    yLabel: "ms",
    formatY: (value) => formatDuration(value),
    ariaLabel: "Latencia p95 y p99 en el tiempo",
    description: "Latencia p95 y p99 en el tiempo",
    threshold: thresholdFor(run, "latencia p95"),
  })));
  charts.append(chartCard("Errores", lineChart({
    series: [{ name: "error rate", points: series.map((point) => ({ x: point.t_ms / 1000, y: point.error_rate * 100 })) }],
    xLabel: "tiempo (s)",
    yLabel: "%",
    formatY: (value) => `${formatNumber(value, 2)}%`,
    ariaLabel: "Tasa de error en el tiempo",
    description: "Tasa de error en el tiempo",
    threshold: thresholdFor(run, "tasa de error", 100),
  })));

  const cpu = points(series, "cpu_percent");
  const memory = points(series, "mem_percent");
  if (cpu.length > 0 || memory.length > 0) {
    charts.append(chartCard("Generador", lineChart({
      series: [
        { name: "CPU", points: cpu, color: "var(--warn)" },
        { name: "RAM", points: memory, color: "var(--accent)" },
      ],
      xLabel: "tiempo (s)",
      yLabel: "%",
      formatY: (value) => `${formatNumber(value, 0)}%`,
      ariaLabel: "CPU y RAM del generador",
      description: "CPU y RAM del generador",
    })));
  }

  if (open) {
    charts.append(chartCard("Tasa", lineChart({
      series: [
        { name: "objetivo", points: points(series, "target_rate") },
        { name: "alcanzada", points: points(series, "achieved_rate") },
      ],
      xLabel: "tiempo (s)",
      yLabel: "iteraciones/s",
      ariaLabel: "Tasa objetivo y alcanzada",
      description: "Tasa objetivo y alcanzada",
    })));
    charts.append(chartCard("In-flight", lineChart({
      series: [{ name: "in-flight", points: points(series, "in_flight") }],
      xLabel: "tiempo (s)",
      yLabel: "workers",
      formatY: (value) => formatNumber(value, 0),
      ariaLabel: "Workers en vuelo",
      description: "Workers en vuelo",
    })));
    if (series.some((point) => point.dropped > 0)) {
      charts.append(chartCard("Descartes", lineChart({
        series: [{ name: "descartes", points: points(series, "dropped") }],
        xLabel: "tiempo (s)",
        yLabel: "iteraciones",
        formatY: (value) => formatNumber(value, 0),
        ariaLabel: "Iteraciones descartadas",
        description: "Iteraciones descartadas",
      })));
    }
  }

  const container = document.getElementById("charts");
  container.replaceWith(charts);
}

function comparisonSelect(id, runs, selected) {
  return `<select id="${id}">
    <option value="">Elegir corrida…</option>
    ${runs.map((run) => `<option value="${escapeHTML(run.run_id)}"${run.run_id === selected ? " selected" : ""}>${escapeHTML(run.name)} · ${escapeHTML(run.run_id)}</option>`).join("")}
  </select>`;
}

function signedDuration(delta) {
  return `${delta >= 0 ? "+" : "-"}${formatDuration(Math.abs(delta))}`;
}

function deltaRow(label, first, second, formatter, deltaFormatter, lowerIsBetter) {
  const delta = second - first;
  const good = delta === 0 ? "muted" : (lowerIsBetter ? delta < 0 : delta > 0) ? "good" : "bad";
  return `<tr>
    <td>${escapeHTML(label)}</td>
    <td>${formatter(first)} → ${formatter(second)}</td>
    <td class="${good}">${deltaFormatter(delta)}</td>
  </tr>`;
}

function signedNumber(value, digits = 1) {
  return `${value >= 0 ? "+" : ""}${formatNumber(value, digits)}`;
}

function signedPercent(rate) {
  return `${rate >= 0 ? "+" : "-"}${formatPercent(Math.abs(rate))}`;
}

function comparisonTable(a, b) {
  return `<div class="table-wrap"><table class="compare-table">
    <thead><tr><th>Métrica</th><th>A → B</th><th>Delta</th></tr></thead>
    <tbody>
      ${deltaRow("p95", a.global.p95_ms, b.global.p95_ms, formatDuration, signedDuration, true)}
      ${deltaRow("p99", a.global.p99_ms, b.global.p99_ms, formatDuration, signedDuration, true)}
      ${deltaRow("RPS medio", a.global.rps_avg, b.global.rps_avg, formatNumber, signedNumber, false)}
      ${deltaRow("Error rate", a.global.error_rate, b.global.error_rate, formatPercent, signedPercent, true)}
      ${deltaRow("Peticiones", a.global.requests, b.global.requests, (value) => formatNumber(value, 0), (value) => signedNumber(value, 0), false)}
      <tr><td>Veredicto</td><td>${badge(a.verdict)} ${badge(b.verdict)}</td><td>${a.verdict === b.verdict ? "sin cambio" : `${escapeHTML(a.verdict)} → ${escapeHTML(b.verdict)}`}</td></tr>
    </tbody>
  </table></div>`;
}

async function renderCompare(aId, bId) {
  const runs = await loadRuns();
  if (runs.length < 2) {
    app.innerHTML = `${backLink()}<h1>Comparar corridas</h1><p class="empty">Necesitas al menos dos corridas guardadas.</p>`;
    return;
  }
  const first = aId || runs[1].run_id;
  const second = bId || runs.find((run) => run.run_id !== first)?.run_id || runs[0].run_id;

  app.innerHTML = `
    ${backLink()}
    <h1>Comparar corridas</h1>
    <section class="filters">
      ${comparisonSelect("compare-a", runs, first)}
      ${comparisonSelect("compare-b", runs, second)}
      <button id="refresh-runs" class="raw" type="button">Refrescar</button>
    </section>
    <div id="compare-result"></div>`;
  bindRefresh();

  const update = () => {
    const a = document.getElementById("compare-a").value;
    const b = document.getElementById("compare-b").value;
    if (a && b) {
      location.hash = `#/compare?a=${encodeURIComponent(a)}&b=${encodeURIComponent(b)}`;
    }
  };
  document.getElementById("compare-a").addEventListener("change", update);
  document.getElementById("compare-b").addEventListener("change", update);

  if (!aId || !bId) {
    location.hash = `#/compare?a=${encodeURIComponent(first)}&b=${encodeURIComponent(second)}`;
    return;
  }

  const [a, b, seriesA, seriesB] = await Promise.all([
    getJSON(`/api/runs/${encodeURIComponent(aId)}`),
    getJSON(`/api/runs/${encodeURIComponent(bId)}`),
    getJSON(`/api/runs/${encodeURIComponent(aId)}/series`),
    getJSON(`/api/runs/${encodeURIComponent(bId)}/series`),
  ]);

  const result = document.getElementById("compare-result");
  result.innerHTML = `
    <section class="compare-head">
      <div><span class="badge muted">A</span> ${escapeHTML(a.name)} <small>${escapeHTML(a.run_id)}</small></div>
      <div><span class="badge muted">B</span> ${escapeHTML(b.name)} <small>${escapeHTML(b.run_id)}</small></div>
    </section>
    ${comparisonTable(a, b)}
    <div id="compare-charts" class="charts-grid"></div>`;

  const charts = document.createElement("div");
  charts.className = "charts-grid";
  charts.append(chartCard("RPS", lineChart({
    series: [
      { name: `A · ${a.name}`, points: points(seriesA, "rps") },
      { name: `B · ${b.name}`, points: points(seriesB, "rps"), color: "var(--warn)" },
    ],
    xLabel: "tiempo (s)",
    yLabel: "RPS",
    ariaLabel: "RPS de ambas corridas",
    description: "RPS de ambas corridas",
  })));
  charts.append(chartCard("p95", lineChart({
    series: [
      { name: `A · ${a.name}`, points: points(seriesA, "p95_ms") },
      { name: `B · ${b.name}`, points: points(seriesB, "p95_ms"), color: "var(--warn)" },
    ],
    xLabel: "tiempo (s)",
    yLabel: "ms",
    formatY: (value) => formatDuration(value),
    ariaLabel: "p95 de ambas corridas",
    description: "p95 de ambas corridas",
  })));
  document.getElementById("compare-charts").replaceWith(charts);
}
