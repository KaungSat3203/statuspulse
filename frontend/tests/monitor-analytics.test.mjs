import assert from "node:assert/strict";
import test from "node:test";
import { readFile } from "node:fs/promises";
import ts from "typescript";

// Reuse Next.js's installed compiler and Node's built-in test runner.
const source = await readFile(new URL("../lib/monitor-analytics.ts", import.meta.url), "utf8");
const { outputText } = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2020 },
});
const {
  buildMetricChartData,
  formatMetricDate,
  formatMetricLatency,
  getMetricDateRange,
  normalizeLatency,
  summarizeMetrics,
} = await import(`data:text/javascript;base64,${Buffer.from(outputText).toString("base64")}`);

const metric = {
  monitor_id: "11111111-1111-1111-1111-111111111111",
  metric_date: "2026-10-08T00:00:00Z",
  total_checks: 10,
  successful_checks: 9,
  mean_latency_ms: 100,
  p95_latency_ms: 200,
};

test("date-only and backend timestamp dates retain the UTC calendar day", () => {
  const originalTimezone = process.env.TZ;
  try {
    process.env.TZ = "America/Los_Angeles";
    assert.equal(formatMetricDate("2026-10-08"), "Oct 8");
    assert.equal(formatMetricDate(metric.metric_date), "Oct 8");
    assert.equal(formatMetricDate(metric.metric_date, true), "Oct 8, 2026");
    assert.equal(formatMetricDate("2026-02-30"), "—");
    assert.equal(formatMetricDate("invalid"), "—");
  } finally {
    if (originalTimezone === undefined) delete process.env.TZ;
    else process.env.TZ = originalTimezone;
  }
});

test("chart sorts chronologically without mutating metrics and retains measurement gaps", () => {
  const rows = [
    metric,
    { ...metric, metric_date: "2026-10-07T00:00:00Z", mean_latency_ms: null, p95_latency_ms: undefined },
    { ...metric, metric_date: "2026-10-06", mean_latency_ms: 0 },
  ];
  const original = structuredClone(rows);
  assert.deepEqual(buildMetricChartData(rows), [
    { date: "2026-10-06", meanLatencyMs: 0, p95LatencyMs: 200 },
    { date: "2026-10-07", meanLatencyMs: null, p95LatencyMs: null },
    { date: "2026-10-08", meanLatencyMs: 100, p95LatencyMs: 200 },
  ]);
  assert.deepEqual(rows, original);
});

test("date range uses the actual oldest and newest recorded days", () => {
  assert.equal(getMetricDateRange([
    metric,
    { ...metric, metric_date: "2026-09-30T00:00:00Z" },
  ]), "Sep 30, 2026 – Oct 8, 2026");
  assert.equal(getMetricDateRange([metric]), "Oct 8, 2026");
  assert.equal(getMetricDateRange([]), "—");
});

test("summary calculates equally weighted daily means, highest daily P95, and check-weighted availability", () => {
  assert.deepEqual(summarizeMetrics([
    metric,
    { ...metric, total_checks: 90, successful_checks: 81, mean_latency_ms: 300, p95_latency_ms: 600 },
  ]), {
    totalChecks: 100,
    successfulChecks: 90,
    averageDailyLatency: 200,
    highestDailyP95: 600,
    availability: 90,
  });
});

test("missing latency is unavailable, never an artificial zero", () => {
  for (const value of [null, undefined, NaN, Infinity, -1]) {
    assert.equal(normalizeLatency(value), null);
    assert.equal(formatMetricLatency(value), "—");
  }
  const missing = { ...metric, mean_latency_ms: undefined, p95_latency_ms: null };
  const summary = summarizeMetrics([missing]);
  assert.equal(summary.averageDailyLatency, null);
  assert.equal(summary.highestDailyP95, null);
  assert.equal(formatMetricLatency(0), "0 ms");
  assert.equal(summarizeMetrics([
    { ...metric, mean_latency_ms: 0, p95_latency_ms: 0 },
    missing,
  ]).averageDailyLatency, 0);
});

test("no checks differs from measured zero-percent availability", () => {
  const empty = {
    ...metric, total_checks: 0, successful_checks: 0,
    mean_latency_ms: null, p95_latency_ms: null,
  };
  assert.equal(summarizeMetrics([empty]).availability, null);
  assert.equal(summarizeMetrics([]).availability, null);
  assert.equal(summarizeMetrics([{ ...metric, successful_checks: 0 }]).availability, 0);
});
