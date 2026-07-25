import { useParams } from "react-router-dom";

import { EmptyState, ErrorState, LoadingState } from "@/components/states";
import { formatDate } from "@/lib/dates";
import { useDocumentMeta } from "@/lib/meta";
import { useApi } from "@/lib/use-api";
import { NotFoundView } from "@/pages/not-found";
import type { Post } from "@/types/post";

export function PostPage() {
  const { slug } = useParams<{ slug: string }>();
  const path = slug ? `/api/posts/${encodeURIComponent(slug)}` : null;
  const { data, loading, error } = useApi<Post>(path);

  const missing = !slug || error?.status === 404;

  useDocumentMeta(
    missing ? "Not found" : (data?.title ?? "Post"),
    missing
      ? "No page at this address on yusufcancoskun.com."
      : data?.summary || "Field notes from systems that had to work.",
  );

  if (missing && !loading) return <NotFoundView />;

  if (loading) {
    return (
      <article>
        <LoadingState />
      </article>
    );
  }

  if (error) {
    return (
      <article>
        <ErrorState message={error.message} />
      </article>
    );
  }

  if (!data) {
    return (
      <article>
        <EmptyState message="Post unavailable." />
      </article>
    );
  }

  const dateLabel = formatDate(data.published_at ?? data.created_at);

  return (
    <article>
      <p className="font-mono text-xs tracking-[0.25em] text-ink-600 uppercase dark:text-ink-400">
        Blog
      </p>
      <h1 className="mt-5 font-display text-4xl leading-tight font-semibold">{data.title}</h1>
      {dateLabel ? (
        <p className="mt-3 font-mono text-xs text-ink-600 dark:text-ink-400">{dateLabel}</p>
      ) : null}
      <div
        className="prose-site mt-10"
        dangerouslySetInnerHTML={{ __html: data.content_html }}
      />
    </article>
  );
}
