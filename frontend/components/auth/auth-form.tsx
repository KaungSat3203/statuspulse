"use client";

import Link from "next/link";
import { useState, type FormEvent } from "react";
import { authenticate } from "@/lib/api";

export default function AuthForm({ mode }: { mode: "login" | "signup" }) {
  const signup = mode === "signup";
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [visible, setVisible] = useState(false);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending) return;
    setError(null);
    if (signup && ([...password].length < 15 || [...password].length > 128)) {
      setError("Choose a password between 15 and 128 characters.");
      return;
    }
    setPending(true);
    try {
      await authenticate(mode, email.trim(), password);
      // Replace the document to discard cached state from any previous account.
      window.location.replace("/");
    } catch (error) {
      setError(error instanceof Error ? error.message : "Unable to connect. Please try again.");
      setPending(false);
    }
  }

  return (
    <main className="dashboard-surface flex min-h-screen items-center justify-center px-4 py-12">
      <section className="ui-panel w-full max-w-md p-7 sm:p-9" aria-labelledby="auth-title">
        <p className="text-sm font-semibold tracking-wide text-indigo-400">StatusPulse</p>
        <h1 id="auth-title" className="mt-5 text-2xl font-semibold tracking-tight">
          {signup ? "Create your account" : "Welcome back"}
        </h1>
        <p className="mt-2 text-sm leading-6 text-zinc-400">
          {signup ? "Your services, checks, and incidents in one place." : "Sign in to keep an eye on your services."}
        </p>
        <form onSubmit={submit} className="mt-7 space-y-5" aria-busy={pending}>
          <div>
            <label htmlFor="email" className="mb-2 block text-sm font-medium text-zinc-300">Email</label>
            <input id="email" name="email" type="email" autoComplete="email" required maxLength={254}
              className="ui-input" value={email} onChange={(event) => setEmail(event.target.value)}
              disabled={pending} aria-describedby={error ? "auth-error" : undefined} />
          </div>
          <div>
            <label htmlFor="password" className="mb-2 block text-sm font-medium text-zinc-300">Password</label>
            <div className="relative">
              <input id="password" name="password" type={visible ? "text" : "password"}
                autoComplete={signup ? "new-password" : "current-password"} required
                maxLength={256} className="ui-input pr-20" value={password}
                onChange={(event) => setPassword(event.target.value)} disabled={pending}
                aria-describedby={signup ? "password-help" : undefined} />
              <button type="button" className="absolute inset-y-0 right-1 px-3 text-xs text-zinc-400 hover:text-white focus-visible:outline-2 focus-visible:outline-indigo-400"
                aria-label={visible ? "Hide password" : "Show password"} aria-pressed={visible}
                onClick={() => setVisible(!visible)}>{visible ? "Hide" : "Show"}</button>
            </div>
            {signup && <p id="password-help" className="mt-2 text-xs leading-5 text-zinc-500">
              15–128 characters. A long, unique passphrase works well. Spaces are welcome.
            </p>}
          </div>
          {error && <p id="auth-error" role="alert" className="rounded-lg border border-red-500/20 bg-red-500/10 p-3 text-sm text-red-300">{error}</p>}
          <button type="submit" disabled={pending} className="ui-button-primary w-full">
            {pending ? (signup ? "Creating account…" : "Signing in…") : (signup ? "Create account" : "Sign in")}
          </button>
        </form>
        <p className="mt-6 text-center text-sm text-zinc-400">
          {signup ? "Already have an account? " : "New to StatusPulse? "}
          <Link className="font-medium text-indigo-400 hover:text-indigo-300" href={signup ? "/login" : "/signup"}>
            {signup ? "Sign in" : "Create an account"}
          </Link>
        </p>
      </section>
    </main>
  );
}
