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
  mean_latency_ms: number | null;
  p95_latency_ms: number | null;
};

export const API_URL =
  process.env.NEXT_PUBLIC_STATUSPULSE_API_URL ?? "http://localhost:8080";

export async function getMonitors(): Promise<Monitor[]> {
  const response = await fetch(`${API_URL}/monitors`);

  if (!response.ok) {
    throw new Error("Failed to fetch monitors");
  }

  return response.json();
}

export async function getIncidents(): Promise<Incident[]> {
  const response = await fetch(`${API_URL}/incidents`);

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
  const response = await fetch(
    `${API_URL}/monitors/${encodeURIComponent(monitorID)}`,
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
  const response = await fetch(
    `${API_URL}/monitors/${encodeURIComponent(monitorID)}`,
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
  const response = await fetch(
    `${API_URL}/monitors/${encodeURIComponent(monitorID)}`,
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
  const response = await fetch(`${API_URL}/monitors`, {
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
): Promise<CheckResult[]> {
  const response = await fetch(
    `${API_URL}/monitors/${encodeURIComponent(monitorID)}/checks?limit=${limit}`,
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
  const response = await fetch(
    `${API_URL}/monitors/${encodeURIComponent(monitorID)}/metrics?limit=${limit}`,
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

