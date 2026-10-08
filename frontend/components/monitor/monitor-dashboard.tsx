"use client";

import { useEffect, useState } from "react";

import AddMonitorDialog from "@/components/monitor/add-monitor-dialog";
import AccountMenu from "@/components/auth/account-menu";
import DeleteMonitorDialog from "@/components/monitor/delete-monitor-dialog";
import EditMonitorDialog from "@/components/monitor/edit-monitor-dialog";
import IncidentList from "@/components/incident/incident-list";
import MonitorAnalyticsDialog from "@/components/monitor/monitor-analytics-dialog";
import MonitorHistoryDialog from "@/components/monitor/monitor-history-dialog";
import {
  API_URL,
  getSessionUser,
  type CheckResult,
  type Incident,
  updateMonitorActive,
} from "@/lib/api";

import {
  countMonitorStatuses,
  formatCheckLatency,
  formatCheckStatus,
  getMonitorStatus,
  type LiveMonitor,
  type MonitorStatus,
} from "@/lib/monitor-status";

const statusStyles: Record<MonitorStatus, { dot: string; text: string }> = {
  UP: { dot: "bg-green-500", text: "text-green-400" },
  DOWN: { dot: "bg-red-500", text: "text-red-400" },
  PAUSED: { dot: "bg-zinc-500", text: "text-zinc-400" },
  "AWAITING CHECK": { dot: "bg-amber-500", text: "text-amber-400" },
};

type Props = {
  email: string;
  initialMonitors: LiveMonitor[];
  initialIncidents: Incident[];
  incidentError: string | null;
};

type ActionError = {
  monitorID: string;
  message: string;
};

export default function MonitorDashboard({
  email,
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
    let disposed = false;
    let checkingSession = false;
    const sessionEnded = () => {
      if (disposed) return;
      events.close();
      window.location.replace("/login");
    };
    const handleConnectionError = async () => {
      // EventSource does not expose HTTP status. Check only on connection
      // errors, keeping automatic SSE reconnects for transient outages.
      if (disposed || checkingSession) return;
      checkingSession = true;
      try { await getSessionUser(); } catch { /* 401 dispatches session-ended. */ }
      finally { checkingSession = false; }
    };

    const handleCheck = (event: Event) => {
      try {
        const check = JSON.parse((event as MessageEvent<string>).data) as CheckResult;
        setMonitors((current) =>
          current.map((monitor) =>
            monitor.id === check.monitor_id
              ? { ...monitor, latestCheck: check, latestCheckError: undefined }
              : monitor,
          ),
        );
      } catch {
        // Ignore malformed live events.
      }
    };

    events.addEventListener("check", handleCheck);
    events.addEventListener("session-ended", sessionEnded);
    events.addEventListener("error", handleConnectionError);
    window.addEventListener("statuspulse:session-ended", sessionEnded);

    return () => {
      disposed = true;
      events.removeEventListener("check", handleCheck);
      events.removeEventListener("session-ended", sessionEnded);
      events.removeEventListener("error", handleConnectionError);
      window.removeEventListener("statuspulse:session-ended", sessionEnded);
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

  const statusCounts = countMonitorStatuses(monitors);
  const historyMonitor =
    monitors.find((monitor) => monitor.id === historyMonitorID) ?? null;
  const analyticsMonitor =
    monitors.find((monitor) => monitor.id === analyticsMonitorID) ?? null;

  return (
    <>
      <header className="flex items-center justify-between border-b border-zinc-800/80 pb-8">
        <div className="flex items-center gap-4">
          <div className="flex h-11 w-11 items-center justify-center rounded-xl border border-indigo-400/20 bg-indigo-500/10 text-indigo-300">
            <svg aria-hidden="true" width="26" height="26" viewBox="0 0 24 24" fill="none">
              <path d="M2 12h4l3-7 5 14 3-7h5" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
          </div>
          <div>
            <h1 className="text-2xl font-semibold tracking-tight">StatusPulse</h1>
            <p className="mt-1 text-sm text-zinc-400">
              Monitor your services in real time.
            </p>
          </div>
        </div>

        <div className="flex flex-wrap items-center justify-end gap-3">
          <AddMonitorDialog onCreated={(monitor) => setMonitors((current) => [monitor, ...current])} />
          <AccountMenu email={email} />
        </div>
      </header>

      <section className="mt-10">
        <h2 className="text-xs font-semibold uppercase tracking-widest text-zinc-500">Overview</h2>
        <div className="mt-4 grid gap-4 sm:grid-cols-3 lg:grid-cols-5">
          <StatCard label="Monitors" value={monitors.length} />
          <StatCard label="Up" value={statusCounts.UP} dot="bg-green-500" />
          <StatCard label="Down" value={statusCounts.DOWN} dot="bg-red-500" />
          <StatCard label="Paused" value={statusCounts.PAUSED} dot="bg-zinc-500" />
          <StatCard label="Awaiting check" value={statusCounts["AWAITING CHECK"]} dot="bg-amber-500" />
        </div>
      </section>

      <section className="mt-10">
        <div className="flex items-center gap-3">
          <h2 className="text-lg font-semibold tracking-tight">Monitors</h2>
          <span className="rounded-md border border-zinc-800 bg-zinc-900 px-2 py-0.5 text-xs tabular-nums text-zinc-400">{monitors.length}</span>
        </div>
        <div className="mt-5 space-y-4">
          {monitors.map((monitor) => {
            const status = getMonitorStatus(monitor);
            const style = statusStyles[status];
            const check = monitor.latestCheck;
            const isUpdating = updatingMonitorID === monitor.id;
            const actionError =
              monitorActionError?.monitorID === monitor.id
                ? monitorActionError.message
                : null;

            return (
              <div
                key={monitor.id}
                className="ui-panel overflow-hidden p-6 transition-colors hover:border-zinc-700"
              >
                <div className="flex items-center justify-between gap-4">
                  <div className="min-w-0">
                    <div className="flex items-center gap-3">
                      <h3 className="text-base font-semibold tracking-tight">{monitor.name}</h3>
                      <span className={`inline-flex items-center gap-1.5 rounded-full border border-current/15 bg-current/5 px-2.5 py-1 text-[11px] font-semibold tracking-wide ${style.text}`}>
                        <span className={`h-1.5 w-1.5 rounded-full ${style.dot}`} />
                        {status}
                      </span>
                    </div>
                    <p className="mt-2 truncate text-sm text-zinc-500" title={monitor.url}>{monitor.url}</p>
                  </div>

                  <div className="flex flex-wrap items-center justify-end gap-2">


                    <button
                      className="ui-button"
                      onClick={() => setHistoryMonitorID(monitor.id)}
                      type="button"
                    >
                      History
                    </button>

                    <button
                      className="ui-button"
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
                      className="ui-button"
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

                <div className="mt-6 grid grid-cols-[1fr_1fr_2fr] gap-6 border-t border-zinc-800/70 pt-5">
                  <MonitorReading label="HTTP status" value={formatCheckStatus(check)} />
                  <MonitorReading label="Response time" value={formatCheckLatency(check)} />
                  <MonitorReading label="Last checked" value={formatCheckedAt(check?.checked_at)} />
                </div>
                <div className="mt-5 flex flex-wrap gap-x-5 gap-y-1 text-xs text-zinc-500">
                  <span>Expected HTTP <span className="text-zinc-400">{monitor.expected_status_code}</span></span>
                  <span>Every <span className="text-zinc-400">{monitor.interval_seconds}s</span></span>
                  <span>Timeout <span className="text-zinc-400">{monitor.timeout_seconds}s</span></span>
                </div>

                {monitor.latestCheckError && (
                  <p className="mt-4 text-sm text-amber-400">
                    {monitor.latestCheckError}
                  </p>
                )}

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

function StatCard({ label, value, dot = "bg-indigo-400" }: { label: string; value: number; dot?: string }) {
  return (
    <div className="ui-panel px-5 py-5">
      <p className="flex items-center gap-2 text-xs font-medium text-zinc-400">
        <span className={`h-1.5 w-1.5 rounded-full ${dot}`} />{label}
      </p>
      <p className="mt-3 text-3xl font-semibold tracking-tight tabular-nums">{value}</p>
    </div>
  );
}

function MonitorReading({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <p className="text-xs text-zinc-500">{label}</p>
      <p className="mt-1.5 text-sm font-medium tabular-nums text-zinc-200">{value}</p>
    </div>
  );
}

function formatCheckedAt(value?: string) {
  if (!value) return "—";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "—" : date.toLocaleString();
}
