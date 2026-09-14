"use client";

import { useState } from "react";

import { deleteMonitor, type Monitor } from "@/lib/api";

type DeleteMonitorDialogProps = {
  monitor: Monitor;
  onDeleted: (monitorID: string) => void;
};

export default function DeleteMonitorDialog({ monitor, onDeleted }: DeleteMonitorDialogProps) {
  const [isOpen, setIsOpen] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const close = () => { if (!isDeleting) { setIsOpen(false); setError(null); } };
  const confirm = async () => {
    setError(null); setIsDeleting(true);
    try { await deleteMonitor(monitor.id); onDeleted(monitor.id); setIsOpen(false); }
    catch (deletionError) { setError(deletionError instanceof Error ? deletionError.message : "Unable to delete monitor"); }
    finally { setIsDeleting(false); }
  };

  return <>
    <button className="rounded-lg border border-red-900 px-3 py-1.5 text-sm font-medium text-red-300 hover:border-red-700 hover:text-red-200" onClick={() => setIsOpen(true)} type="button">Delete</button>
    {isOpen && <div aria-modal="true" className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 px-4" role="dialog">
      <div className="w-full max-w-md rounded-xl border border-zinc-800 bg-zinc-900 p-6 shadow-2xl">
        <h2 className="text-lg font-medium">Delete Monitor</h2>
        <p className="mt-2 text-sm text-zinc-400">Delete <span className="font-medium text-zinc-200">{monitor.name}</span>? Its check logs, incidents, and metrics will also be removed.</p>
        {error && <p className="mt-4 rounded-lg border border-red-900 bg-red-950/50 px-3 py-2 text-sm text-red-300">{error}</p>}
        <div className="mt-6 flex justify-end gap-3"><button className="rounded-lg px-4 py-2 text-sm text-zinc-300 hover:text-white" disabled={isDeleting} onClick={close} type="button">Cancel</button><button className="rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white hover:bg-red-500 disabled:cursor-not-allowed disabled:opacity-60" disabled={isDeleting} onClick={confirm} type="button">{isDeleting ? "Deleting…" : "Delete Monitor"}</button></div>
      </div>
    </div>}
  </>;
}