import { useCallback, useEffect, useState } from "react";
import type { FormEvent } from "react";

import { EmptyState, ErrorState, LoadingState } from "@/components/states";
import { ApiError, apiDelete, apiGet, apiPost, apiPut } from "@/lib/api";
import { useDocumentMeta } from "@/lib/meta";
import { handleAdminUnauthorized, useAdminSession } from "@/pages/admin/session";
import type { OkResponse } from "@/types/admin";
import type { WorkItem } from "@/types/cms";

const fieldLabel =
  "font-mono text-[0.65rem] tracking-[0.16em] text-ink-600 uppercase dark:text-ink-400";
const fieldInput =
  "mt-1 w-full rounded-chip border border-ink-200 bg-paper px-2 py-1 text-sm text-ink-900 outline-none focus:border-ember-600 dark:border-ink-800 dark:bg-ink-950 dark:text-ink-200 dark:focus:border-ember-400";

interface WorkForm {
  name: string;
  one_liner: string;
  body: string;
  stack: string;
  status: string;
  href: string;
  sort_order: number;
}

const emptyForm = (): WorkForm => ({
  name: "",
  one_liner: "",
  body: "",
  stack: "",
  status: "",
  href: "",
  sort_order: 0,
});

export function AdminWorkPage() {
  const { onUnauthorized } = useAdminSession();
  const [items, setItems] = useState<WorkItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [editId, setEditId] = useState<number | null>(null);
  const [form, setForm] = useState<WorkForm>(emptyForm());
  const [saving, setSaving] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);

  useDocumentMeta("Admin · Work", "Edit work portfolio items.");

  const load = useCallback(() => {
    setLoading(true);
    void apiGet<WorkItem[]>("/api/admin/work")
      .then((data) => {
        setItems(data);
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

  function openEdit(item: WorkItem) {
    setEditId(item.id);
    setForm({
      name: item.name,
      one_liner: item.one_liner,
      body: item.body,
      stack: item.stack.join(", "),
      status: item.status,
      href: item.href,
      sort_order: item.sort_order,
    });
    setFormError(null);
  }

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    setSaving(true);
    setFormError(null);
    const payload = {
      name: form.name,
      one_liner: form.one_liner,
      body: form.body,
      stack: form.stack
        .split(",")
        .map((s) => s.trim())
        .filter(Boolean),
      status: form.status,
      href: form.href,
      sort_order: form.sort_order,
    };
    const req =
      editId === null
        ? apiPost<WorkItem>("/api/admin/work", payload)
        : apiPut<WorkItem>(`/api/admin/work/${editId}`, payload);
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
    if (!window.confirm("Delete this work item?")) return;
    void apiDelete<OkResponse>(`/api/admin/work/${id}`)
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
          <h1 className="font-display text-2xl font-semibold">Work</h1>
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
            <EmptyState message="No work items yet." />
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
                  {item.name}
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
        {(
          [
            ["name", form.name],
            ["one_liner", form.one_liner],
            ["status", form.status],
            ["href", form.href],
            ["stack", form.stack],
          ] as const
        ).map(([key, val]) => (
          <label key={key} className="block">
            <span className={fieldLabel}>{key}</span>
            <input
              className={fieldInput}
              value={val}
              onChange={(e) => setForm((f) => ({ ...f, [key]: e.target.value }))}
            />
          </label>
        ))}
        <label className="block">
          <span className={fieldLabel}>body</span>
          <textarea
            className={`${fieldInput} min-h-28`}
            value={form.body}
            onChange={(e) => setForm((f) => ({ ...f, body: e.target.value }))}
          />
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
