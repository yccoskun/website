import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router";

import { EmptyState, ErrorState, LoadingState } from "@/components/states";
import { ApiError, apiDelete, apiGet } from "@/lib/api";
import { formatDate } from "@/lib/dates";
import { useDocumentMeta } from "@/lib/meta";
import { handleAdminUnauthorized, useAdminSession } from "@/pages/admin/session";
import type { OkResponse } from "@/types/admin";
import type { AdminPostSummary } from "@/types/post";

function StatusChip({ published }: { published: boolean }) {
  if (published) {
    return (
      <span className="rounded-chip bg-ember-600/15 px-1.5 py-0.5 font-mono text-[0.65rem] tracking-wide text-ember-600 uppercase dark:text-ember-400">
        published
      </span>
    );
  }
  return (
    <span className="rounded-chip border border-ink-200 px-1.5 py-0.5 font-mono text-[0.65rem] tracking-wide text-ink-600 uppercase dark:border-ink-800 dark:text-ink-400">
      draft
    </span>
  );
}

function DeleteConfirm({
  busy,
  onConfirm,
  onCancel,
}: {
  busy: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}) {
  return (
    <span className="inline-flex items-center gap-1.5">
      <button
        type="button"
        disabled={busy}
        onClick={onConfirm}
        className="font-mono text-[0.65rem] tracking-wide text-ember-600 uppercase transition-transform enabled:active:translate-y-0.5 dark:text-ember-400"
      >
        Confirm
      </button>
      <button
        type="button"
        disabled={busy}
        onClick={onCancel}
        className="font-mono text-[0.65rem] tracking-wide text-ink-600 uppercase transition-transform enabled:active:translate-y-0.5 dark:text-ink-400"
      >
        Cancel
      </button>
    </span>
  );
}

export function AdminPostsPage() {
  const { onUnauthorized } = useAdminSession();
  const [posts, setPosts] = useState<AdminPostSummary[] | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [confirmId, setConfirmId] = useState<number | null>(null);
  const [deleting, setDeleting] = useState(false);

  useDocumentMeta("Admin · Posts", "Manage blog drafts and published posts.");

  const load = useCallback(() => {
    setLoading(true);
    setError(null);
    void apiGet<AdminPostSummary[]>("/api/admin/posts")
      .then((data) => {
        setPosts(data);
        setLoading(false);
      })
      .catch((err: unknown) => {
        if (handleAdminUnauthorized(err, onUnauthorized)) return;
        setPosts(null);
        setError(err instanceof ApiError ? err.message : "Failed to load posts");
        setLoading(false);
      });
  }, [onUnauthorized]);

  useEffect(() => {
    load();
  }, [load]);

  async function deletePost(id: number) {
    setDeleting(true);
    try {
      await apiDelete<OkResponse>(`/api/admin/posts/${id}`);
      setConfirmId(null);
      load();
    } catch (err: unknown) {
      if (handleAdminUnauthorized(err, onUnauthorized)) return;
      setError(err instanceof ApiError ? err.message : "Delete failed");
    } finally {
      setDeleting(false);
    }
  }

  if (loading) {
    return (
      <section>
        <PostsHeader />
        <div className="mt-4">
          <LoadingState />
        </div>
      </section>
    );
  }

  if (error && !posts) {
    return (
      <section>
        <PostsHeader />
        <div className="mt-4">
          <ErrorState message={error} />
        </div>
      </section>
    );
  }

  const rows = posts ?? [];

  return (
    <section>
      <PostsHeader />
      {error ? (
        <div className="mt-3">
          <ErrorState message={error} />
        </div>
      ) : null}
      {rows.length === 0 ? (
        <div className="mt-6">
          <EmptyState message="No posts yet" />
        </div>
      ) : (
        <div className="mt-4 overflow-x-auto">
          <table className="w-full min-w-[36rem] border-collapse text-left text-sm">
            <thead>
              <tr className="border-b border-ink-200 font-mono text-[0.65rem] tracking-[0.14em] text-ink-600 uppercase dark:border-ink-800 dark:text-ink-400">
                <th className="py-2 pr-3 font-medium">Title</th>
                <th className="py-2 pr-3 font-medium">Slug</th>
                <th className="py-2 pr-3 font-medium">Status</th>
                <th className="py-2 pr-3 font-medium">Updated</th>
                <th className="py-2 font-medium">Actions</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((post) => (
                <tr
                  key={post.id}
                  className="border-b border-ink-200/80 dark:border-ink-800/80"
                >
                  <td className="py-2 pr-3 font-medium text-ink-900 dark:text-ink-200">
                    {post.title}
                  </td>
                  <td className="py-2 pr-3 font-mono text-xs text-ink-600 dark:text-ink-400">
                    {post.slug}
                  </td>
                  <td className="py-2 pr-3">
                    <StatusChip published={post.published} />
                  </td>
                  <td className="py-2 pr-3 font-mono text-xs text-ink-600 dark:text-ink-400">
                    {formatDate(post.updated_at)}
                  </td>
                  <td className="py-2">
                    <span className="inline-flex items-center gap-2">
                      <Link
                        to={`/admin/posts/${post.id}`}
                        className="font-mono text-[0.65rem] tracking-wide text-ink-600 uppercase transition-[transform,color] hover:text-ink-900 active:translate-y-0.5 dark:text-ink-400 dark:hover:text-paper"
                      >
                        Edit
                      </Link>
                      {confirmId === post.id ? (
                        <DeleteConfirm
                          busy={deleting}
                          onConfirm={() => void deletePost(post.id)}
                          onCancel={() => setConfirmId(null)}
                        />
                      ) : (
                        <button
                          type="button"
                          onClick={() => setConfirmId(post.id)}
                          className="font-mono text-[0.65rem] tracking-wide text-ink-600 uppercase transition-[transform,color] hover:text-ember-600 active:translate-y-0.5 dark:text-ink-400 dark:hover:text-ember-400"
                        >
                          Delete
                        </button>
                      )}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}

function PostsHeader() {
  return (
    <div className="flex items-baseline justify-between gap-4">
      <h1 className="font-display text-2xl font-semibold tracking-tight">Posts</h1>
      <Link
        to="/admin/posts/new"
        className="rounded-chip border border-ink-900 bg-ink-900 px-2.5 py-1 font-mono text-[0.65rem] tracking-[0.14em] text-paper uppercase transition-transform active:translate-y-0.5 dark:border-paper dark:bg-paper dark:text-ink-950"
      >
        New post
      </Link>
    </div>
  );
}
