import { Link } from "react-router";

import { EmptyState, ErrorState, LoadingState } from "@/components/states";
import { formatDate } from "@/lib/dates";
import { useDocumentMeta } from "@/lib/meta";
import { useApi } from "@/lib/use-api";
import type { NotesPageBody, Page } from "@/types/cms";
import type { PostSummary } from "@/types/post";

function parseNotesBody(raw: string): NotesPageBody {
  try {
    const parsed = JSON.parse(raw) as NotesPageBody;
    return {
      eyebrow: parsed.eyebrow ?? "Notes",
      headline: parsed.headline ?? "",
      intro: parsed.intro ?? "",
      empty_message: parsed.empty_message || "Nothing published yet.",
    };
  } catch {
    return {
      eyebrow: "Notes",
      headline: "",
      intro: "",
      empty_message: "Nothing published yet.",
    };
  }
}

export function BlogPage() {
  const page = useApi<Page>("/api/pages/notes");
  const posts = useApi<PostSummary[]>("/api/posts");

  const body = page.data ? parseNotesBody(page.data.body_json) : null;

  useDocumentMeta(page.data?.title || "Notes", page.data?.meta_description || "");

  if (page.loading || posts.loading) {
    return (
      <article>
        <NotesHeader body={body} />
        <div className="mt-14">
          <LoadingState />
        </div>
      </article>
    );
  }

  if (page.error) {
    return <ErrorState message={page.error.message} />;
  }

  if (posts.error) {
    return (
      <article>
        <NotesHeader body={body} />
        <div className="mt-14">
          <ErrorState message={posts.error.message} />
        </div>
      </article>
    );
  }

  if (!posts.data || posts.data.length === 0) {
    return (
      <article>
        <NotesHeader body={body} />
        <div className="mt-14">
          <EmptyState message={body?.empty_message || "Nothing published yet."} />
        </div>
      </article>
    );
  }

  return (
    <article>
      <NotesHeader body={body} />
      <ul className="mt-14 space-y-12">
        {posts.data.map((post, i) => (
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

function NotesHeader({ body }: { body: NotesPageBody | null }) {
  if (!body) return null;
  return (
    <>
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
    </>
  );
}
