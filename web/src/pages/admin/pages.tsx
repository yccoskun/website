import { useCallback, useEffect, useState } from "react";
import type { FormEvent } from "react";

import { ErrorState, LoadingState } from "@/components/states";
import { ApiError, apiGet, apiPut } from "@/lib/api";
import { useDocumentMeta } from "@/lib/meta";
import { isValidNavPath } from "@/lib/urls";
import { handleAdminUnauthorized, useAdminSession } from "@/pages/admin/session";
import type { HomeBody, Page } from "@/types/cms";

const SLUGS = ["home", "work", "studio", "notes", "resume", "not_found"] as const;

const fieldLabel =
  "font-mono text-[0.65rem] tracking-[0.16em] text-ink-600 uppercase dark:text-ink-400";
const fieldInput =
  "mt-1 w-full rounded-chip border border-ink-200 bg-paper px-2 py-1 text-sm text-ink-900 outline-none focus:border-ember-600 dark:border-ink-800 dark:bg-ink-950 dark:text-ink-200 dark:focus:border-ember-400";

const EMPTY_BODIES: Record<(typeof SLUGS)[number], string> = {
  home: JSON.stringify(
    {
      eyebrow: "",
      headline: "",
      intro: "",
      domains: [],
      now: "",
      accordion: false,
    },
    null,
    2,
  ),
  work: JSON.stringify(
    {
      eyebrow: "Work",
      headline: "",
      intro: "",
      empty_message: "Nothing listed yet.",
      accordion: false,
    },
    null,
    2,
  ),
  studio: JSON.stringify(
    {
      eyebrow: "Studio",
      headline: "",
      intro: "",
      tools_line: "",
      empty_message: "Nothing listed yet.",
    },
    null,
    2,
  ),
  notes: JSON.stringify(
    {
      eyebrow: "Notes",
      headline: "",
      intro: "",
      empty_message: "Nothing published yet.",
    },
    null,
    2,
  ),
  resume: JSON.stringify(
    {
      eyebrow: "Resume",
      headline: "",
      blurb: "",
      pdf_media_id: null,
    },
    null,
    2,
  ),
  not_found: JSON.stringify(
    { eyebrow: "404", headline: "Lost the trail.", body: "Nothing lives at this URL." },
    null,
    2,
  ),
};

export function AdminPagesPage() {
  const { onUnauthorized } = useAdminSession();
  const [slug, setSlug] = useState<(typeof SLUGS)[number]>("home");
  const [title, setTitle] = useState("");
  const [meta, setMeta] = useState("");
  const [body, setBody] = useState(EMPTY_BODIES.home);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);

  useDocumentMeta("Admin · Pages", "Edit CMS page documents.");

  const load = useCallback(
    (s: (typeof SLUGS)[number]) => {
      setLoading(true);
      setError(null);
      setSaved(false);
      void apiGet<Page>(`/api/admin/pages/${s}`)
        .then((page) => {
          setTitle(page.title);
          setMeta(page.meta_description);
          setBody(
            page.body_json && page.body_json !== "{}"
              ? JSON.stringify(JSON.parse(page.body_json) as unknown, null, 2)
              : EMPTY_BODIES[s],
          );
          setLoading(false);
        })
        .catch((err: unknown) => {
          if (handleAdminUnauthorized(err, onUnauthorized)) return;
          setError(err instanceof ApiError ? err.message : "Failed to load page");
          setBody(EMPTY_BODIES[s]);
          setLoading(false);
        });
    },
    [onUnauthorized],
  );

  useEffect(() => {
    load(slug);
  }, [load, slug]);

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    setSaving(true);
    setError(null);
    setSaved(false);
    let bodyJson = body;
    let parsed: unknown;
    try {
      parsed = JSON.parse(body) as unknown;
      bodyJson = JSON.stringify(parsed);
    } catch {
      setError("body_json must be valid JSON");
      setSaving(false);
      return;
    }
    if (slug === "home") {
      const home = parsed as HomeBody;
      const domains = home.domains ?? [];
      for (let i = 0; i < domains.length; i++) {
        const link = domains[i]?.link;
        if (!link) continue;
        if (!isValidNavPath(link.to ?? "")) {
          setError(`domains[${i}].link.to must be a relative path starting with /`);
          setSaving(false);
          return;
        }
      }
    }
    void apiPut<Page>(`/api/admin/pages/${slug}`, {
      title,
      meta_description: meta,
      body_json: bodyJson,
    })
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

  return (
    <div className="max-w-3xl">
      <h1 className="font-display text-2xl font-semibold">Pages</h1>
      <div className="mt-4 flex flex-wrap gap-1">
        {SLUGS.map((s) => (
          <button
            key={s}
            type="button"
            onClick={() => setSlug(s)}
            className={`rounded-chip px-2 py-1 font-mono text-[0.65rem] tracking-[0.14em] uppercase transition-transform active:translate-y-0.5 ${
              slug === s
                ? "bg-ink-900 text-paper dark:bg-paper dark:text-ink-950"
                : "border border-ink-200 text-ink-600 dark:border-ink-800 dark:text-ink-400"
            }`}
          >
            {s}
          </button>
        ))}
      </div>
      {loading ? (
        <div className="mt-6">
          <LoadingState />
        </div>
      ) : (
        <form onSubmit={onSubmit} className="mt-6 space-y-4">
          <label className="block">
            <span className={fieldLabel}>title</span>
            <input className={fieldInput} value={title} onChange={(e) => setTitle(e.target.value)} />
          </label>
          <label className="block">
            <span className={fieldLabel}>meta_description</span>
            <input className={fieldInput} value={meta} onChange={(e) => setMeta(e.target.value)} />
          </label>
          <label className="block">
            <span className={fieldLabel}>body_json</span>
            <textarea
              className={`${fieldInput} min-h-80 font-mono text-xs`}
              value={body}
              onChange={(e) => setBody(e.target.value)}
              spellCheck={false}
            />
          </label>
          {error ? <ErrorState message={error} /> : null}
          {saved ? <p className="font-mono text-xs text-ink-600 dark:text-ink-400">Saved.</p> : null}
          <button
            type="submit"
            disabled={saving}
            className="rounded-chip border border-ink-900 bg-ink-900 px-3 py-1.5 font-mono text-xs tracking-[0.14em] text-paper uppercase transition-transform active:translate-y-0.5 disabled:opacity-50 dark:border-paper dark:bg-paper dark:text-ink-950"
          >
            {saving ? "Saving…" : "Save"}
          </button>
        </form>
      )}
    </div>
  );
}
