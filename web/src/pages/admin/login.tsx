import { useState } from "react";
import type { FormEvent } from "react";

import { useDocumentMeta } from "@/lib/meta";
import { useAdminSession } from "@/pages/admin/session";

export function LoginPage() {
  const { login, reauthRequired } = useAdminSession();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  useDocumentMeta("Admin login", "Sign in to edit posts and resume entries.");

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await login(username, password);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Login failed");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="min-h-dvh bg-paper font-sans text-ink-900 dark:bg-ink-950 dark:text-ink-200">
      {/* Offset card — left-heavy, not dead-center. */}
      <div className="px-5 pt-20 pb-16 md:pl-24 lg:pl-40 md:pr-10">
        <div className="max-w-sm rounded-card border border-ink-200 bg-paper-raised p-6 dark:border-ink-800 dark:bg-ink-900">
          <p className="font-mono text-[0.65rem] tracking-[0.22em] text-ink-600 uppercase dark:text-ink-400">
            YCC admin
          </p>
          <h1 className="mt-3 font-display text-2xl font-semibold tracking-tight">
            Sign in to edit.
          </h1>
          <p className="mt-2 text-sm text-ink-600 dark:text-ink-400">
            Session cookie only — lasts 24 hours, then you sign in again.
          </p>

          {reauthRequired ? (
            <p
              role="status"
              className="mt-4 border-l-2 border-ember-600 pl-3 font-mono text-xs text-ink-600 dark:border-ember-400 dark:text-ink-400"
            >
              Session ended — sign in again.
            </p>
          ) : null}

          <form className="mt-6 space-y-4" onSubmit={onSubmit}>
            <label className="block">
              <span className="font-mono text-[0.65rem] tracking-[0.18em] text-ink-600 uppercase dark:text-ink-400">
                Username
              </span>
              <input
                type="text"
                name="username"
                autoComplete="username"
                required
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                className="mt-1.5 w-full rounded-chip border border-ink-200 bg-paper px-2.5 py-1.5 font-mono text-sm text-ink-900 outline-none focus:border-ember-600 dark:border-ink-800 dark:bg-ink-950 dark:text-ink-200 dark:focus:border-ember-400"
              />
            </label>
            <label className="block">
              <span className="font-mono text-[0.65rem] tracking-[0.18em] text-ink-600 uppercase dark:text-ink-400">
                Password
              </span>
              <input
                type="password"
                name="password"
                autoComplete="current-password"
                required
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                className="mt-1.5 w-full rounded-chip border border-ink-200 bg-paper px-2.5 py-1.5 font-mono text-sm text-ink-900 outline-none focus:border-ember-600 dark:border-ink-800 dark:bg-ink-950 dark:text-ink-200 dark:focus:border-ember-400"
              />
            </label>

            {error ? (
              <p role="alert" className="font-mono text-sm text-ember-600 dark:text-ember-400">
                {error}
              </p>
            ) : null}

            <button
              type="submit"
              disabled={submitting}
              className="rounded-chip border border-ink-900 bg-ink-900 px-3 py-1.5 font-mono text-xs tracking-[0.14em] text-paper uppercase transition-transform enabled:active:translate-y-0.5 disabled:opacity-50 dark:border-paper dark:bg-paper dark:text-ink-950"
            >
              {submitting ? "Signing in…" : "Sign in"}
            </button>
          </form>
        </div>
      </div>
    </div>
  );
}
