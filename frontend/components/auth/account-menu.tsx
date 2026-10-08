"use client";

import { useState } from "react";
import { logout } from "@/lib/api";

export default function AccountMenu({ email }: { email: string }) {
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  async function signOut() {
    setPending(true);
    setError(null);
    try {
      await logout();
      window.dispatchEvent(new Event("statuspulse:session-ended"));
      window.location.replace("/login");
    } catch {
      setError("Unable to sign out. Please try again.");
      setPending(false);
    }
  }
  return (
    <details className="relative">
      <summary className="ui-button max-w-52 cursor-pointer list-none" aria-label="Account menu">
        <span className="truncate">{email}</span><span aria-hidden="true" className="ml-2">⌄</span>
      </summary>
      <div className="ui-panel absolute right-0 z-20 mt-2 w-64 max-w-[85vw] p-4">
        <p className="text-xs text-zinc-500">Signed in as</p>
        <p className="mt-1 break-all text-sm text-zinc-200">{email}</p>
        <button type="button" onClick={signOut} disabled={pending} className="ui-button mt-4 w-full">
          {pending ? "Signing out…" : "Sign out"}
        </button>
        {error && <p role="alert" className="mt-3 text-xs text-red-300">{error}</p>}
      </div>
    </details>
  );
}
