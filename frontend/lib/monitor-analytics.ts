import type { DailyMonitorMetric } from "./api";

function parseMetricDate(value: string): Date | null {
  // metric_date is a database calendar date: keep its UTC day even when
  // represented as an RFC3339 timestamp or viewed in another timezone.
  const day = /^(\d{4}-\d{2}-\d{2})(?:T|$)/.exec(value)?.[1];
  if (!day) return null;
  const date = new Date(`${day}T00:00:00Z`);
  return Number.isNaN(date.getTime()) || date.toISOString().slice(0, 10) !== day
    ? null
    : date;
}

export function formatMetricDate(value: string, includeYear = false): string {
  const date = parseMetricDate(value);
  if (!date) return "—";
  return new Intl.DateTimeFormat("en-US", {
    month: "short",
    day: "numeric",
    ...(includeYear ? { year: "numeric" } : {}),
    timeZone: "UTC",
  }).format(date);
}

export function normalizeLatency(value: number | null | undefined): number | null {
  return typeof value === "number" && Number.isFinite(value) && value >= 0
    ? value
    : null;
}

export function formatMetricLatency(value: number | null | undefined): string {
  const latency = normalizeLatency(value);
  return latency === null ? "—" : `${Math.round(latency)} ms`;
}

export function buildMetricChartData(metrics: DailyMonitorMetric[]) {
  return metrics
    .map((metric) => ({
      date: parseMetricDate(metric.metric_date)?.toISOString().slice(0, 10) ?? null,
      meanLatencyMs: normalizeLatency(metric.mean_latency_ms),
      p95LatencyMs: normalizeLatency(metric.p95_latency_ms),
    }))
    .filter((metric): metric is typeof metric & { date: string } => metric.date !== null)
    .sort((a, b) => a.date.localeCompare(b.date));
}

export function getMetricDateRange(metrics: DailyMonitorMetric[]): string {
  const dates = buildMetricChartData(metrics).map((metric) => metric.date);
  if (dates.length === 0) return "—";
  const first = formatMetricDate(dates[0], true);
  const last = formatMetricDate(dates[dates.length - 1], true);
  return dates[0] === dates[dates.length - 1] ? first : `${first} – ${last}`;
}

export function summarizeMetrics(metrics: DailyMonitorMetric[]) {
  const totalChecks = metrics.reduce((sum, metric) => sum + metric.total_checks, 0);
  const successfulChecks = metrics.reduce((sum, metric) => sum + metric.successful_checks, 0);
  const dailyMeans = metrics
    .map((metric) => normalizeLatency(metric.mean_latency_ms))
    .filter((value): value is number => value !== null);
  const dailyP95s = metrics
    .map((metric) => normalizeLatency(metric.p95_latency_ms))
    .filter((value): value is number => value !== null);

  return {
    totalChecks,
    successfulChecks,
    averageDailyLatency: dailyMeans.length
      ? dailyMeans.reduce((sum, value) => sum + value, 0) / dailyMeans.length
      : null,
    highestDailyP95: dailyP95s.length ? Math.max(...dailyP95s) : null,
    availability: totalChecks > 0 ? (successfulChecks / totalChecks) * 100 : null,
  };
}
