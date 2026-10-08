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

import {
  buildMetricChartData,
  formatMetricDate,
  formatMetricLatency,
  getMetricDateRange,
  summarizeMetrics,
} from "@/lib/monitor-analytics";

import {
  DialogEmptyState,
  DialogErrorState,
  DialogLoadingState,
} from "@/components/ui/dialog-data-state";

type MonitorAnalyticsDialogProps = {
  monitor: Monitor;
  onClose: () => void;
};

export default function MonitorAnalyticsDialog({
  monitor,
  onClose,
}: MonitorAnalyticsDialogProps) {
  const [metrics, setMetrics] = useState<DailyMonitorMetric[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  const [retryCount, setRetryCount] = useState(0);

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
  }, [monitor.id, retryCount]);

  const retry = () => {
    setMetrics(null);
    setError(null);
    setRetryCount((count) => count + 1);
  };

  const chartData = useMemo(() => buildMetricChartData(metrics ?? []), [metrics]);
  const summary = useMemo(() => summarizeMetrics(metrics ?? []), [metrics]);
  const dateRange = useMemo(() => getMetricDateRange(metrics ?? []), [metrics]);
  const hasLatency = chartData.some(
    (metric) => metric.meanLatencyMs !== null || metric.p95LatencyMs !== null,
  );

  return (
    <div
      aria-modal="true"
      className="ui-dialog-backdrop"
      role="dialog"
    >
      <div className="ui-dialog max-w-4xl">
        <div className="flex items-start justify-between gap-4">
          <div>
            <h2 className="text-lg font-semibold tracking-tight">Analytics</h2>
            <p className="mt-1 text-sm text-zinc-400">{monitor.name}</p>
          </div>
          <button
            aria-label="Close analytics dialog"
            className="ui-close"
            onClick={onClose}
            type="button"
          >
            ×
          </button>
        </div>

        <div className="mt-6">
          {error ? (
            <DialogErrorState
              title="Unable to load analytics"
              message={error}
              onRetry={retry}
            />
          ) : metrics === null ? (
            <DialogLoadingState label="Loading daily metrics…" variant="analytics" />
          ) : metrics.length === 0 ? (
            <DialogEmptyState
              title="No daily metrics yet"
              description="Daily metrics summarize completed UTC days. Today's checks are included after that day's aggregation runs."
            />
          ) : (
            <>
              <p className="mb-4 text-xs text-zinc-400">
                {dateRange} · {metrics.length} aggregated{" "}
                {metrics.length === 1 ? "day" : "days"} · UTC
              </p>
              <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
                <MetricStat label="Total checks" value={summary.totalChecks} />
                <MetricStat
                  label="Successful checks"
                  value={summary.successfulChecks}
                />
                <MetricStat
                  label="Average daily latency"
                  value={formatMetricLatency(summary.averageDailyLatency)}
                  description="Average of daily means; each day weighted equally."
                />
                <MetricStat
                  label="Highest daily P95"
                  value={formatMetricLatency(summary.highestDailyP95)}
                  description="Highest P95 among the recorded days."
                />
              </div>

              <div className="mt-4 rounded-lg border border-zinc-800 bg-zinc-950/80 p-3 text-sm text-zinc-300">
                Availability:{" "}
                {summary.availability === null
                  ? "No checks"
                  : `${summary.availability.toFixed(1)}%`}
                <span className="ml-2 text-xs text-zinc-500">
                  {summary.availability === null
                    ? "No checks recorded in these daily metrics."
                    : "Successful checks / total checks in this period"}
                </span>
              </div>

              {hasLatency ? (
                <>
                  <div className="mt-6 flex items-center justify-between gap-4">
                    <h3 className="text-sm font-medium text-zinc-200">
                      Daily latency
                    </h3>
                    <div className="flex gap-4 text-xs text-zinc-400">
                      <span className="flex items-center gap-2">
                        <span className="h-0.5 w-4 bg-zinc-100" />
                        Daily mean
                      </span>
                      <span className="flex items-center gap-2">
                        <span className="h-0.5 w-4 bg-amber-500" />
                        Daily P95
                      </span>
                    </div>
                  </div>
                  <p className="mt-2 text-xs text-zinc-500">
                    Gaps indicate days with no recorded latency measurement.
                  </p>
                  <div className="mt-3 h-64 w-full">
                    <ResponsiveContainer width="100%" height="100%">
                      <LineChart
                        data={chartData}
                        margin={{ top: 12, right: 18, left: 4, bottom: 8 }}
                      >
                        <CartesianGrid stroke="#27272a" strokeDasharray="3 3" />
                        <XAxis
                          dataKey="date"
                          tickFormatter={(value) => formatMetricDate(String(value))}
                          interval="preserveStartEnd"
                          minTickGap={28}
                          tickMargin={10}
                          tick={{ fill: "#a1a1aa", fontSize: 12 }}
                        />
                        <YAxis
                          tick={{ fill: "#a1a1aa", fontSize: 12 }}
                          tickFormatter={(value) => `${value} ms`}
                        />
                        <Tooltip
                          formatter={(value: number | string, name: string) => {
                            return [
                              formatMetricLatency(
                                typeof value === "number" ? value : undefined,
                              ),
                              name,
                            ];
                          }}
                          labelFormatter={(label) =>
                            `${formatMetricDate(String(label), true)} · UTC`
                          }
                          contentStyle={{
                            backgroundColor: "#09090b",
                            borderColor: "#3f3f46",
                            borderRadius: 12,
                            color: "#f4f4f5",
                          }}
                        />
                        <Line
                          connectNulls={false}
                          dataKey="meanLatencyMs"
                          dot={{ r: 2 }}
                          name="Daily mean"
                          stroke="#f4f4f5"
                          strokeWidth={2}
                          type="monotone"
                        />
                        <Line
                          connectNulls={false}
                          dataKey="p95LatencyMs"
                          dot={{ r: 2 }}
                          name="Daily P95"
                          stroke="#f59e0b"
                          strokeWidth={2}
                          type="monotone"
                        />
                      </LineChart>
                    </ResponsiveContainer>
                  </div>
                </>
              ) : (
                <p className="mt-6 rounded-lg border border-zinc-800 bg-zinc-950/60 px-4 py-5 text-sm text-zinc-400">
                  No latency measurements were recorded for these days.
                </p>
              )}
            </>
          )}
        </div>

        <div className="mt-6 flex justify-end">
          <button
            className="ui-button-quiet"
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
  description,
}: {
  label: string;
  value: string | number;
  description?: string;
}) {
  return (
    <div className="rounded-xl border border-zinc-800/80 bg-zinc-950/60 p-4">
      <p className="text-sm text-zinc-400">{label}</p>
      <p className="mt-2 text-2xl font-semibold tracking-tight tabular-nums text-white">
        {value}
      </p>
      {description && (
        <p className="mt-2 text-xs leading-relaxed text-zinc-500">{description}</p>
      )}
    </div>
  );
}
