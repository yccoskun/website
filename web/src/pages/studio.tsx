import { useEffect, useId, useRef, useState } from "react";

import { EmptyState, ErrorState, LoadingState } from "@/components/states";
import { useDocumentMeta } from "@/lib/meta";
import { useApi } from "@/lib/use-api";
import type { Page, StudioPageBody, StudioPiece } from "@/types/cms";

function parseStudioBody(raw: string): StudioPageBody {
  try {
    const parsed = JSON.parse(raw) as StudioPageBody;
    return {
      eyebrow: parsed.eyebrow ?? "Studio",
      headline: parsed.headline ?? "",
      intro: parsed.intro ?? "",
      tools_line: parsed.tools_line ?? "",
      empty_message:
        parsed.empty_message ||
        "Nothing listed yet — stills and notes are still being gathered.",
    };
  } catch {
    return {
      eyebrow: "Studio",
      headline: "",
      intro: "",
      tools_line: "",
      empty_message: "Nothing listed yet.",
    };
  }
}

export function StudioPage() {
  const page = useApi<Page>("/api/pages/studio");
  const pieces = useApi<StudioPiece[]>("/api/studio");
  const [active, setActive] = useState<StudioPiece | null>(null);

  const body = page.data ? parseStudioBody(page.data.body_json) : null;

  useDocumentMeta(page.data?.title || "Studio", page.data?.meta_description || "");

  if (page.loading || pieces.loading) return <LoadingState />;
  if (page.error) return <ErrorState message={page.error.message} />;
  if (pieces.error) return <ErrorState message={pieces.error.message} />;
  if (!body) return <EmptyState message="Studio page content has not been published yet." />;

  const list = pieces.data ?? [];

  return (
    <article>
      {body.eyebrow ? (
        <p className="font-mono text-xs tracking-[0.25em] text-ink-600 uppercase dark:text-ink-400">
          {body.eyebrow}
        </p>
      ) : null}
      {body.headline ? (
        <h1 className="mt-5 font-display text-4xl leading-tight font-semibold">{body.headline}</h1>
      ) : null}
      {body.intro ? (
        <p className="mt-6 max-w-xl text-base leading-relaxed text-ink-600 dark:text-ink-300">
          {body.intro}
        </p>
      ) : null}
      {body.tools_line ? (
        <p className="mt-3 max-w-xl font-mono text-xs tracking-wide text-ink-500 dark:text-ink-500">
          {body.tools_line}
        </p>
      ) : null}

      {list.length === 0 ? (
        <p className="mt-14 max-w-lg font-mono text-sm leading-relaxed text-ink-600 dark:text-ink-400">
          {body.empty_message}
        </p>
      ) : (
        <ul className="mt-14 space-y-14">
          {list.map((piece, i) => (
            <li key={piece.id} className={`max-w-lg ${i % 2 === 1 ? "md:ml-8" : ""}`}>
              <StudioPieceRow piece={piece} onOpen={setActive} />
            </li>
          ))}
        </ul>
      )}

      {active?.image_url ? (
        <StudioLightbox piece={active} onClose={() => setActive(null)} />
      ) : null}
    </article>
  );
}

function StudioPieceRow({
  piece,
  onOpen,
}: {
  piece: StudioPiece;
  onOpen: (piece: StudioPiece) => void;
}) {
  const meta = [piece.year, piece.medium].filter(Boolean).join(" · ");
  const canOpen = Boolean(piece.image_url);

  return (
    <div>
      {piece.image_url ? (
        <button
          type="button"
          onClick={() => onOpen(piece)}
          className="group block w-full text-left transition-transform active:translate-y-0.5"
          aria-label={`View larger: ${piece.title}`}
        >
          <img
            src={piece.image_url}
            alt={piece.title}
            className="max-h-64 w-full border border-ink-200 object-cover object-center dark:border-ink-800"
          />
        </button>
      ) : null}
      <h2
        className={`font-display text-2xl font-semibold tracking-tight ${piece.image_url ? "mt-4" : ""}`}
      >
        {canOpen ? (
          <button
            type="button"
            onClick={() => onOpen(piece)}
            className="text-left transition-colors hover:text-ember-600 active:translate-y-0.5 dark:hover:text-ember-400"
          >
            {piece.title}
          </button>
        ) : (
          piece.title
        )}
      </h2>
      {meta ? (
        <p className="mt-1.5 font-mono text-xs tracking-wide text-ink-600 dark:text-ink-400">
          {meta}
        </p>
      ) : null}
      {piece.caption ? (
        <p className="mt-3 text-base leading-relaxed text-ink-600 dark:text-ink-300">
          {piece.caption}
        </p>
      ) : null}
    </div>
  );
}

function StudioLightbox({
  piece,
  onClose,
}: {
  piece: StudioPiece;
  onClose: () => void;
}) {
  const titleId = useId();
  const closeRef = useRef<HTMLButtonElement>(null);
  const image = piece.image_url;

  useEffect(() => {
    const previous = document.activeElement;
    closeRef.current?.focus();

    function onKey(event: KeyboardEvent) {
      if (event.key === "Escape") {
        event.preventDefault();
        onClose();
      }
    }

    document.addEventListener("keydown", onKey);
    const prevOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";

    return () => {
      document.removeEventListener("keydown", onKey);
      document.body.style.overflow = prevOverflow;
      if (previous instanceof HTMLElement) previous.focus();
    };
  }, [onClose]);

  if (!image) return null;

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby={titleId}
      className="fixed inset-0 z-50 flex items-center justify-center bg-ink-950/80 p-5 dark:bg-ink-950/90"
      onClick={onClose}
    >
      <div
        className="relative max-h-[90dvh] w-full max-w-3xl border border-ink-200 bg-paper p-4 dark:border-ink-800 dark:bg-ink-950"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="flex items-start justify-between gap-4">
          <div>
            <h2 id={titleId} className="font-display text-xl font-semibold tracking-tight">
              {piece.title}
            </h2>
            <p className="mt-1 font-mono text-xs tracking-wide text-ink-600 dark:text-ink-400">
              {[piece.year, piece.medium].filter(Boolean).join(" · ")}
            </p>
          </div>
          <button
            ref={closeRef}
            type="button"
            onClick={onClose}
            className="shrink-0 font-mono text-xs tracking-[0.18em] text-ink-600 uppercase transition-[transform,color] hover:text-ink-900 active:translate-y-0.5 dark:text-ink-400 dark:hover:text-paper"
          >
            Close
          </button>
        </div>
        <img src={image} alt={piece.title} className="mt-4 max-h-[70dvh] w-full object-contain" />
      </div>
    </div>
  );
}
