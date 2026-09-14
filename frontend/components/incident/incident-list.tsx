import type { Incident, Monitor } from "@/lib/api";

type IncidentListProps = {
  incidents: Incident[];
  monitors: Monitor[];
  error: string | null;
};

export default function IncidentList({
  incidents,
  monitors,
  error,
}: IncidentListProps) {
  const monitorNames = new Map(monitors.map((monitor) => [monitor.id, monitor.name]));

  return (
    <section className="mt-10">
      <h2 className="text-lg font-medium">Recent Incidents</h2>

      {error ? (
        <p className="mt-4 rounded-xl border border-red-900 bg-red-950/50 p-5 text-sm text-red-300">
          {error}
        </p>
      ) : incidents.length === 0 ? (
        <p className="mt-4 rounded-xl border border-zinc-800 bg-zinc-900 p-5 text-sm text-zinc-400">
          No recent incidents.
        </p>
      ) : (
        <div className="mt-4 space-y-3">
          {incidents.map((incident) => {
            const isOpen = !incident.recovered_at;
            const monitorName = monitorNames.get(incident.monitor_id) ?? "Unknown monitor";

            return (
              <div
                key={incident.id}
                className="rounded-xl border border-zinc-800 bg-zinc-900 p-5"
              >
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <h3 className="font-medium">{monitorName}</h3>
                    <p className="mt-1 text-sm text-zinc-400">
                      Started: {formatIncidentTime(incident.started_at)}
                    </p>
                  </div>
                  <span
                    className={
                      isOpen
                        ? "rounded-full bg-red-950 px-2.5 py-1 text-xs font-medium text-red-300"
                        : "rounded-full bg-green-950 px-2.5 py-1 text-xs font-medium text-green-300"
                    }
                  >
                    {isOpen ? "Open" : "Recovered"}
                  </span>
                </div>

                <div className="mt-4 flex flex-wrap gap-x-6 gap-y-2 text-sm text-zinc-400">
                  {incident.recovered_at && (
                    <span>Recovered: {formatIncidentTime(incident.recovered_at)}</span>
                  )}
                  {incident.downtime_seconds !== undefined && (
                    <span>Downtime: {formatDowntime(incident.downtime_seconds)}</span>
                  )}
                </div>

                <p className="mt-3 text-sm text-zinc-300">
                  Reason: {incident.reason || "Monitor check failed"}
                </p>
              </div>
            );
          })}
        </div>
      )}
    </section>
  );
}

function formatIncidentTime(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "—" : date.toLocaleString();
}

function formatDowntime(seconds: number) {
  if (seconds < 60) return `${seconds}s`;

  const minutes = Math.floor(seconds / 60);
  const remainingSeconds = seconds % 60;

  return remainingSeconds ? `${minutes}m ${remainingSeconds}s` : `${minutes}m`;
}