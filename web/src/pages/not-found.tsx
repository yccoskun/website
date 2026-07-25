import { Link } from "react-router-dom";

import { useDocumentMeta } from "@/lib/meta";

/** Presentational 404 UI — no document meta side effects. */
export function NotFoundView() {
  return (
    <article>
      <p className="font-mono text-xs tracking-[0.25em] text-ink-600 uppercase dark:text-ink-400">
        404
      </p>
      <h1 className="mt-5 font-display text-4xl leading-tight font-semibold">Lost the trail.</h1>
      <p className="mt-6 font-mono text-sm leading-relaxed text-ink-600 dark:text-ink-400">
        Nothing lives at this URL. It may have moved, or it was never published.
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
  useDocumentMeta("Not found", "No page at this address on yusufcancoskun.com.");
  return <NotFoundView />;
}
