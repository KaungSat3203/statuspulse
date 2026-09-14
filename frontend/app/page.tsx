import MonitorDashboard from "@/components/monitor/monitor-dashboard";
import { getIncidents, getMonitors } from "@/lib/api";

export const dynamic = "force-dynamic";

export default async function Home() {
  const [monitors, incidentResult] = await Promise.all([
    getMonitors(),
    getIncidents()
      .then((incidents) => ({ incidents, error: null }))
      .catch(() => ({
        incidents: [],
        error: "Unable to load recent incidents right now.",
      })),
  ]);

  return (
    <main className="min-h-screen bg-zinc-950 text-white">
      <div className="mx-auto max-w-7xl px-6 py-8">
        <MonitorDashboard
          incidentError={incidentResult.error}
          initialIncidents={incidentResult.incidents}
          initialMonitors={monitors}
        />
      </div>
    </main>
  );
}