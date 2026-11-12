#!/usr/bin/env node
// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors
//
// Dev-only browser check. It is NOT part of make check.
//
// Setup:
//   npm install playwright
//   npx playwright install firefox
// Then:
//   make build
//   node scripts/web-browser-check.js
//
// Optional:
//   SIAFU_WEB_BROWSER_PORT=18097

const fs = require("fs");
const os = require("os");
const path = require("path");
const { spawn } = require("child_process");

let firefox;
try {
  ({ firefox } = require("playwright"));
} catch {
  console.error("playwright no está instalado. Ejecuta: npm install playwright && npx playwright install firefox");
  process.exit(2);
}

const root = path.resolve(__dirname, "..");
const binary = path.join(root, "bin", "siafu");
const port = Number(process.env.SIAFU_WEB_BROWSER_PORT || 18097);
const base = `http://127.0.0.1:${port}`;
const xssName = `<img src=x onerror="window.__xss=1">`;

function fixture() {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "siafu-web-check-"));
  const runID = "xss-run";
  const runDir = path.join(dir, runID);
  fs.mkdirSync(runDir, { recursive: true });

  const createdAt = new Date().toISOString();
  const record = {
    schema_version: 1,
    run_id: runID,
    created_at: createdAt,
    name: xssName,
    tag: "<script>window.__xss=2</script>",
    mode: "closed",
    target: "http://127.0.0.1:1",
    source: "fixture.yaml",
    config_hash: "sha256:test",
    version: "test",
    commit: "test",
    seed: 1,
    planned_ms: 3600000,
    actual_ms: 3600000,
    verdict: "PASS",
    exit_code: 0,
    global: {
      requests: 3600,
      failed: 0,
      canceled: 0,
      iterations: 3600,
      failed_iterations: 0,
      error_rate: 0,
      rps_avg: 1,
      rps_peak: 2,
      p50_ms: 40,
      p90_ms: 45,
      p95_ms: 50,
      p99_ms: 80,
      min_ms: 10,
      max_ms: 100,
      mean_ms: 45,
      checks_total: 3600,
      checks_passed: 3600,
      transferred_bytes: 3600,
      analysis_skipped: 0,
      status_codes: { 200: 3600 },
      errors: [],
      check_failures: [],
    },
    scenarios: [],
    thresholds: [{
      name: "latencia p95",
      expected: "≤ 1s",
      observed: "50ms",
      pass: true,
      evaluated: true,
      limit: 1000,
      unit: "ms",
    }],
    diagnosis: [],
    host: { cpu_avg: 10, cpu_max: 20, mem_max_percent: 30, net_rx_peak: 1, net_tx_peak: 1, samples: 1 },
  };
  fs.writeFileSync(path.join(runDir, "run.json"), JSON.stringify(record));
  fs.writeFileSync(path.join(runDir, "series.jsonl"), Array.from({ length: 3600 }, (_, i) => JSON.stringify({
    t_ms: (i + 1) * 1000,
    rps: 1000 + i,
    p95_ms: 50,
    p99_ms: 80,
    error_rate: 0,
    cpu_percent: 10,
    mem_percent: 30,
  })).join("\n") + "\n");
  fs.writeFileSync(path.join(dir, "index.jsonl"), JSON.stringify({
    schema_version: 1,
    run_id: runID,
    created_at: createdAt,
    name: xssName,
    tag: "<script>window.__xss=2</script>",
    mode: "closed",
    verdict: "PASS",
    exit_code: 0,
    duration_ms: 3600000,
    requests: 3600,
    failed: 0,
    error_rate: 0,
    p95_ms: 50,
    rps_avg: 1,
  }) + "\n");
  return { dir, runID };
}

async function waitForServer() {
  const deadline = Date.now() + 5000;
  while (Date.now() < deadline) {
    try {
      const response = await fetch(`${base}/api/health`);
      if (response.ok) return;
    } catch {
      // retry
    }
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  throw new Error("el servidor no arrancó");
}

async function main() {
  if (!fs.existsSync(binary)) {
    throw new Error(`falta ${binary}; ejecuta make build`);
  }
  const { dir } = fixture();
  const server = spawn(binary, ["serve", "--addr", `127.0.0.1:${port}`, "--results-dir", dir], {
    stdio: ["ignore", "pipe", "pipe"],
  });
  const browser = await firefox.launch();
  const errors = [];
  try {
    await waitForServer();
    const page = await browser.newPage({ viewport: { width: 390, height: 844 } });
    page.on("pageerror", (error) => errors.push(String(error)));
    page.on("console", (message) => {
      if (message.type() === "error") errors.push(message.text());
    });

    await page.goto(base + "/", { waitUntil: "networkidle" });
    await page.waitForSelector(".run-card");
    const cards = await page.locator(".run-card").count();
    const listText = await page.locator("body").innerText();
    const listImages = await page.locator("img").count();

    await page.locator(".run-card").first().click();
    await page.waitForSelector(".charts-grid");
    const detailImages = await page.locator("img").count();
    const descCount = await page.locator("svg desc").count();
    const thresholdLines = await page.locator("line.threshold").count();
    const firstPath = await page.locator(".chart svg path").first().getAttribute("d");
    const segments = firstPath ? (firstPath.match(/[ML]/g) || []).length : 0;
    const xss = await page.evaluate(() => window.__xss);

    const result = {
      cards,
      escaped: listText.includes(xssName),
      listImages,
      detailImages,
      descCount,
      thresholdLines,
      segments,
      xss: xss ?? null,
      errors,
    };
    console.log(JSON.stringify(result, null, 2));

    if (cards !== 1) throw new Error("expected one fixture run");
    if (!result.escaped) throw new Error("run name was not rendered as text");
    if (listImages !== 0 || detailImages !== 0) throw new Error("XSS payload created an img element");
    if (xss !== null && xss !== undefined) throw new Error("XSS payload executed");
    if (descCount < 1) throw new Error("missing chart desc");
    if (thresholdLines < 1) throw new Error("missing threshold line");
    if (segments > 601) throw new Error(`series was not downsampled: ${segments} segments`);
    if (errors.length > 0) throw new Error(`browser errors: ${errors.join(" | ")}`);
  } finally {
    await browser.close();
    server.kill("SIGTERM");
    fs.rmSync(dir, { recursive: true, force: true });
  }
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
