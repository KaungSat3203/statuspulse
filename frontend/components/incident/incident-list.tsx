import type { Incident, Monitor } from "@/lib/api";
import { formatDowntime, formatIncidentTime } from "@/lib/incident-presentation";

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
    <section aria-labelledby="recent-incidents-heading" className="mt-10">
      <div>
        <h2
          className="text-lg font-semibold tracking-tight"
          id="recent-incidents-heading"
        >
          Recent incidents
        </h2>
        <p className="mt-1 text-sm text-zinc-400">
          Latest open and recovered incidents.
        </p>
      </div>

      {error ? (
        <div
          className="mt-4 rounded-2xl border border-red-900/60 bg-red-950/30 p-5"
          role="alert"
        >
          <p className="text-sm font-medium text-red-300">Unable to load incidents</p>
          <p className="mt-1 text-sm leading-relaxed text-red-300/80">{error}</p>
        </div>
      ) : incidents.length === 0 ? (
        <div className="ui-panel mt-4 px-6 py-8">
          <p className="text-sm font-medium text-zinc-200">No recent incidents</p>
          <p className="mt-2 max-w-xl text-sm leading-relaxed text-zinc-400">
            Incidents will appear here when a monitor reaches its failure threshold.
          </p>
        </div>
      ) : (
        <div className="mt-4 space-y-3">
          {incidents.map((incident) => {
            const isOpen = !incident.recovered_at;
            const monitorName = monitorNames.get(incident.monitor_id) ?? "Unknown monitor";

            return (
              <article
                key={incident.id}
                className={`ui-panel border-l-4 p-5 ${isOpen
                  ? "border-l-red-400 bg-red-950/20"
                  : "border-l-zinc-700"
                }`}
              >
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <h3 className="min-w-0 flex-1 font-semibold tracking-tight text-zinc-100 [overflow-wrap:anywhere]">
                    {monitorName}
                  </h3>
                  <span
                    className={isOpen
                      ? "inline-flex shrink-0 items-center gap-1.5 rounded-full border border-red-400/20 bg-red-500/10 px-2.5 py-1 text-xs font-semibold text-red-300"
                      : "inline-flex shrink-0 items-center gap-1.5 rounded-full border border-zinc-700/70 bg-zinc-800/50 px-2.5 py-1 text-xs font-medium text-zinc-400"
                    }
                  >
                    <span
                      aria-hidden="true"
                      className={`h-1.5 w-1.5 rounded-full ${isOpen ? "bg-red-400" : "bg-emerald-400/70"}`}
                    />
                    {isOpen ? "Open" : "Recovered"}
                  </span>
                </div>

                <p className="mt-3 text-[11px] font-medium uppercase tracking-wider text-zinc-500">
                  Failure reason
                </p>
                <p className={`mt-1 whitespace-pre-wrap text-sm leading-relaxed [overflow-wrap:anywhere] ${isOpen ? "text-red-200" : "text-zinc-300"}`}>
                  {incident.reason || "Monitor check failed"}
                </p>

                <dl className="mt-4 flex flex-wrap gap-x-8 gap-y-3 border-t border-zinc-800/70 pt-4 text-xs">
                  <div>
                    <dt className="text-zinc-500">Started</dt>
                    <dd className="mt-1 tabular-nums text-zinc-400">
                      <time dateTime={incident.started_at}>
                        {formatIncidentTime(incident.started_at)}
                      </time>
                    </dd>
                  </div>
                  {incident.recovered_at && (
                    <div>
                      <dt className="text-zinc-500">Recovered</dt>
                      <dd className="mt-1 tabular-nums text-zinc-400">
                        <time dateTime={incident.recovered_at}>
                          {formatIncidentTime(incident.recovered_at)}
                        </time>
                      </dd>
                    </div>
                  )}
                  {incident.downtime_seconds !== undefined && (
                    <div>
                      <dt className="text-zinc-500">Downtime</dt>
                      <dd className="mt-1 font-medium tabular-nums text-zinc-300">
                        {formatDowntime(incident.downtime_seconds)}
                      </dd>
                    </div>
                  )}
                </dl>
              </article>
            );
          })}
        </div>
      )}
    </section>
  );
}
