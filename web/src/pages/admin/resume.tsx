import { useCallback, useEffect, useMemo, useState } from "react";
import type { FormEvent } from "react";

import { EmptyState, ErrorState, LoadingState } from "@/components/states";
import { ApiError, apiDelete, apiGet, apiPost, apiPut } from "@/lib/api";
import { useDebouncedValue } from "@/lib/debounce";
import { useDocumentMeta } from "@/lib/meta";
import { handleAdminUnauthorized, useAdminSession } from "@/pages/admin/session";
import type { OkResponse, PreviewResponse } from "@/types/admin";
import type { ResumeEntry, ResumeSection, ResumeSectionKind } from "@/types/resume";

interface EntryForm {
  section_id: number;
  org: string;
  role: string;
  location: string;
  period: string;
  body_md: string;
  tech: string;
  sort_order: number;
}

function emptyForm(sectionId: number): EntryForm {
  return {
    section_id: sectionId,
    org: "",
    role: "",
    location: "",
    period: "",
    body_md: "",
    tech: "",
    sort_order: 0,
  };
}

function formFromEntry(entry: ResumeEntry): EntryForm {
  return {
    section_id: entry.section_id,
    org: entry.org,
    role: entry.role,
    location: entry.location,
    period: entry.period,
    body_md: entry.body_md,
    tech: entry.tech,
    sort_order: entry.sort_order,
  };
}

const fieldLabel =
  "font-mono text-[0.65rem] tracking-[0.16em] text-ink-600 uppercase dark:text-ink-400";
const fieldInput =
  "mt-1 w-full rounded-chip border border-ink-200 bg-paper px-2 py-1 text-sm text-ink-900 outline-none focus:border-ember-600 dark:border-ink-800 dark:bg-ink-950 dark:text-ink-200 dark:focus:border-ember-400";

export function AdminResumePage() {
  const { onUnauthorized } = useAdminSession();
  const [sections, setSections] = useState<ResumeSection[]>([]);
  const [entries, setEntries] = useState<ResumeEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [panelMode, setPanelMode] = useState<"closed" | "create" | "edit">("closed");
  const [editId, setEditId] = useState<number | null>(null);
  const [form, setForm] = useState<EntryForm>(emptyForm(0));
  const [saving, setSaving] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [confirmId, setConfirmId] = useState<number | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [previewHtml, setPreviewHtml] = useState("");
  const [previewError, setPreviewError] = useState<string | null>(null);

  const debouncedBody = useDebouncedValue(form.body_md, 300);

  useDocumentMeta("Admin · Resume", "Edit resume sections and entries.");

  const load = useCallback(() => {
    setLoading(true);
    setError(null);
    void Promise.all([
      apiGet<ResumeEntry[]>("/api/admin/resume/entries"),
      apiGet<ResumeSection[]>("/api/admin/resume/sections"),
    ])
      .then(([entryList, sectionList]) => {
        setEntries(entryList);
        setSections(sectionList);
        setLoading(false);
      })
      .catch((err: unknown) => {
        if (handleAdminUnauthorized(err, onUnauthorized)) return;
        setError(err instanceof ApiError ? err.message : "Failed to load resume");
        setLoading(false);
      });
  }, [onUnauthorized]);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    if (panelMode === "closed") return;
    if (debouncedBody === "") {
      setPreviewHtml("");
      setPreviewError(null);
      return;
    }
    let cancelled = false;
    void apiPost<PreviewResponse>("/api/admin/preview", { content_md: debouncedBody })
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
  }, [debouncedBody, panelMode, onUnauthorized]);

  const grouped = useMemo(() => {
    const bySection = new Map<number, ResumeEntry[]>();
    for (const entry of entries) {
      const list = bySection.get(entry.section_id) ?? [];
      list.push(entry);
      bySection.set(entry.section_id, list);
    }
    const ordered = [...sections].sort((a, b) => a.sort_order - b.sort_order);
    return ordered.map((section) => ({
      section,
      entries: (bySection.get(section.id) ?? []).sort((a, b) => a.sort_order - b.sort_order),
    }));
  }, [entries, sections]);

  function openCreate() {
    const firstId = sections[0]?.id ?? 0;
    setForm(emptyForm(firstId));
    setEditId(null);
    setFormError(null);
    setPreviewHtml("");
    setPreviewError(null);
    setPanelMode("create");
  }

  function openEdit(entry: ResumeEntry) {
    setForm(formFromEntry(entry));
    setEditId(entry.id);
    setFormError(null);
    setPreviewHtml(entry.body_html);
    setPreviewError(null);
    setPanelMode("edit");
  }

  function closePanel() {
    setPanelMode("closed");
    setEditId(null);
    setFormError(null);
  }

  async function onSave(e: FormEvent) {
    e.preventDefault();
    setFormError(null);
    setSaving(true);
    const body = { ...form };
    try {
      if (panelMode === "create") {
        await apiPost<ResumeEntry>("/api/admin/resume/entries", body);
      } else if (editId !== null) {
        await apiPut<ResumeEntry>(`/api/admin/resume/entries/${editId}`, body);
      }
      closePanel();
      load();
    } catch (err: unknown) {
      if (handleAdminUnauthorized(err, onUnauthorized)) return;
      setFormError(err instanceof ApiError ? err.message : "Save failed");
    } finally {
      setSaving(false);
    }
  }

  async function deleteEntry(id: number) {
    setDeleting(true);
    try {
      await apiDelete<OkResponse>(`/api/admin/resume/entries/${id}`);
      setConfirmId(null);
      if (editId === id) closePanel();
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
        <ResumeHeader onNew={openCreate} disabled />
        <div className="mt-4">
          <LoadingState />
        </div>
      </section>
    );
  }

  if (error && entries.length === 0 && sections.length === 0) {
    return (
      <section>
        <ResumeHeader onNew={openCreate} disabled />
        <div className="mt-4">
          <ErrorState message={error} />
        </div>
      </section>
    );
  }

  return (
    <section>
      <ResumeHeader onNew={openCreate} disabled={sections.length === 0} />
      <SectionManager sections={sections} onChange={load} onUnauthorized={onUnauthorized} />
      {error ? (
        <div className="mt-3">
          <ErrorState message={error} />
        </div>
      ) : null}

      {panelMode !== "closed" ? (
        <form
          onSubmit={onSave}
          className="mt-4 rounded-card border border-ink-200 p-3 dark:border-ink-800"
        >
          <p className="font-mono text-[0.65rem] tracking-[0.16em] text-ink-600 uppercase dark:text-ink-400">
            {panelMode === "create" ? "New entry" : "Edit entry"}
          </p>
          {formError ? (
            <div className="mt-2">
              <ErrorState message={formError} />
            </div>
          ) : null}
          <div className="mt-3 grid gap-3 sm:grid-cols-2">
            <label className="block">
              <span className={fieldLabel}>Section</span>
              <select
                value={form.section_id}
                onChange={(e) =>
                  setForm((prev) => ({ ...prev, section_id: Number(e.target.value) }))
                }
                className={fieldInput}
              >
                {sections.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.title}
                  </option>
                ))}
              </select>
            </label>
            <label className="block">
              <span className={fieldLabel}>Sort order</span>
              <input
                type="number"
                value={form.sort_order}
                onChange={(e) =>
                  setForm((prev) => ({ ...prev, sort_order: Number(e.target.value) || 0 }))
                }
                className={`${fieldInput} font-mono`}
              />
            </label>
            <label className="block">
              <span className={fieldLabel}>Org</span>
              <input
                type="text"
                required
                value={form.org}
                onChange={(e) => setForm((prev) => ({ ...prev, org: e.target.value }))}
                className={fieldInput}
              />
            </label>
            <label className="block">
              <span className={fieldLabel}>Role</span>
              <input
                type="text"
                value={form.role}
                onChange={(e) => setForm((prev) => ({ ...prev, role: e.target.value }))}
                className={fieldInput}
              />
            </label>
            <label className="block">
              <span className={fieldLabel}>Location</span>
              <input
                type="text"
                value={form.location}
                onChange={(e) => setForm((prev) => ({ ...prev, location: e.target.value }))}
                className={fieldInput}
              />
            </label>
            <label className="block">
              <span className={fieldLabel}>Period</span>
              <input
                type="text"
                value={form.period}
                onChange={(e) => setForm((prev) => ({ ...prev, period: e.target.value }))}
                className={`${fieldInput} font-mono text-xs`}
              />
            </label>
            <label className="block sm:col-span-2">
              <span className={fieldLabel}>Tech (CSV)</span>
              <input
                type="text"
                value={form.tech}
                onChange={(e) => setForm((prev) => ({ ...prev, tech: e.target.value }))}
                className={`${fieldInput} font-mono text-xs`}
              />
            </label>
            <label className="block sm:col-span-2">
              <span className={fieldLabel}>Body markdown</span>
              <textarea
                rows={5}
                value={form.body_md}
                onChange={(e) => setForm((prev) => ({ ...prev, body_md: e.target.value }))}
                className={`${fieldInput} font-mono text-xs`}
              />
            </label>
            <div className="sm:col-span-2">
              <p className={`${fieldLabel} mb-1`}>Preview</p>
              {previewError ? (
                <p className="mb-2 font-mono text-xs text-ember-600 dark:text-ember-400">
                  {previewError}
                </p>
              ) : null}
              {previewHtml ? (
                <div
                  className="prose-site text-sm"
                  dangerouslySetInnerHTML={{ __html: previewHtml }}
                />
              ) : (
                <p className="font-mono text-xs text-ink-600 dark:text-ink-400">Empty.</p>
              )}
            </div>
          </div>
          <div className="mt-3 flex flex-wrap gap-2">
            <button
              type="submit"
              disabled={saving}
              className="rounded-chip border border-ink-900 bg-ink-900 px-2.5 py-1 font-mono text-[0.65rem] tracking-[0.14em] text-paper uppercase transition-transform enabled:active:translate-y-0.5 disabled:opacity-50 dark:border-paper dark:bg-paper dark:text-ink-950"
            >
              {saving ? "Saving…" : "Save"}
            </button>
            <button
              type="button"
              onClick={closePanel}
              className="rounded-chip border border-ink-200 px-2.5 py-1 font-mono text-[0.65rem] tracking-[0.14em] text-ink-600 uppercase transition-transform active:translate-y-0.5 dark:border-ink-800 dark:text-ink-400"
            >
              Cancel
            </button>
          </div>
        </form>
      ) : null}

      {entries.length === 0 && panelMode === "closed" ? (
        <div className="mt-6">
          <EmptyState message="No resume entries yet" />
        </div>
      ) : (
        <div className="mt-5 space-y-6">
          {grouped.map(({ section, entries: sectionEntries }) => (
            <div key={section.id}>
              <h2 className="font-display text-lg font-semibold tracking-tight">
                {section.title}
                <span className="ml-2 font-mono text-[0.65rem] font-normal tracking-wide text-ink-600 uppercase dark:text-ink-400">
                  {section.kind}
                </span>
              </h2>
              {sectionEntries.length === 0 ? (
                <p className="mt-2 font-mono text-xs text-ink-600 dark:text-ink-400">
                  No entries in this section.
                </p>
              ) : (
                <div className="mt-2 overflow-x-auto">
                  <table className="w-full min-w-[40rem] border-collapse text-left text-sm">
                    <thead>
                      <tr className="border-b border-ink-200 font-mono text-[0.65rem] tracking-[0.14em] text-ink-600 uppercase dark:border-ink-800 dark:text-ink-400">
                        <th className="py-1.5 pr-3 font-medium">Org</th>
                        <th className="py-1.5 pr-3 font-medium">Role</th>
                        <th className="py-1.5 pr-3 font-medium">Period</th>
                        <th className="py-1.5 pr-3 font-medium">Sort</th>
                        <th className="py-1.5 font-medium">Actions</th>
                      </tr>
                    </thead>
                    <tbody>
                      {sectionEntries.map((entry) => (
                        <tr
                          key={entry.id}
                          className="border-b border-ink-200/80 dark:border-ink-800/80"
                        >
                          <td className="py-1.5 pr-3">{entry.org}</td>
                          <td className="py-1.5 pr-3 text-ink-600 dark:text-ink-400">
                            {entry.role}
                          </td>
                          <td className="py-1.5 pr-3 font-mono text-xs text-ink-600 dark:text-ink-400">
                            {entry.period}
                          </td>
                          <td className="py-1.5 pr-3 font-mono text-xs text-ink-600 dark:text-ink-400">
                            {entry.sort_order}
                          </td>
                          <td className="py-1.5">
                            <span className="inline-flex items-center gap-2">
                              <button
                                type="button"
                                onClick={() => openEdit(entry)}
                                className="font-mono text-[0.65rem] tracking-wide text-ink-600 uppercase transition-[transform,color] hover:text-ink-900 active:translate-y-0.5 dark:text-ink-400 dark:hover:text-paper"
                              >
                                Edit
                              </button>
                              {confirmId === entry.id ? (
                                <span className="inline-flex gap-1.5">
                                  <button
                                    type="button"
                                    disabled={deleting}
                                    onClick={() => void deleteEntry(entry.id)}
                                    className="font-mono text-[0.65rem] tracking-wide text-ember-600 uppercase transition-transform enabled:active:translate-y-0.5 dark:text-ember-400"
                                  >
                                    Confirm
                                  </button>
                                  <button
                                    type="button"
                                    disabled={deleting}
                                    onClick={() => setConfirmId(null)}
                                    className="font-mono text-[0.65rem] tracking-wide text-ink-600 uppercase transition-transform enabled:active:translate-y-0.5 dark:text-ink-400"
                                  >
                                    Cancel
                                  </button>
                                </span>
                              ) : (
                                <button
                                  type="button"
                                  onClick={() => setConfirmId(entry.id)}
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
            </div>
          ))}
        </div>
      )}
    </section>
  );
}

function ResumeHeader({
  onNew,
  disabled,
}: {
  onNew: () => void;
  disabled?: boolean;
}) {
  return (
    <div className="flex items-baseline justify-between gap-4">
      <h1 className="font-display text-2xl font-semibold tracking-tight">Resume</h1>
      <button
        type="button"
        disabled={disabled}
        onClick={onNew}
        className="rounded-chip border border-ink-900 bg-ink-900 px-2.5 py-1 font-mono text-[0.65rem] tracking-[0.14em] text-paper uppercase transition-transform enabled:active:translate-y-0.5 disabled:opacity-40 dark:border-paper dark:bg-paper dark:text-ink-950"
      >
        New entry
      </button>
    </div>
  );
}

function SectionManager({
  sections,
  onChange,
  onUnauthorized,
}: {
  sections: ResumeSection[];
  onChange: () => void;
  onUnauthorized: () => void;
}) {
  const [title, setTitle] = useState("");
  const [kind, setKind] = useState<ResumeSectionKind>("experience");
  const [sortOrder, setSortOrder] = useState(0);
  const [saving, setSaving] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  async function createSection(e: FormEvent) {
    e.preventDefault();
    setSaving(true);
    setErr(null);
    try {
      await apiPost<ResumeSection>("/api/admin/resume/sections", {
        title,
        kind,
        sort_order: sortOrder,
      });
      setTitle("");
      onChange();
    } catch (error: unknown) {
      if (handleAdminUnauthorized(error, onUnauthorized)) return;
      setErr(error instanceof ApiError ? error.message : "Create section failed");
    } finally {
      setSaving(false);
    }
  }

  async function deleteSection(id: number) {
    if (!window.confirm("Delete section and all its entries?")) return;
    try {
      await apiDelete<OkResponse>(`/api/admin/resume/sections/${id}`);
      onChange();
    } catch (error: unknown) {
      if (handleAdminUnauthorized(error, onUnauthorized)) return;
      setErr(error instanceof ApiError ? error.message : "Delete section failed");
    }
  }

  return (
    <div className="mt-4 rounded-card border border-ink-200 p-3 dark:border-ink-800">
      <p className="font-mono text-[0.65rem] tracking-[0.16em] text-ink-600 uppercase dark:text-ink-400">
        Sections
      </p>
      <ul className="mt-2 space-y-1">
        {sections.map((s) => (
          <li key={s.id} className="flex items-center justify-between gap-2 font-mono text-xs">
            <span>
              #{s.id} {s.title} ({s.kind}) · sort {s.sort_order}
            </span>
            <button
              type="button"
              onClick={() => void deleteSection(s.id)}
              className="tracking-[0.12em] text-ink-500 uppercase"
            >
              Delete
            </button>
          </li>
        ))}
      </ul>
      <form onSubmit={createSection} className="mt-3 grid gap-2 sm:grid-cols-4">
        <input
          className={fieldInput}
          placeholder="Title"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          required
        />
        <select
          className={fieldInput}
          value={kind}
          onChange={(e) => setKind(e.target.value as ResumeSectionKind)}
        >
          <option value="experience">experience</option>
          <option value="education">education</option>
          <option value="activity">activity</option>
        </select>
        <input
          type="number"
          className={fieldInput}
          value={sortOrder}
          onChange={(e) => setSortOrder(Number(e.target.value) || 0)}
        />
        <button
          type="submit"
          disabled={saving}
          className="rounded-chip border border-ink-200 px-2 py-1 font-mono text-[0.65rem] tracking-[0.14em] uppercase dark:border-ink-800"
        >
          Add section
        </button>
      </form>
      {err ? <p className="mt-2 font-mono text-xs text-ember-600">{err}</p> : null}
    </div>
  );
}
