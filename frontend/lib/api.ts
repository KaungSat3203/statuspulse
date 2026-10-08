import type { LiveMonitor } from "./monitor-status";

export type Monitor = {
  id: string;
  name: string;
  url: string;
  expected_status_code: number;
  interval_seconds: number;
  timeout_seconds: number;
  active: boolean;
  consecutive_failures: number;
  consecutive_successes: number;
  created_at: string;
  updated_at: string;
};

export type CheckResult = {
  monitor_id: string;
  checked_at: string;
  status_code: number;
  latency_ms: number;
  reachable: boolean;
  error: string;
};

export type Incident = {
  id: string;
  monitor_id: string;
  started_at: string;
  recovered_at?: string;
  downtime_seconds?: number;
  reason: string;
  created_at: string;
};

export type DailyMonitorMetric = {
  monitor_id: string;
  metric_date: string;
  total_checks: number;
  successful_checks: number;
  mean_latency_ms?: number | null;
  p95_latency_ms?: number | null;
};

// Browser requests stay same-origin. Only the server knows the Go upstream URL.
export const API_URL = "/api";
export type User = { id: string; email: string };
export type ApiFetch = (path: string, init?: RequestInit) => Promise<Response>;

export class ApiError extends Error {
  constructor(message: string, readonly status: number) { super(message); }
}

export const apiFetch: ApiFetch = async (path, init = {}) => {
  const headers = new Headers(init.headers);
  if (!["GET", "HEAD"].includes(init.method ?? "GET")) {
    headers.set("X-StatusPulse-CSRF", "1");
  }
  const response = await fetch(API_URL + path, {
    ...init, headers, cache: "no-store", credentials: "same-origin",
  });
  if (response.status === 401 && !["/auth/login", "/auth/signup"].includes(path)) {
    if (typeof window !== "undefined") window.dispatchEvent(new Event("statuspulse:session-ended"));
    throw new ApiError("Your session has expired. Please sign in again.", 401);
  }
  return response;
};

export async function authenticate(mode: "login" | "signup", email: string, password: string): Promise<User> {
  const response = await apiFetch("/auth/" + mode, {
    method: "POST", headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password }),
  });
  if (!response.ok) {
    const body = await response.json().catch(() => null) as { error?: string } | null;
    throw new ApiError(body?.error ?? "Unable to sign in. Please try again.", response.status);
  }
  return response.json();
}

export async function logout(): Promise<void> {
  const response = await apiFetch("/auth/logout", {
    method: "POST", headers: { "Content-Type": "application/json" }, body: "{}",
  });
  if (!response.ok) throw new Error("Unable to sign out. Please try again.");
}

export async function getSessionUser(): Promise<User> {
  const response = await apiFetch("/auth/me");
  if (!response.ok) throw new ApiError("Unable to verify your session.", response.status);
  return response.json();
}

export async function getMonitors(request: ApiFetch = apiFetch): Promise<Monitor[]> {
  const response = await request("/monitors", { cache: "no-store" });

  if (!response.ok) {
    throw new Error("Failed to fetch monitors");
  }

  return response.json();
}

// Hydrate status from persisted checks on every dashboard request.
export async function getMonitorsWithLatestChecks(request: ApiFetch = apiFetch): Promise<LiveMonitor[]> {
  const monitors = await getMonitors(request);
  return Promise.all(
    monitors.map(async (monitor): Promise<LiveMonitor> => {
      try {
        const [latestCheck] = await getMonitorChecks(monitor.id, 1, request);
        return { ...monitor, latestCheck };
      } catch (error) {
        if (error instanceof ApiError && error.status === 401) throw error;
        return {
          ...monitor,
          latestCheckError: "Unable to load the latest check. Waiting for a live update.",
        };
      }
    }),
  );
}

export async function getIncidents(request: ApiFetch = apiFetch): Promise<Incident[]> {
  const response = await request("/incidents");

  if (!response.ok) {
    throw new Error("Failed to fetch incidents");
  }

  return response.json();
}

export type UpdateMonitorInput = {
  name?: string;
  url?: string;
  expected_status_code?: number;
  interval_seconds?: number;
  timeout_seconds?: number;
  active?: boolean;
};

export async function updateMonitor(
  monitorID: string,
  input: UpdateMonitorInput,
): Promise<Monitor> {
  const response = await apiFetch(
    `/monitors/${encodeURIComponent(monitorID)}`,
    {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(input),
    },
  );

  if (!response.ok) {
    let message = "Failed to update monitor";
    try {
      const body = (await response.json()) as { error?: string };
      if (body.error) message = body.error;
    } catch {
      // Use the generic message when the response has no JSON error body.
    }
    throw new Error(message);
  }

  return response.json();
}
export async function updateMonitorActive(
  monitorID: string,
  active: boolean,
): Promise<Monitor> {
  const response = await apiFetch(
    `/monitors/${encodeURIComponent(monitorID)}`,
    {
      method: "PATCH",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ active }),
    },
  );

  if (!response.ok) {
    let message = "Failed to update monitor";

    try {
      const body = (await response.json()) as { error?: string };
      if (body.error) {
        message = body.error;
      }
    } catch {
      // Use the generic message when the response has no JSON error body.
    }

    throw new Error(message);
  }

  return response.json();
}

export async function deleteMonitor(monitorID: string): Promise<void> {
  const response = await apiFetch(
    `/monitors/${encodeURIComponent(monitorID)}`,
    { method: "DELETE" },
  );

  if (!response.ok) {
    let message = "Failed to delete monitor";
    try {
      const body = (await response.json()) as { error?: string };
      if (body.error) message = body.error;
    } catch {
      // Use the generic message when the response has no JSON error body.
    }
    throw new Error(message);
  }
}
export type CreateMonitorInput = {
  name: string;
  url: string;
  expected_status_code: number;
  interval_seconds: number;
  timeout_seconds: number;
};

export async function createMonitor(
  input: CreateMonitorInput,
): Promise<Monitor> {
  const response = await apiFetch(`/monitors`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(input),
  });

  if (!response.ok) {
    let message = "Failed to create monitor";

    try {
      const body = (await response.json()) as { error?: string };
      if (body.error) {
        message = body.error;
      }
    } catch {
      // Use the generic message when the response has no JSON error body.
    }

    throw new Error(message);
  }

  return response.json();
}

export async function getMonitorChecks(
  monitorID: string,
  limit = 50,
  request: ApiFetch = apiFetch,
): Promise<CheckResult[]> {
  const response = await request(
    `/monitors/${encodeURIComponent(monitorID)}/checks?limit=${limit}`,
    { cache: "no-store" },
  );

  if (!response.ok) {
    let message = "Failed to fetch monitor checks";
    try {
      const body = (await response.json()) as { error?: string };
      if (body.error) message = body.error;
    } catch {
      // Use the generic message when the response has no JSON error body.
    }
    throw new Error(message);
  }

  return response.json();
}

export async function getMonitorMetrics(
  monitorID: string,
  limit = 30,
): Promise<DailyMonitorMetric[]> {
  const response = await apiFetch(
    `/monitors/${encodeURIComponent(monitorID)}/metrics?limit=${limit}`,
  );

  if (!response.ok) {
    let message = "Failed to fetch monitor metrics";
    try {
      const body = (await response.json()) as { error?: string };
      if (body.error) message = body.error;
    } catch {
      // Use the generic message when the response has no JSON error body.
    }
    throw new Error(message);
  }

  return response.json();
}

