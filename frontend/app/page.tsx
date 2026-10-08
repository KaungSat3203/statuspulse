import MonitorDashboard from "@/components/monitor/monitor-dashboard";
import { getIncidents, getMonitorsWithLatestChecks } from "@/lib/api";
import { ApiError } from "@/lib/api";
import { currentUser, handleServerAuthError, serverAPI } from "@/lib/server-api";
import { redirect } from "next/navigation";

export const dynamic = "force-dynamic";

export default async function Home() {
  const request = await serverAPI();
  const user = await currentUser(request);
  if (!user) redirect("/login");
  const [monitors, incidentResult] = await Promise.all([
    getMonitorsWithLatestChecks(request),
    getIncidents(request)
      .then((incidents) => ({ incidents, error: null }))
      .catch((error) => {
        if (error instanceof ApiError && error.status === 401) throw error;
        return { incidents: [], error: "Unable to load recent incidents right now." };
      }),
  ]).catch(handleServerAuthError);

  return (
    <main className="dashboard-surface min-h-screen text-zinc-100">
      <div className="mx-auto max-w-7xl px-6 py-10">
        <MonitorDashboard
          email={user.email}
          incidentError={incidentResult.error}
          initialIncidents={incidentResult.incidents}
          initialMonitors={monitors}
        />
      </div>
    </main>
  );
}
