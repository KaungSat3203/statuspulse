"use client";

import { useState, type FormEvent } from "react";

import { type Monitor, updateMonitor } from "@/lib/api";

type EditMonitorDialogProps = {
  monitor: Monitor;
  onUpdated: (monitor: Monitor) => void;
};

type Values = {
  name: string;
  url: string;
  expectedStatusCode: string;
  intervalSeconds: string;
  timeoutSeconds: string;
};

function valuesFor(monitor: Monitor): Values {
  return {
    name: monitor.name,
    url: monitor.url,
    expectedStatusCode: String(monitor.expected_status_code),
    intervalSeconds: String(monitor.interval_seconds),
    timeoutSeconds: String(monitor.timeout_seconds),
  };
}

export default function EditMonitorDialog({ monitor, onUpdated }: EditMonitorDialogProps) {
  const [isOpen, setIsOpen] = useState(false);
  const [values, setValues] = useState<Values>(() => valuesFor(monitor));
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const open = () => { setValues(valuesFor(monitor)); setError(null); setIsOpen(true); };
  const close = () => { if (!isSubmitting) { setIsOpen(false); setError(null); } };

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const expectedStatusCode = Number(values.expectedStatusCode);
    const intervalSeconds = Number(values.intervalSeconds);
    const timeoutSeconds = Number(values.timeoutSeconds);
    if (!values.name.trim()) { setError("Monitor name is required."); return; }
    try {
      const url = new URL(values.url);
      if (url.protocol !== "http:" && url.protocol !== "https:") throw new Error();
    } catch { setError("Enter a valid HTTP or HTTPS URL."); return; }
    if (!Number.isInteger(expectedStatusCode) || expectedStatusCode < 100 || expectedStatusCode > 599) { setError("Expected HTTP status must be between 100 and 599."); return; }
    if (!Number.isInteger(intervalSeconds) || intervalSeconds < 5 || intervalSeconds > 3600) { setError("Interval must be between 5 and 3600 seconds."); return; }
    if (!Number.isInteger(timeoutSeconds) || timeoutSeconds < 1 || timeoutSeconds > 300) { setError("Timeout must be between 1 and 300 seconds."); return; }

    setError(null); setIsSubmitting(true);
    try {
      const updated = await updateMonitor(monitor.id, { name: values.name.trim(), url: values.url.trim(), expected_status_code: expectedStatusCode, interval_seconds: intervalSeconds, timeout_seconds: timeoutSeconds });
      onUpdated(updated); setIsOpen(false);
    } catch (submissionError) {
      setError(submissionError instanceof Error ? submissionError.message : "Unable to update monitor");
    } finally { setIsSubmitting(false); }
  };

  return <>
    <button className="rounded-lg border border-zinc-700 px-3 py-1.5 text-sm font-medium text-zinc-200 hover:border-zinc-500 hover:text-white" onClick={open} type="button">Edit</button>
    {isOpen && <div aria-modal="true" className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 px-4" role="dialog">
      <form className="w-full max-w-lg rounded-xl border border-zinc-800 bg-zinc-900 p-6 shadow-2xl" onSubmit={submit}>
        <div className="flex items-start justify-between gap-4"><div><h2 className="text-lg font-medium">Edit Monitor</h2><p className="mt-1 text-sm text-zinc-400">Update this monitor&apos;s configuration.</p></div><button aria-label="Close edit monitor form" className="text-zinc-400 hover:text-white" onClick={close} type="button">×</button></div>
        <div className="mt-6 space-y-4">
          <Field label="Name"><input autoFocus className="w-full rounded-lg border border-zinc-700 bg-zinc-950 px-3 py-2 text-sm outline-none focus:border-zinc-400" onChange={(event) => setValues({ ...values, name: event.target.value })} required value={values.name} /></Field>
          <Field label="URL"><input className="w-full rounded-lg border border-zinc-700 bg-zinc-950 px-3 py-2 text-sm outline-none focus:border-zinc-400" onChange={(event) => setValues({ ...values, url: event.target.value })} required type="url" value={values.url} /></Field>
          <div className="grid gap-4 sm:grid-cols-3"><Field label="Expected HTTP status"><input className="w-full rounded-lg border border-zinc-700 bg-zinc-950 px-3 py-2 text-sm outline-none focus:border-zinc-400" min="100" max="599" onChange={(event) => setValues({ ...values, expectedStatusCode: event.target.value })} required type="number" value={values.expectedStatusCode} /></Field><Field label="Interval (seconds)"><input className="w-full rounded-lg border border-zinc-700 bg-zinc-950 px-3 py-2 text-sm outline-none focus:border-zinc-400" min="5" max="3600" onChange={(event) => setValues({ ...values, intervalSeconds: event.target.value })} required type="number" value={values.intervalSeconds} /></Field><Field label="Timeout (seconds)"><input className="w-full rounded-lg border border-zinc-700 bg-zinc-950 px-3 py-2 text-sm outline-none focus:border-zinc-400" min="1" max="300" onChange={(event) => setValues({ ...values, timeoutSeconds: event.target.value })} required type="number" value={values.timeoutSeconds} /></Field></div>
        </div>
        {error && <p className="mt-4 rounded-lg border border-red-900 bg-red-950/50 px-3 py-2 text-sm text-red-300">{error}</p>}
        <div className="mt-6 flex justify-end gap-3"><button className="rounded-lg px-4 py-2 text-sm text-zinc-300 hover:text-white" onClick={close} type="button">Cancel</button><button className="rounded-lg bg-white px-4 py-2 text-sm font-medium text-zinc-950 hover:bg-zinc-200 disabled:cursor-not-allowed disabled:opacity-60" disabled={isSubmitting} type="submit">{isSubmitting ? "Saving…" : "Save changes"}</button></div>
      </form>
    </div>}
  </>;
}

function Field({ label, children }: { label: string; children: React.ReactNode }) { return <label className="block text-sm text-zinc-300"><span className="mb-1.5 block">{label}</span>{children}</label>; }