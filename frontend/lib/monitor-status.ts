import type { CheckResult, Monitor } from "./api";

export type LiveMonitor = Monitor & {
  latestCheck?: CheckResult;
  latestCheckError?: string;
};

export type MonitorStatus = "UP" | "DOWN" | "PAUSED" | "AWAITING CHECK";

export function getMonitorStatus(monitor: LiveMonitor): MonitorStatus {
  if (!monitor.active) return "PAUSED";
  if (!monitor.latestCheck) return "AWAITING CHECK";
  return monitor.latestCheck.reachable ? "UP" : "DOWN";
}

export function countMonitorStatuses(monitors: LiveMonitor[]) {
  const counts: Record<MonitorStatus, number> = {
    UP: 0,
    DOWN: 0,
    PAUSED: 0,
    "AWAITING CHECK": 0,
  };
  for (const monitor of monitors) counts[getMonitorStatus(monitor)]++;
  return counts;
}

export function formatCheckStatus(check?: CheckResult) {
  return check && check.status_code > 0 ? String(check.status_code) : "—";
}

export function formatCheckLatency(check?: CheckResult) {
  return check && check.latency_ms > 0 ? `${check.latency_ms} ms` : "—";
}
