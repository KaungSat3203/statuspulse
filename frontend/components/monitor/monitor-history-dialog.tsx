"use client";

import { useEffect, useState } from "react";
import { getMonitorChecks, type CheckResult, type Monitor } from "@/lib/api";
import {
  DialogEmptyState,
  DialogErrorState,
  DialogLoadingState,
} from "@/components/ui/dialog-data-state";

type Props = { monitor: Monitor; onClose: () => void };

export default function MonitorHistoryDialog({ monitor, onClose }: Props) {
  const [checks, setChecks] = useState<CheckResult[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [retryCount, setRetryCount] = useState(0);

  useEffect(() => {
    let cancelled = false;
    const loadChecks = async () => {
      setChecks(null);
      setError(null);
      try {
        const results = await getMonitorChecks(monitor.id, 50);
        if (!cancelled) setChecks(results);
      } catch (fetchError) {
        if (!cancelled) {
          setError(
            fetchError instanceof Error
              ? fetchError.message
              : "Unable to load recent checks",
          );
        }
      }
    };
    void loadChecks();
    return () => {
      cancelled = true;
    };
  }, [monitor.id, retryCount]);

  const retry = () => {
    setChecks(null);
    setError(null);
    setRetryCount((count) => count + 1);
  };

  return (
    <div aria-modal="true" className="ui-dialog-backdrop" role="dialog">
      <div className="ui-dialog max-w-2xl">
        <div className="flex items-start justify-between gap-4">
          <div>
            <h2 className="text-lg font-semibold tracking-tight">Recent Checks</h2>
            <p className="mt-1 text-sm text-zinc-400">{monitor.name}</p>
          </div>
          <button
            aria-label="Close monitor history"
            className="ui-close"
            onClick={onClose}
            type="button"
          >
            &times;
          </button>
        </div>
        <div className="mt-6">
          {error ? (
            <DialogErrorState
              title="Unable to load check history"
              message={error}
              onRetry={retry}
            />
          ) : checks === null ? (
            <DialogLoadingState label="Loading recent checks…" variant="history" />
          ) : checks.length === 0 ? (
            <DialogEmptyState
              title="No check history yet"
              description={monitor.active
                ? "History will be available after this monitor's first completed check."
                : "This monitor has no recorded checks. Resume it to start checking."}
            />
          ) : (
            <div className="max-h-[60vh] overflow-y-auto rounded-lg border border-zinc-800">
              <ul className="divide-y divide-zinc-800">
                {checks.map((check, index) => (
                  <li
                    className="px-4 py-3 text-sm"
                    key={`${check.checked_at}-${index}`}
                  >
                    <div className="flex flex-wrap items-center gap-x-4 gap-y-1">
                      <span className="w-40 text-zinc-400">
                        {formatCheckedAt(check.checked_at)}
                      </span>
                      <span className={check.reachable
                        ? "font-medium text-green-400"
                        : "font-medium text-red-400"}
                      >
                        {check.reachable ? "UP" : "DOWN"}
                      </span>
                      <span className="text-zinc-300">{formatStatus(check)}</span>
                      <span className="text-zinc-300">{formatLatency(check)}</span>
                    </div>
                    {check.error && (
                      <p className="mt-1 text-sm text-red-300">{check.error}</p>
                    )}
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>
        <div className="mt-6 flex justify-end">
          <button className="ui-button-quiet" onClick={onClose} type="button">
            Close
          </button>
        </div>
      </div>
    </div>
  );
}

function formatCheckedAt(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "-" : date.toLocaleString();
}
function formatStatus(check: CheckResult) {
  return check.status_code > 0 ? `HTTP ${check.status_code}` : "HTTP -";
}
function formatLatency(check: CheckResult) {
  return check.latency_ms > 0 ? `${check.latency_ms} ms` : "Latency -";
}
