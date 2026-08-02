import { useCallback, useEffect, useState } from "react";
import type { FormEvent } from "react";
import { Link, useLocation, useNavigate, useParams } from "react-router";

import { ErrorState, LoadingState } from "@/components/states";
import { ApiError, apiDelete, apiGet, apiPost, apiPut } from "@/lib/api";
import { useDebouncedValue } from "@/lib/debounce";
import { useDocumentMeta } from "@/lib/meta";
import { isValidPostSlug, slugify } from "@/lib/slug";
import { handleAdminUnauthorized, useAdminSession } from "@/pages/admin/session";
import type { OkResponse, PreviewResponse } from "@/types/admin";
import type { Post } from "@/types/post";

interface PostForm {
  title: string;
  slug: string;
  summary: string;
  content_md: string;
  published: boolean;
}

interface CreateNavState {
  createdPost?: Post;
}

const emptyForm: PostForm = {
  title: "",
  slug: "",
  summary: "",
  content_md: "",
  published: false,
};

function formFromPost(post: Post): PostForm {
  return {
    title: post.title,
    slug: post.slug,
    summary: post.summary,
    content_md: post.content_md,
    published: post.published,
  };
}

function formsEqual(a: PostForm, b: PostForm): boolean {
  return (
    a.title === b.title &&
    a.slug === b.slug &&
    a.summary === b.summary &&
    a.content_md === b.content_md &&
    a.published === b.published
  );
}

function createdSeed(
  isNew: boolean,
  postId: number | null,
  state: CreateNavState | null,
): Post | null {
  if (isNew || postId === null) return null;
  const created = state?.createdPost;
  if (!created || created.id !== postId) return null;
  return created;
}

const fieldLabel =
  "font-mono text-[0.65rem] tracking-[0.16em] text-ink-600 uppercase dark:text-ink-400";
const fieldInput =
  "mt-1 w-full rounded-chip border border-ink-200 bg-paper px-2 py-1.5 text-sm text-ink-900 outline-none focus:border-ember-600 dark:border-ink-800 dark:bg-ink-950 dark:text-ink-200 dark:focus:border-ember-400";
const fieldMono = `${fieldInput} font-mono text-xs`;

export function AdminPostEditorPage() {
  const { id: idParam } = useParams<{ id: string }>();
  const isNew = idParam === undefined;
  const postId = isNew ? null : Number(idParam);
  const invalidId = !isNew && (!Number.isFinite(postId) || (postId ?? 0) <= 0);

  const navigate = useNavigate();
  const location = useLocation();
  const { onUnauthorized } = useAdminSession();

  const seed = createdSeed(isNew, postId, location.state as CreateNavState | null);
  const seedForm = seed ? formFromPost(seed) : null;

  const [form, setForm] = useState<PostForm>(seedForm ?? emptyForm);
  const [saved, setSaved] = useState<PostForm>(seedForm ?? emptyForm);
  const [slugPristine, setSlugPristine] = useState(seedForm === null);
  const [loading, setLoading] = useState(!isNew && seedForm === null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [previewHtml, setPreviewHtml] = useState("");
  const [previewError, setPreviewError] = useState<string | null>(null);

  const debouncedMd = useDebouncedValue(form.content_md, 300);

  useDocumentMeta(
    isNew ? "Admin · New post" : "Admin · Edit post",
    "Write and preview blog markdown.",
  );

  useEffect(() => {
    if (isNew || invalidId || postId === null) {
      setLoading(false);
      return;
    }
    // Seeded by create→navigate; skip cold load to avoid a full-page flash.
    const fromCreate = createdSeed(isNew, postId, location.state as CreateNavState | null);
    if (fromCreate) {
      const next = formFromPost(fromCreate);
      setForm(next);
      setSaved(next);
      setSlugPristine(false);
      setLoading(false);
      return;
    }
    let cancelled = false;
    setLoading(true);
    setLoadError(null);
    void apiGet<Post>(`/api/admin/posts/${postId}`)
      .then((post) => {
        if (cancelled) return;
        const next = formFromPost(post);
        setForm(next);
        setSaved(next);
        setSlugPristine(false);
        setLoading(false);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        if (handleAdminUnauthorized(err, onUnauthorized)) return;
        setLoadError(err instanceof ApiError ? err.message : "Failed to load post");
        setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [isNew, invalidId, postId, location.state, onUnauthorized]);

  useEffect(() => {
    if (debouncedMd === "") {
      setPreviewHtml("");
      setPreviewError(null);
      return;
    }
    let cancelled = false;
    void apiPost<PreviewResponse>("/api/admin/preview", { content_md: debouncedMd })
      .then((res) => {
        if (cancelled) return;
        setPreviewError(null);
        setPreviewHtml(res.html);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        if (handleAdminUnauthorized(err, onUnauthorized)) return;
        setPreviewError(err instanceof ApiError ? err.message : "Preview failed");
      });
    return () => {
      cancelled = true;
    };
  }, [debouncedMd, onUnauthorized]);

  const setTitle = useCallback((title: string) => {
    setForm((prev) => {
      if (!slugPristine) return { ...prev, title };
      return { ...prev, title, slug: slugify(title) };
    });
  }, [slugPristine]);

  async function onSave(e: FormEvent) {
    e.preventDefault();
    setSaveError(null);
    const slug = form.slug.trim();
    if (!isValidPostSlug(slug)) {
      setSaveError(
        "Slug must be lowercase letters, digits, and single hyphens (max 100).",
      );
      return;
    }
    setSaving(true);
    const body = {
      title: form.title,
      slug,
      summary: form.summary,
      content_md: form.content_md,
      published: form.published,
    };
    try {
      if (isNew) {
        const created = await apiPost<Post>("/api/admin/posts", body);
        const next = formFromPost(created);
        setSaved(next);
        setForm(next);
        setSlugPristine(false);
        navigate(`/admin/posts/${created.id}`, {
          replace: true,
          state: { createdPost: created } satisfies CreateNavState,
        });
        return;
      }
      if (postId === null) return;
      const updated = await apiPut<Post>(`/api/admin/posts/${postId}`, body);
      const next = formFromPost(updated);
      setForm(next);
      setSaved(next);
    } catch (err: unknown) {
      if (handleAdminUnauthorized(err, onUnauthorized)) return;
      setSaveError(err instanceof ApiError ? err.message : "Save failed");
    } finally {
      setSaving(false);
    }
  }

  async function onDelete() {
    if (postId === null) return;
    setDeleting(true);
    try {
      await apiDelete<OkResponse>(`/api/admin/posts/${postId}`);
      navigate("/admin/posts", { replace: true });
    } catch (err: unknown) {
      if (handleAdminUnauthorized(err, onUnauthorized)) return;
      setSaveError(err instanceof ApiError ? err.message : "Delete failed");
      setConfirmDelete(false);
    } finally {
      setDeleting(false);
    }
  }

  if (invalidId) {
    return <ErrorState message="Invalid post id" />;
  }

  if (loading) {
    return <LoadingState />;
  }

  if (loadError) {
    return <ErrorState message={loadError} />;
  }

  const dirty = !formsEqual(form, saved);

  return (
    <section>
      <div className="flex flex-wrap items-baseline justify-between gap-3">
        <div className="flex items-baseline gap-3">
          <h1 className="font-display text-2xl font-semibold tracking-tight">
            {isNew ? "New post" : "Edit post"}
          </h1>
          {dirty ? (
            <span className="font-mono text-[0.65rem] tracking-wide text-ember-600 uppercase dark:text-ember-400">
              Unsaved
            </span>
          ) : (
            <span className="font-mono text-[0.65rem] tracking-wide text-ink-600 uppercase dark:text-ink-400">
              Saved
            </span>
          )}
        </div>
        <Link
          to="/admin/posts"
          className="font-mono text-[0.65rem] tracking-wide text-ink-600 uppercase transition-[transform,color] hover:text-ink-900 active:translate-y-0.5 dark:text-ink-400 dark:hover:text-paper"
        >
          ← Posts
        </Link>
      </div>

      {saveError ? (
        <div className="mt-3">
          <ErrorState message={saveError} />
        </div>
      ) : null}

      <form onSubmit={onSave} className="mt-4 grid gap-5 lg:grid-cols-2">
        <div className="space-y-3">
          <label className="block">
            <span className={fieldLabel}>Title</span>
            <input
              type="text"
              required
              value={form.title}
              onChange={(e) => setTitle(e.target.value)}
              className={fieldInput}
            />
          </label>
          <label className="block">
            <span className={fieldLabel}>Slug</span>
            <input
              type="text"
              required
              value={form.slug}
              onChange={(e) => {
                setSlugPristine(false);
                setForm((prev) => ({ ...prev, slug: e.target.value }));
              }}
              className={fieldMono}
            />
          </label>
          <label className="block">
            <span className={fieldLabel}>Summary</span>
            <textarea
              rows={2}
              value={form.summary}
              onChange={(e) => setForm((prev) => ({ ...prev, summary: e.target.value }))}
              className={fieldInput}
            />
          </label>
          <label className="block">
            <span className={fieldLabel}>Markdown</span>
            <textarea
              rows={18}
              value={form.content_md}
              onChange={(e) => setForm((prev) => ({ ...prev, content_md: e.target.value }))}
              className={`${fieldMono} min-h-64 resize-y`}
            />
          </label>

          <div className="flex flex-wrap items-center gap-3 pt-1">
            <button
              type="button"
              role="switch"
              aria-checked={form.published}
              onClick={() =>
                setForm((prev) => ({ ...prev, published: !prev.published }))
              }
              className="rounded-chip border border-ink-200 px-2.5 py-1 font-mono text-[0.65rem] tracking-[0.14em] uppercase transition-transform active:translate-y-0.5 dark:border-ink-800"
            >
              {form.published ? (
                <span className="text-ember-600 dark:text-ember-400">Published</span>
              ) : (
                <span className="text-ink-600 dark:text-ink-400">Draft</span>
              )}
            </button>
            <button
              type="submit"
              disabled={saving}
              className="rounded-chip border border-ink-900 bg-ink-900 px-2.5 py-1 font-mono text-[0.65rem] tracking-[0.14em] text-paper uppercase transition-transform enabled:active:translate-y-0.5 disabled:opacity-50 dark:border-paper dark:bg-paper dark:text-ink-950"
            >
              {saving ? "Saving…" : "Save"}
            </button>
            {!isNew ? (
              confirmDelete ? (
                <span className="inline-flex items-center gap-2">
                  <button
                    type="button"
                    disabled={deleting}
                    onClick={() => void onDelete()}
                    className="font-mono text-[0.65rem] tracking-wide text-ember-600 uppercase transition-transform enabled:active:translate-y-0.5 dark:text-ember-400"
                  >
                    Confirm delete
                  </button>
                  <button
                    type="button"
                    disabled={deleting}
                    onClick={() => setConfirmDelete(false)}
                    className="font-mono text-[0.65rem] tracking-wide text-ink-600 uppercase transition-transform enabled:active:translate-y-0.5 dark:text-ink-400"
                  >
                    Cancel
                  </button>
                </span>
              ) : (
                <button
                  type="button"
                  onClick={() => setConfirmDelete(true)}
                  className="font-mono text-[0.65rem] tracking-wide text-ink-600 uppercase transition-[transform,color] hover:text-ember-600 active:translate-y-0.5 dark:text-ink-400 dark:hover:text-ember-400"
                >
                  Delete
                </button>
              )
            ) : null}
          </div>
        </div>

        <div className="rounded-card border border-ink-200 p-3 dark:border-ink-800">
          <p className={`${fieldLabel} mb-3`}>Preview</p>
          {previewError ? (
            <p className="mb-2 font-mono text-xs text-ember-600 dark:text-ember-400">
              {previewError}
            </p>
          ) : null}
          {previewHtml ? (
            <div className="prose-site" dangerouslySetInnerHTML={{ __html: previewHtml }} />
          ) : (
            <p className="font-mono text-xs text-ink-600 dark:text-ink-400">Nothing to preview.</p>
          )}
        </div>
      </form>
    </section>
  );
}
