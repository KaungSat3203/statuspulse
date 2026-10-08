"use client";

export default function ErrorPage({ reset }: { reset: () => void }) {
  return (
    <main className="dashboard-surface flex min-h-screen items-center justify-center p-6">
      <section role="alert" className="ui-panel max-w-md p-8">
        <h1 className="text-xl font-semibold">Unable to load StatusPulse</h1>
        <p className="mt-3 text-sm leading-6 text-zinc-400">The service may be temporarily unavailable. Please try again.</p>
        <button onClick={reset} className="ui-button-primary mt-6">Try again</button>
      </section>
    </main>
  );
}
