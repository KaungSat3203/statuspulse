"use client";

import { useEffect, useState } from "react";

import AddMonitorDialog from "@/components/monitor/add-monitor-dialog";
import DeleteMonitorDialog from "@/components/monitor/delete-monitor-dialog";
import EditMonitorDialog from "@/components/monitor/edit-monitor-dialog";
import IncidentList from "@/components/incident/incident-list";
import MonitorAnalyticsDialog from "@/components/monitor/monitor-analytics-dialog";
import MonitorHistoryDialog from "@/components/monitor/monitor-history-dialog";
import {
  API_URL,
  type CheckResult,
  type Incident,
  type Monitor,
  updateMonitorActive,
} from "@/lib/api";

type LiveMonitor = Monitor & { latestCheck?: CheckResult };

type Props = {
  initialMonitors: Monitor[];
  initialIncidents: Incident[];
  incidentError: string | null;
};

type ActionError = {
  monitorID: string;
  message: string;
};

export default function MonitorDashboard({
  initialMonitors,
  initialIncidents,
  incidentError,
}: Props) {
  const [monitors, setMonitors] = useState<LiveMonitor[]>(initialMonitors);
  const [updatingMonitorID, setUpdatingMonitorID] = useState<string | null>(null);
  const [historyMonitorID, setHistoryMonitorID] = useState<string | null>(null);
  const [analyticsMonitorID, setAnalyticsMonitorID] = useState<string | null>(null);
  const [monitorActionError, setMonitorActionError] = useState<ActionError | null>(null);

  useEffect(() => {
    const events = new EventSource(API_URL + "/events");

    const handleCheck = (event: Event) => {
      try {
        const check = JSON.parse((event as MessageEvent<string>).data) as CheckResult;
        setMonitors((current) =>
          current.map((monitor) =>
            monitor.id === check.monitor_id
              ? { ...monitor, latestCheck: check }
              : monitor,
          ),
        );
      } catch {
        // Ignore malformed live events.
      }
    };

    events.addEventListener("check", handleCheck);
    events.addEventListener("error", () => {});

    return () => {
      events.removeEventListener("check", handleCheck);
      events.close();
    };
  }, []);

  const updateActive = async (monitor: LiveMonitor) => {
    setUpdatingMonitorID(monitor.id);
    setMonitorActionError(null);

    try {
      const updated = await updateMonitorActive(monitor.id, !monitor.active);
      setMonitors((current) =>
        current.map((existing) =>
          existing.id === updated.id ? { ...existing, ...updated } : existing,
        ),
      );
    } catch (error) {
      setMonitorActionError({
        monitorID: monitor.id,
        message:
          error instanceof Error ? error.message : "Unable to update monitor status",
      });
    } finally {
      setUpdatingMonitorID(null);
    }
  };

  const onlineCount = monitors.filter(isMonitorUp).length;
  const historyMonitor =
    monitors.find((monitor) => monitor.id === historyMonitorID) ?? null;
  const analyticsMonitor =
    monitors.find((monitor) => monitor.id === analyticsMonitorID) ?? null;

  return (
    <>
      <header className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold">StatusPulse</h1>
          <p className="mt-1 text-sm text-zinc-400">
            Monitor your services in real time.
          </p>
        </div>

        <AddMonitorDialog
          onCreated={(monitor) => setMonitors((current) => [monitor, ...current])}
        />
      </header>

      <section className="mt-10">
        <h2 className="text-lg font-medium">Overview</h2>
        <div className="mt-4 grid gap-4 sm:grid-cols-3">
          <StatCard label="Monitors" value={monitors.length} />
          <StatCard label="Online" value={onlineCount} />
          <StatCard label="Down" value={monitors.length - onlineCount} />
        </div>
      </section>

      <section className="mt-10">
        <h2 className="text-lg font-medium">Monitors</h2>
        <div className="mt-4 space-y-3">
          {monitors.map((monitor) => {
            const isUp = isMonitorUp(monitor);
            const check = monitor.latestCheck;
            const isUpdating = updatingMonitorID === monitor.id;
            const actionError =
              monitorActionError?.monitorID === monitor.id
                ? monitorActionError.message
                : null;

            return (
              <div
                key={monitor.id}
                className="rounded-xl border border-zinc-800 bg-zinc-900 p-5"
              >
                <div className="flex items-center justify-between gap-4">
                  <div>
                    <h3 className="font-medium">{monitor.name}</h3>
                    <p className="mt-1 text-sm text-zinc-400">{monitor.url}</p>
                  </div>

                  <div className="flex flex-wrap items-center justify-end gap-2">
                    <div className="mr-2 flex items-center gap-2">
                      <span
                        className={
                          isUp
                            ? "h-2.5 w-2.5 rounded-full bg-green-500"
                            : "h-2.5 w-2.5 rounded-full bg-red-500"
                        }
                      />
                      <span
                        className={
                          isUp ? "text-sm text-green-400" : "text-sm text-red-400"
                        }
                      >
                        {isUp ? "UP" : "DOWN"}
                      </span>
                    </div>

                    <button
                      className="rounded-lg border border-zinc-700 px-3 py-1.5 text-sm font-medium text-zinc-200 hover:border-zinc-500 hover:text-white"
                      onClick={() => setHistoryMonitorID(monitor.id)}
                      type="button"
                    >
                      History
                    </button>

                    <button
                      className="rounded-lg border border-zinc-700 px-3 py-1.5 text-sm font-medium text-zinc-200 hover:border-zinc-500 hover:text-white"
                      onClick={() => setAnalyticsMonitorID(monitor.id)}
                      type="button"
                    >
                      Analytics
                    </button>

                    <EditMonitorDialog
                      monitor={monitor}
                      onUpdated={(updated) =>
                        setMonitors((current) =>
                          current.map((existing) =>
                            existing.id === updated.id
                              ? { ...existing, ...updated }
                              : existing,
                          ),
                        )
                      }
                    />

                    <DeleteMonitorDialog
                      monitor={monitor}
                      onDeleted={(id) => {
                        setMonitors((current) =>
                          current.filter((item) => item.id !== id),
                        );
                        setHistoryMonitorID((current) =>
                          current === id ? null : current,
                        );
                        setAnalyticsMonitorID((current) =>
                          current === id ? null : current,
                        );
                        setMonitorActionError((current) =>
                          current?.monitorID === id ? null : current,
                        );
                        setUpdatingMonitorID((current) =>
                          current === id ? null : current,
                        );
                      }}
                    />

                    <button
                      className="rounded-lg border border-zinc-700 px-3 py-1.5 text-sm font-medium text-zinc-200 hover:border-zinc-500 hover:text-white disabled:cursor-not-allowed disabled:opacity-60"
                      disabled={isUpdating}
                      onClick={() => updateActive(monitor)}
                      type="button"
                    >
                      {isUpdating
                        ? monitor.active
                          ? "Pausing..."
                          : "Resuming..."
                        : monitor.active
                          ? "Pause"
                          : "Resume"}
                    </button>
                  </div>
                </div>

                <div className="mt-4 flex flex-wrap gap-x-6 gap-y-2 text-sm text-zinc-400">
                  <span>Expected: {monitor.expected_status_code}</span>
                  <span>Interval: {monitor.interval_seconds}s</span>
                  <span>Timeout: {monitor.timeout_seconds}s</span>
                  <span>HTTP {check?.status_code ?? "-"}</span>
                  <span>{check ? `${check.latency_ms} ms` : "Latency -"}</span>
                  <span>Last checked: {formatCheckedAt(check?.checked_at)}</span>
                </div>

                {actionError && (
                  <p className="mt-4 rounded-lg border border-red-900 bg-red-950/50 px-3 py-2 text-sm text-red-300">
                    {actionError}
                  </p>
                )}
              </div>
            );
          })}
        </div>
      </section>

      <IncidentList
        error={incidentError}
        incidents={initialIncidents}
        monitors={monitors}
      />

      {historyMonitor && (
        <MonitorHistoryDialog
          monitor={historyMonitor}
          onClose={() => setHistoryMonitorID(null)}
        />
      )}

      {analyticsMonitor && (
        <MonitorAnalyticsDialog
          monitor={analyticsMonitor}
          onClose={() => setAnalyticsMonitorID(null)}
        />
      )}
    </>
  );
}

function StatCard({ label, value }: { label: string; value: number }) {
  return (
    <div className="rounded-xl border border-zinc-800 bg-zinc-900 p-5">
      <p className="text-sm text-zinc-400">{label}</p>
      <p className="mt-2 text-3xl font-semibold">{value}</p>
    </div>
  );
}

function isMonitorUp(monitor: LiveMonitor) {
  return monitor.latestCheck?.reachable ?? monitor.consecutive_failures === 0;
}

function formatCheckedAt(value?: string) {
  if (!value) return "-";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "-" : date.toLocaleString();
}
