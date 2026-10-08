type LoadingProps = { label: string; variant: "history" | "analytics" };

export function DialogLoadingState({ label, variant }: LoadingProps) {
  return (
    <div role="status" aria-live="polite" aria-busy="true">
      <p className="mb-4 text-sm text-zinc-400">{label}</p>
      <div aria-hidden="true" className="motion-safe:animate-pulse">
        {variant === "history" ? (
          <div className="divide-y divide-zinc-800 rounded-xl border border-zinc-800 bg-zinc-950/40">
            {[0, 1, 2, 3].map((row) => (
              <div className="flex items-center gap-4 px-4 py-5" key={row}>
                <div className="h-3 w-2/5 rounded bg-zinc-800" />
                <div className="h-3 w-10 rounded bg-zinc-800" />
                <div className="h-3 w-16 rounded bg-zinc-800" />
              </div>
            ))}
          </div>
        ) : (
          <>
            <div className="grid grid-cols-2 gap-4 xl:grid-cols-4">
              {[0, 1, 2, 3].map((card) => (
                <div
                  className="rounded-xl border border-zinc-800 bg-zinc-950/40 p-4"
                  key={card}
                >
                  <div className="h-3 w-3/4 rounded bg-zinc-800" />
                  <div className="mt-4 h-6 w-1/2 rounded bg-zinc-800" />
                </div>
              ))}
            </div>
            <div className="mt-6 h-64 rounded-xl border border-zinc-800 bg-zinc-950/40" />
          </>
        )}
      </div>
    </div>
  );
}

export function DialogErrorState({
  title,
  message,
  onRetry,
}: {
  title: string;
  message: string;
  onRetry: () => void;
}) {
  return (
    <div className="rounded-xl border border-red-900/60 bg-red-950/30 p-5">
      <div role="alert">
        <p className="text-sm font-medium text-red-300">{title}</p>
        <p className="mt-2 text-sm leading-relaxed text-red-300/80 [overflow-wrap:anywhere]">
          {message}
        </p>
      </div>
      <button className="ui-button mt-4" onClick={onRetry} type="button">
        Retry
      </button>
    </div>
  );
}

export function DialogEmptyState({
  title,
  description,
}: {
  title: string;
  description: string;
}) {
  return (
    <div className="rounded-xl border border-zinc-800 bg-zinc-950/40 px-5 py-8">
      <p className="text-sm font-medium text-zinc-200">{title}</p>
      <p className="mt-2 max-w-xl text-sm leading-relaxed text-zinc-400">
        {description}
      </p>
    </div>
  );
}
