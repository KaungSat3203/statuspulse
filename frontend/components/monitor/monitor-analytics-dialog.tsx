"use client";

import { useEffect, useMemo, useState } from "react";
import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import {
  getMonitorMetrics,
  type DailyMonitorMetric,
  type Monitor,
} from "@/lib/api";

type MonitorAnalyticsDialogProps = {
  monitor: Monitor;
  onClose: () => void;
};

function safeNumber(value: number | null | undefined): number {
  return typeof value === "number" && Number.isFinite(value) ? value : 0;
}

function formatMetricDate(value: string): string {
  const date = new Date(`${value}T00:00:00Z`);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  return new Intl.DateTimeFormat("en-US", {
    month: "short",
    day: "numeric",
  }).format(date);
}

function formatLatency(value: number | null | undefined): string {
  const numeric = safeNumber(value);
  return numeric > 0 ? `${Math.round(numeric)} ms` : "N/A";
}

export default function MonitorAnalyticsDialog({
  monitor,
  onClose,
}: MonitorAnalyticsDialogProps) {
  const [metrics, setMetrics] = useState<DailyMonitorMetric[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let isMounted = true;

    const loadMetrics = async () => {
      setMetrics(null);
      setError(null);

      try {
        const results = await getMonitorMetrics(monitor.id, 30);
        if (!isMounted) return;
        setMetrics(results);
      } catch (fetchError) {
        if (!isMounted) return;
        setError(
          fetchError instanceof Error
            ? fetchError.message
            : "Unable to load monitor analytics",
        );
      }
    };

    void loadMetrics();

    return () => {
      isMounted = false;
    };
  }, [monitor.id]);

  const chartData = useMemo(
    () =>
      [...(metrics ?? [])]
        .reverse()
        .map((metric) => ({
          ...metric,
          label: formatMetricDate(metric.metric_date),
          meanLatencyMs: safeNumber(metric.mean_latency_ms),
          p95LatencyMs: safeNumber(metric.p95_latency_ms),
        })),
    [metrics],
  );

  const summary = useMemo(() => {
    if (!metrics || metrics.length === 0) {
      return {
        totalChecks: 0,
        successfulChecks: 0,
        averageLatency: 0,
        p95Latency: 0,
        availability: 0,
      };
    }

    const totalChecks = metrics.reduce(
      (sum, metric) => sum + metric.total_checks,
      0,
    );
    const successfulChecks = metrics.reduce(
      (sum, metric) => sum + metric.successful_checks,
      0,
    );
    const validMeanLatencies = metrics
      .map((metric) => safeNumber(metric.mean_latency_ms))
      .filter((value) => value > 0);
    const validP95Latencies = metrics
      .map((metric) => safeNumber(metric.p95_latency_ms))
      .filter((value) => value > 0);

    const averageLatency =
      validMeanLatencies.length === 0
        ? 0
        : validMeanLatencies.reduce((sum, value) => sum + value, 0) /
          validMeanLatencies.length;
    const p95Latency =
      validP95Latencies.length === 0
        ? 0
        : Math.max(...validP95Latencies);

    return {
      totalChecks,
      successfulChecks,
      averageLatency,
      p95Latency,
      availability:
        totalChecks === 0 ? 0 : (successfulChecks / totalChecks) * 100,
    };
  }, [metrics]);

  return (
    <div
      aria-modal="true"
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 px-4"
      role="dialog"
    >
      <div className="w-full max-w-4xl rounded-xl border border-zinc-800 bg-zinc-900 p-6 shadow-2xl">
        <div className="flex items-start justify-between gap-4">
          <div>
            <h2 className="text-lg font-medium">Analytics</h2>
            <p className="mt-1 text-sm text-zinc-400">{monitor.name}</p>
          </div>
          <button
            aria-label="Close analytics dialog"
            className="text-zinc-400 hover:text-white"
            onClick={onClose}
            type="button"
          >
            ×
          </button>
        </div>

        <div className="mt-6">
          {error ? (
            <p className="rounded-lg border border-red-900 bg-red-950/50 px-3 py-2 text-sm text-red-300">
              {error}
            </p>
          ) : metrics === null ? (
            <p className="text-sm text-zinc-400">Loading daily metrics…</p>
          ) : metrics.length === 0 ? (
            <p className="rounded-lg border border-zinc-800 bg-zinc-950 px-3 py-4 text-sm text-zinc-400">
              No daily metrics are available for this monitor yet.
            </p>
          ) : (
            <>
              <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
                <MetricStat label="Total checks" value={summary.totalChecks} />
                <MetricStat
                  label="Successful checks"
                  value={summary.successfulChecks}
                />
                <MetricStat
                  label="Average latency"
                  value={formatLatency(summary.averageLatency)}
                />
                <MetricStat
                  label="P95 latency"
                  value={formatLatency(summary.p95Latency)}
                />
              </div>

              <div className="mt-4 rounded-lg border border-zinc-800 bg-zinc-950/80 p-3 text-sm text-zinc-300">
                Overall availability: {summary.availability.toFixed(1)}%
              </div>

              <div className="mt-6 h-72 w-full">
                <ResponsiveContainer width="100%" height="100%">
                  <LineChart
                    data={chartData}
                    margin={{ top: 12, right: 18, left: 4, bottom: 8 }}
                  >
                    <CartesianGrid stroke="#27272a" strokeDasharray="3 3" />
                    <XAxis
                      dataKey="label"
                      interval={0}
                      minTickGap={18}
                      tick={{ fill: "#a1a1aa", fontSize: 12 }}
                    />
                    <YAxis
                      tick={{ fill: "#a1a1aa", fontSize: 12 }}
                      tickFormatter={(value) => `${value} ms`}
                    />
                    <Tooltip
                      formatter={(value: number | string, name: string) => {
                        const numericValue =
                          typeof value === "number" ? value : Number(value);
                        const label =
                          name === "meanLatencyMs" ? "Mean latency" : "P95 latency";
                        return [`${Math.round(numericValue)} ms`, label];
                      }}
                      labelFormatter={(label) => `Date: ${label}`}
                      contentStyle={{
                        backgroundColor: "#09090b",
                        borderColor: "#3f3f46",
                        borderRadius: 12,
                        color: "#f4f4f5",
                      }}
                    />
                    <Line
                      connectNulls
                      dataKey="meanLatencyMs"
                      dot={{ r: 2 }}
                      name="meanLatencyMs"
                      stroke="#f4f4f5"
                      strokeWidth={2}
                      type="monotone"
                    />
                    <Line
                      connectNulls
                      dataKey="p95LatencyMs"
                      dot={{ r: 2 }}
                      name="p95LatencyMs"
                      stroke="#f59e0b"
                      strokeWidth={2}
                      type="monotone"
                    />
                  </LineChart>
                </ResponsiveContainer>
              </div>
            </>
          )}
        </div>

        <div className="mt-6 flex justify-end">
          <button
            className="rounded-lg px-4 py-2 text-sm text-zinc-300 hover:text-white"
            onClick={onClose}
            type="button"
          >
            Close
          </button>
        </div>
      </div>
    </div>
  );
}

function MetricStat({
  label,
  value,
}: {
  label: string;
  value: string | number;
}) {
  return (
    <div className="rounded-xl border border-zinc-800 bg-zinc-950 p-4">
      <p className="text-sm text-zinc-400">{label}</p>
      <p className="mt-2 text-xl font-semibold text-white">{value}</p>
    </div>
  );
}
