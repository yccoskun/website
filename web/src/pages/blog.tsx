import { Link } from "react-router-dom";

import { EmptyState, ErrorState, LoadingState } from "@/components/states";
import { formatDate } from "@/lib/dates";
import { useDocumentMeta } from "@/lib/meta";
import { useApi } from "@/lib/use-api";
import type { PostSummary } from "@/types/post";

export function BlogPage() {
  const { data, loading, error } = useApi<PostSummary[]>("/api/posts");

  useDocumentMeta("Blog", "Blog of Yusuf Can Coskun — nothing published yet.");

  if (loading) {
    return (
      <article>
        <BlogHeader />
        <div className="mt-14">
          <LoadingState />
        </div>
      </article>
    );
  }

  if (error) {
    return (
      <article>
        <BlogHeader />
        <div className="mt-14">
          <ErrorState message={error.message} />
        </div>
      </article>
    );
  }

  if (!data || data.length === 0) {
    return (
      <article>
        <BlogHeader />
        <div className="mt-14">
          <EmptyState message="Nothing published yet — drafts stay off this list until they are ready." />
        </div>
      </article>
    );
  }

  return (
    <article>
      <BlogHeader />
      <ul className="mt-14 space-y-12">
        {data.map((post, i) => (
          <li key={post.id} className={i % 2 === 1 ? "md:ml-8" : ""}>
            <Link
              to={`/blog/${post.slug}`}
              className="group block transition-transform active:translate-y-0.5"
            >
              <h2 className="font-display text-2xl font-semibold tracking-tight group-hover:text-ember-600 dark:group-hover:text-ember-400">
                {post.title}
              </h2>
              <p className="mt-1.5 font-mono text-xs text-ink-600 dark:text-ink-400">
                {formatDate(post.published_at ?? post.created_at)}
              </p>
              {post.summary ? (
                <p className="mt-3 max-w-lg text-base leading-relaxed text-ink-600 dark:text-ink-300">
                  {post.summary}
                </p>
              ) : null}
            </Link>
          </li>
        ))}
      </ul>
    </article>
  );
}

function BlogHeader() {
  return (
    <>
      <p className="font-mono text-xs tracking-[0.25em] text-ink-600 uppercase dark:text-ink-400">
        Blog
      </p>
      <h1 className="mt-5 font-display text-4xl leading-tight font-semibold">
        Nothing here yet — posts land through the admin panel.
      </h1>
      <p className="mt-6 max-w-xl text-base leading-relaxed text-ink-600 dark:text-ink-300">
        This page fills itself once the first post is published. Until then it stays honestly
        empty.
      </p>
    </>
  );
}
