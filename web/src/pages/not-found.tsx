import { Link } from "react-router";

import { ErrorState, LoadingState } from "@/components/states";
import { useDocumentMeta } from "@/lib/meta";
import { useApi } from "@/lib/use-api";
import type { NotFoundPageBody, Page } from "@/types/cms";

function parseBody(raw: string): NotFoundPageBody {
  try {
    const parsed = JSON.parse(raw) as NotFoundPageBody;
    return {
      eyebrow: parsed.eyebrow || "404",
      headline: parsed.headline || "Lost the trail.",
      body: parsed.body || "Nothing lives at this URL.",
    };
  } catch {
    return {
      eyebrow: "404",
      headline: "Lost the trail.",
      body: "Nothing lives at this URL.",
    };
  }
}

/** Presentational 404 UI — no document meta side effects. */
export function NotFoundView({ body }: { body?: NotFoundPageBody }) {
  const b = body ?? {
    eyebrow: "404",
    headline: "Lost the trail.",
    body: "Nothing lives at this URL.",
  };
  return (
    <article>
      <p className="font-mono text-xs tracking-[0.25em] text-ink-600 uppercase dark:text-ink-400">
        {b.eyebrow}
      </p>
      <h1 className="mt-5 font-display text-4xl leading-tight font-semibold">{b.headline}</h1>
      <p className="mt-6 font-mono text-sm leading-relaxed text-ink-600 dark:text-ink-400">
        {b.body}
      </p>
      <Link
        to="/"
        className="mt-10 inline-block font-mono text-xs uppercase tracking-[0.18em] text-ember-600 transition-[transform,color] hover:text-ember-700 active:translate-y-0.5 dark:text-ember-400 dark:hover:text-ember-500"
      >
        Back home
      </Link>
    </article>
  );
}

export function NotFoundPage() {
  const page = useApi<Page>("/api/pages/not_found");
  const body = page.data ? parseBody(page.data.body_json) : undefined;

  useDocumentMeta(
    page.data?.title || "Not found",
    page.data?.meta_description || "No page at this address.",
  );

  if (page.loading) return <LoadingState />;
  if (page.error) return <ErrorState message={page.error.message} />;
  return <NotFoundView body={body} />;
}
