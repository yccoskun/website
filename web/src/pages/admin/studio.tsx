import { useCallback, useEffect, useState } from "react";
import type { FormEvent } from "react";

import { EmptyState, ErrorState, LoadingState } from "@/components/states";
import { ApiError, apiDelete, apiGet, apiPost, apiPut } from "@/lib/api";
import { useDocumentMeta } from "@/lib/meta";
import { handleAdminUnauthorized, useAdminSession } from "@/pages/admin/session";
import type { OkResponse } from "@/types/admin";
import type { MediaAsset, StudioPiece } from "@/types/cms";

const fieldLabel =
  "font-mono text-[0.65rem] tracking-[0.16em] text-ink-600 uppercase dark:text-ink-400";
const fieldInput =
  "mt-1 w-full rounded-chip border border-ink-200 bg-paper px-2 py-1 text-sm text-ink-900 outline-none focus:border-ember-600 dark:border-ink-800 dark:bg-ink-950 dark:text-ink-200 dark:focus:border-ember-400";

interface StudioForm {
  slug: string;
  title: string;
  year: string;
  medium: string;
  caption: string;
  image_media_id: string;
  sort_order: number;
  published: boolean;
}

const emptyForm = (): StudioForm => ({
  slug: "",
  title: "",
  year: "",
  medium: "",
  caption: "",
  image_media_id: "",
  sort_order: 0,
  published: false,
});

export function AdminStudioPage() {
  const { onUnauthorized } = useAdminSession();
  const [items, setItems] = useState<StudioPiece[]>([]);
  const [media, setMedia] = useState<MediaAsset[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [editId, setEditId] = useState<number | null>(null);
  const [form, setForm] = useState<StudioForm>(emptyForm());
  const [saving, setSaving] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);

  useDocumentMeta("Admin · Studio", "Edit studio catalogue pieces.");

  const load = useCallback(() => {
    setLoading(true);
    void Promise.all([
      apiGet<StudioPiece[]>("/api/admin/studio"),
      apiGet<MediaAsset[]>("/api/admin/media"),
    ])
      .then(([pieces, assets]) => {
        setItems(pieces);
        setMedia(assets);
        setLoading(false);
      })
      .catch((err: unknown) => {
        if (handleAdminUnauthorized(err, onUnauthorized)) return;
        setError(err instanceof ApiError ? err.message : "Failed to load");
        setLoading(false);
      });
  }, [onUnauthorized]);

  useEffect(() => {
    load();
  }, [load]);

  function openCreate() {
    setEditId(null);
    setForm(emptyForm());
    setFormError(null);
  }

  function openEdit(item: StudioPiece) {
    setEditId(item.id);
    setForm({
      slug: item.slug,
      title: item.title,
      year: item.year,
      medium: item.medium,
      caption: item.caption,
      image_media_id: item.image_media_id?.toString() ?? "",
      sort_order: item.sort_order,
      published: item.published,
    });
    setFormError(null);
  }

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    setSaving(true);
    setFormError(null);
    const mediaId = form.image_media_id.trim() === "" ? null : Number(form.image_media_id);
    const payload = {
      slug: form.slug,
      title: form.title,
      year: form.year,
      medium: form.medium,
      caption: form.caption,
      image_media_id: mediaId,
      sort_order: form.sort_order,
      published: form.published,
    };
    const req =
      editId === null
        ? apiPost<StudioPiece>("/api/admin/studio", payload)
        : apiPut<StudioPiece>(`/api/admin/studio/${editId}`, payload);
    void req
      .then(() => {
        setSaving(false);
        openCreate();
        load();
      })
      .catch((err: unknown) => {
        if (handleAdminUnauthorized(err, onUnauthorized)) return;
        setFormError(err instanceof ApiError ? err.message : "Save failed");
        setSaving(false);
      });
  }

  function onDelete(id: number) {
    if (!window.confirm("Delete this studio piece?")) return;
    void apiDelete<OkResponse>(`/api/admin/studio/${id}`)
      .then(() => load())
      .catch((err: unknown) => {
        if (handleAdminUnauthorized(err, onUnauthorized)) return;
        setError(err instanceof ApiError ? err.message : "Delete failed");
      });
  }

  if (loading) return <LoadingState />;
  if (error) return <ErrorState message={error} />;

  return (
    <div className="grid max-w-5xl gap-8 lg:grid-cols-2">
      <div>
        <div className="flex items-baseline justify-between gap-3">
          <h1 className="font-display text-2xl font-semibold">Studio</h1>
          <button
            type="button"
            onClick={openCreate}
            className="font-mono text-[0.65rem] tracking-[0.14em] text-ink-600 uppercase dark:text-ink-400"
          >
            New
          </button>
        </div>
        {items.length === 0 ? (
          <div className="mt-6">
            <EmptyState message="No studio pieces yet." />
          </div>
        ) : (
          <ul className="mt-4 space-y-2">
            {items.map((item) => (
              <li
                key={item.id}
                className="flex items-center justify-between gap-2 border-b border-ink-200 py-2 dark:border-ink-800"
              >
                <button
                  type="button"
                  onClick={() => openEdit(item)}
                  className="text-left font-mono text-sm hover:text-ember-600 dark:hover:text-ember-400"
                >
                  {item.title}
                  {!item.published ? " (draft)" : ""}
                </button>
                <button
                  type="button"
                  onClick={() => onDelete(item.id)}
                  className="font-mono text-[0.65rem] tracking-[0.12em] text-ink-500 uppercase"
                >
                  Delete
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
      <form onSubmit={onSubmit} className="space-y-3">
        <h2 className="font-mono text-xs tracking-[0.16em] text-ink-600 uppercase dark:text-ink-400">
          {editId === null ? "Create" : `Edit #${editId}`}
        </h2>
        {(["slug", "title", "year", "medium"] as const).map((key) => (
          <label key={key} className="block">
            <span className={fieldLabel}>{key}</span>
            <input
              className={fieldInput}
              value={form[key]}
              onChange={(e) => setForm((f) => ({ ...f, [key]: e.target.value }))}
            />
          </label>
        ))}
        <label className="block">
          <span className={fieldLabel}>caption</span>
          <textarea
            className={`${fieldInput} min-h-20`}
            value={form.caption}
            onChange={(e) => setForm((f) => ({ ...f, caption: e.target.value }))}
          />
        </label>
        <label className="block">
          <span className={fieldLabel}>image_media_id</span>
          <select
            className={fieldInput}
            value={form.image_media_id}
            onChange={(e) => setForm((f) => ({ ...f, image_media_id: e.target.value }))}
          >
            <option value="">None</option>
            {media.map((m) => (
              <option key={m.id} value={m.id}>
                #{m.id} {m.original_name}
              </option>
            ))}
          </select>
        </label>
        <label className="block">
          <span className={fieldLabel}>sort_order</span>
          <input
            type="number"
            className={fieldInput}
            value={form.sort_order}
            onChange={(e) => setForm((f) => ({ ...f, sort_order: Number(e.target.value) }))}
          />
        </label>
        <label className="flex items-center gap-2 font-mono text-xs">
          <input
            type="checkbox"
            checked={form.published}
            onChange={(e) => setForm((f) => ({ ...f, published: e.target.checked }))}
          />
          Published
        </label>
        {formError ? <p className="font-mono text-xs text-ember-600">{formError}</p> : null}
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
