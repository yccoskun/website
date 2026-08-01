import { useCallback, useEffect, useState } from "react";
import type { FormEvent } from "react";

import { ErrorState, LoadingState } from "@/components/states";
import { ApiError, apiGet, apiPut } from "@/lib/api";
import { useDocumentMeta } from "@/lib/meta";
import { isValidEmail, isValidHTTPSURL, isValidNavPath } from "@/lib/urls";
import { handleAdminUnauthorized, useAdminSession } from "@/pages/admin/session";
import type { Contact, NavItem } from "@/types/cms";

const fieldLabel =
  "font-mono text-[0.65rem] tracking-[0.16em] text-ink-600 uppercase dark:text-ink-400";
const fieldInput =
  "mt-1 w-full rounded-chip border border-ink-200 bg-paper px-2 py-1 text-sm text-ink-900 outline-none focus:border-ember-600 dark:border-ink-800 dark:bg-ink-950 dark:text-ink-200 dark:focus:border-ember-400";

const KEYS = [
  "site_name",
  "meta_description",
  "rss_title",
  "rss_description",
  "monogram",
  "nav",
  "contact",
] as const;

const DEFAULTS: Record<(typeof KEYS)[number], string> = {
  site_name: "",
  meta_description: "",
  rss_title: "",
  rss_description: "",
  monogram: "YCC",
  nav: '[{"label":"Home","path":"/"},{"label":"Notes","path":"/blog"},{"label":"Work","path":"/work"},{"label":"Studio","path":"/studio"},{"label":"Resume","path":"/resume"}]',
  contact: '{"email":"","github":"","linkedin":""}',
};

export function AdminSettingsPage() {
  const { onUnauthorized } = useAdminSession();
  const [form, setForm] = useState<Record<(typeof KEYS)[number], string>>({ ...DEFAULTS });
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);

  useDocumentMeta("Admin · Settings", "Edit site-wide settings.");

  const load = useCallback(() => {
    setLoading(true);
    setError(null);
    void apiGet<Record<string, string>>("/api/admin/settings")
      .then((data) => {
        const next = { ...DEFAULTS };
        for (const k of KEYS) {
          if (data[k] !== undefined) next[k] = data[k];
        }
        setForm(next);
        setLoading(false);
      })
      .catch((err: unknown) => {
        if (handleAdminUnauthorized(err, onUnauthorized)) return;
        setError(err instanceof ApiError ? err.message : "Failed to load settings");
        setLoading(false);
      });
  }, [onUnauthorized]);

  useEffect(() => {
    load();
  }, [load]);

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    setSaving(true);
    setSaved(false);
    setError(null);

    try {
      const nav = JSON.parse(form.nav) as NavItem[];
      if (!Array.isArray(nav)) {
        setError("nav must be a JSON array");
        setSaving(false);
        return;
      }
      for (const item of nav) {
        if (!isValidNavPath(item.path ?? "")) {
          setError(`nav path must be a relative path starting with / (got ${JSON.stringify(item.path)})`);
          setSaving(false);
          return;
        }
      }
    } catch {
      setError("nav must be valid JSON");
      setSaving(false);
      return;
    }

    try {
      const contact = JSON.parse(form.contact) as Contact;
      if (!isValidEmail(contact.email ?? "")) {
        setError("contact.email is invalid");
        setSaving(false);
        return;
      }
      if (!isValidHTTPSURL(contact.github ?? "", true)) {
        setError("contact.github must be empty or an https:// URL");
        setSaving(false);
        return;
      }
      if (!isValidHTTPSURL(contact.linkedin ?? "", true)) {
        setError("contact.linkedin must be empty or an https:// URL");
        setSaving(false);
        return;
      }
    } catch {
      setError("contact must be valid JSON");
      setSaving(false);
      return;
    }

    void apiPut<Record<string, string>>("/api/admin/settings", { settings: form })
      .then(() => {
        setSaving(false);
        setSaved(true);
      })
      .catch((err: unknown) => {
        if (handleAdminUnauthorized(err, onUnauthorized)) return;
        setError(err instanceof ApiError ? err.message : "Failed to save");
        setSaving(false);
      });
  }

  if (loading) return <LoadingState />;
  if (error && Object.values(form).every((v) => v === DEFAULTS.site_name || v === "")) {
    return <ErrorState message={error} />;
  }

  return (
    <div className="max-w-2xl">
      <h1 className="font-display text-2xl font-semibold">Settings</h1>
      <p className="mt-1 font-mono text-xs text-ink-600 dark:text-ink-400">
        Site chrome, nav JSON, contact JSON, RSS channel text.
      </p>
      <form onSubmit={onSubmit} className="mt-6 space-y-4">
        {KEYS.map((key) => (
          <label key={key} className="block">
            <span className={fieldLabel}>{key}</span>
            {key === "nav" || key === "contact" || key === "meta_description" || key === "rss_description" ? (
              <textarea
                className={`${fieldInput} min-h-24 font-mono text-xs`}
                value={form[key] ?? ""}
                onChange={(e) => setForm((f) => ({ ...f, [key]: e.target.value }))}
              />
            ) : (
              <input
                className={fieldInput}
                value={form[key] ?? ""}
                onChange={(e) => setForm((f) => ({ ...f, [key]: e.target.value }))}
              />
            )}
          </label>
        ))}
        {error ? <p className="font-mono text-xs text-ember-600">{error}</p> : null}
        {saved ? <p className="font-mono text-xs text-ink-600 dark:text-ink-400">Saved.</p> : null}
        <button
          type="submit"
          disabled={saving}
          className="rounded-chip border border-ink-900 bg-ink-900 px-3 py-1.5 font-mono text-xs tracking-[0.14em] text-paper uppercase transition-transform active:translate-y-0.5 disabled:opacity-50 dark:border-paper dark:bg-paper dark:text-ink-950"
        >
          {saving ? "Saving…" : "Save"}
        </button>
      </form>
    </div>
  );
}
