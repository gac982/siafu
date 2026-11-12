// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

import { formatNumber } from "./format.js";

const NS = "http://www.w3.org/2000/svg";
const CHART_WIDTH = 480;
const PALETTE = ["var(--accent)", "var(--warn)", "var(--bad)", "var(--ok)", "#a78bfa", "#60a5fa"];

function svgEl(name, attrs = {}) {
  const element = document.createElementNS(NS, name);
  for (const [key, value] of Object.entries(attrs)) {
    if (value !== undefined && value !== null) {
      element.setAttribute(key, String(value));
    }
  }
  return element;
}

function downsample(points, maxPoints) {
  if (points.length <= maxPoints) return points;
  const bucket = points.length / maxPoints;
  const out = [];
  for (let start = 0; start < points.length; start += bucket) {
    const from = Math.floor(start);
    const to = Math.min(points.length, Math.floor(start + bucket));
    if (from >= to) continue;
    let min = points[from];
    let max = points[from];
    for (let i = from + 1; i < to; i += 1) {
      if (points[i].y < min.y) min = points[i];
      if (points[i].y > max.y) max = points[i];
    }
    if (min.x <= max.x) {
      out.push(min);
      if (max !== min) out.push(max);
    } else {
      out.push(max);
      if (max !== min) out.push(min);
    }
  }
  return out;
}

function niceMax(value) {
  if (!Number.isFinite(value) || value <= 0) return 1;
  const power = 10 ** Math.floor(Math.log10(value));
  const scaled = value / power;
  const nice = scaled <= 1 ? 1 : scaled <= 2 ? 2 : scaled <= 5 ? 5 : 10;
  return nice * power;
}

export function lineChart(options = {}) {
  const {
    series = [],
    height = 240,
    xLabel = "",
    yLabel = "",
    formatX = (value) => formatNumber(value, 0),
    formatY = (value) => formatNumber(value, 1),
    threshold = null,
    maxPoints = 300,
    ariaLabel = "Gráfico",
    description = "",
  } = options;

  const figure = document.createElement("figure");
  figure.className = "chart";

  const plot = series
    .map((item) => ({
      ...item,
      points: downsample(
        (item.points || []).filter((point) => Number.isFinite(point.x) && Number.isFinite(point.y)),
        maxPoints,
      ),
    }))
    .filter((item) => item.points.length > 0);

  if (plot.length === 0) {
    const empty = document.createElement("p");
    empty.className = "empty";
    empty.textContent = "Sin datos para graficar.";
    figure.append(empty);
    return figure;
  }

  const margin = { top: 16, right: 16, bottom: 34, left: 52 };
  const innerW = CHART_WIDTH - margin.left - margin.right;
  const innerH = height - margin.top - margin.bottom;
  const allX = plot.flatMap((item) => item.points.map((point) => point.x));
  const allY = plot.flatMap((item) => item.points.map((point) => point.y));
  let xMin = Math.min(...allX);
  let xMax = Math.max(...allX);
  if (xMin === xMax) {
    xMin -= 1;
    xMax += 1;
  }
  const yMin = Math.min(0, ...allY);
  let yMax = niceMax(Math.max(...allY));
  if (threshold && Number.isFinite(threshold.value)) {
    yMax = niceMax(Math.max(yMax, threshold.value));
  }
  if (yMin === yMax) yMax = yMin + 1;

  const sx = (x) => margin.left + ((x - xMin) / (xMax - xMin)) * innerW;
  const sy = (y) => margin.top + innerH - ((y - yMin) / (yMax - yMin)) * innerH;

  const svg = svgEl("svg", {
    viewBox: `0 0 ${CHART_WIDTH} ${height}`,
    role: "img",
    "aria-label": ariaLabel,
  });
  const title = svgEl("title");
  title.textContent = ariaLabel;
  const desc = svgEl("desc");
  desc.textContent = description || ariaLabel;
  svg.append(title, desc);

  const yTicks = 4;
  for (let i = 0; i <= yTicks; i += 1) {
    const value = yMin + ((yMax - yMin) * i) / yTicks;
    const y = sy(value);
    svg.append(svgEl("line", { x1: margin.left, x2: margin.left + innerW, y1: y, y2: y, class: "grid" }));
    const label = svgEl("text", { x: margin.left - 8, y: y + 4, "text-anchor": "end", class: "axis" });
    label.textContent = formatY(value);
    svg.append(label);
  }

  const xTicks = 4;
  for (let i = 0; i <= xTicks; i += 1) {
    const value = xMin + ((xMax - xMin) * i) / xTicks;
    const label = svgEl("text", { x: sx(value), y: height - 10, "text-anchor": "middle", class: "axis" });
    label.textContent = formatX(value);
    svg.append(label);
  }

  if (xLabel) {
    const label = svgEl("text", { x: CHART_WIDTH - 8, y: height - 10, "text-anchor": "end", class: "axis" });
    label.textContent = xLabel;
    svg.append(label);
  }

  if (threshold && Number.isFinite(threshold.value) && threshold.value >= yMin && threshold.value <= yMax) {
    const y = sy(threshold.value);
    svg.append(svgEl("line", {
      x1: margin.left,
      x2: margin.left + innerW,
      y1: y,
      y2: y,
      class: "threshold",
    }));
    const label = svgEl("text", { x: margin.left + 4, y: y - 4, class: "axis" });
    label.textContent = threshold.label || formatY(threshold.value);
    svg.append(label);
  }

  plot.forEach((item, index) => {
    const color = item.color || PALETTE[index % PALETTE.length];
    const path = item.points
      .map((point, pointIndex) => `${pointIndex === 0 ? "M" : "L"}${sx(point.x).toFixed(2)},${sy(point.y).toFixed(2)}`)
      .join(" ");
    svg.append(svgEl("path", {
      d: path,
      fill: "none",
      stroke: color,
      "stroke-width": 2,
      "stroke-linejoin": "round",
      "stroke-linecap": "round",
      "vector-effect": "non-scaling-stroke",
    }));
    if (item.points.length === 1) {
      const point = item.points[0];
      svg.append(svgEl("circle", { cx: sx(point.x), cy: sy(point.y), r: 3.5, fill: color }));
    }
  });

  const legend = document.createElement("div");
  legend.className = "chart-legend";
  plot.forEach((item, index) => {
    const entry = document.createElement("span");
    const swatch = document.createElement("i");
    swatch.style.background = item.color || PALETTE[index % PALETTE.length];
    entry.append(swatch, document.createTextNode(item.name));
    legend.append(entry);
  });

  const overlay = svgEl("rect", {
    x: margin.left,
    y: margin.top,
    width: innerW,
    height: innerH,
    fill: "transparent",
    class: "overlay",
  });
  const crosshair = svgEl("line", {
    x1: 0,
    x2: 0,
    y1: margin.top,
    y2: margin.top + innerH,
    class: "crosshair",
    visibility: "hidden",
  });
  const tooltip = svgEl("g", { visibility: "hidden", class: "tooltip" });
  const tooltipRect = svgEl("rect", { rx: 4, ry: 4 });
  const tooltipText = svgEl("text", {});
  tooltip.append(tooltipRect, tooltipText);
  svg.append(overlay, crosshair, tooltip);

  const basePoints = plot[0].points;
  const showTooltip = (event) => {
    const rect = svg.getBoundingClientRect();
    const x = ((event.clientX - rect.left) / rect.width) * CHART_WIDTH;
    if (x < margin.left || x > margin.left + innerW) return;
    const value = xMin + ((x - margin.left) / innerW) * (xMax - xMin);
    let nearest = basePoints[0];
    for (const point of basePoints) {
      if (Math.abs(point.x - value) < Math.abs(nearest.x - value)) nearest = point;
    }
    const px = sx(nearest.x);
    crosshair.setAttribute("x1", px);
    crosshair.setAttribute("x2", px);
    crosshair.setAttribute("visibility", "visible");

    const lines = [`x ${formatX(nearest.x)}`];
    for (const item of plot) {
      let closest = item.points[0];
      for (const point of item.points) {
        if (Math.abs(point.x - value) < Math.abs(closest.x - value)) closest = point;
      }
      lines.push(`${item.name}: ${formatY(closest.y)}`);
    }
    tooltipText.replaceChildren();
    lines.forEach((line, index) => {
      const tspan = svgEl("tspan", { x: 8, dy: 14 });
      tspan.textContent = line;
      if (index === 0) tspan.setAttribute("dy", 14);
      tooltipText.append(tspan);
    });
    const tooltipW = Math.min(220, Math.max(...lines.map((line) => line.length)) * 7 + 16);
    const tooltipH = lines.length * 14 + 8;
    let tooltipX = px + 10;
    if (tooltipX + tooltipW > CHART_WIDTH - 8) tooltipX = px - tooltipW - 10;
    const tooltipY = margin.top + 8;
    tooltipRect.setAttribute("x", tooltipX);
    tooltipRect.setAttribute("y", tooltipY);
    tooltipRect.setAttribute("width", tooltipW);
    tooltipRect.setAttribute("height", tooltipH);
    tooltipText.setAttribute("x", tooltipX);
    tooltipText.setAttribute("y", tooltipY);
    tooltip.setAttribute("visibility", "visible");
  };
  overlay.addEventListener("pointermove", showTooltip);
  overlay.addEventListener("pointerdown", showTooltip);
  overlay.addEventListener("pointerleave", () => {
    crosshair.setAttribute("visibility", "hidden");
    tooltip.setAttribute("visibility", "hidden");
  });

  figure.append(legend, svg);
  return figure;
}
