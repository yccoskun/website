import { useCallback, useEffect, useState } from "react";
import type { ChangeEvent } from "react";

import { EmptyState, ErrorState, LoadingState } from "@/components/states";
import { ApiError, apiDelete, apiGet, apiPost, apiUpload } from "@/lib/api";
import { useDocumentMeta } from "@/lib/meta";
import { handleAdminUnauthorized, useAdminSession } from "@/pages/admin/session";
import type { OkResponse } from "@/types/admin";
import type { ImportResult, MediaAsset } from "@/types/cms";

export function AdminMediaPage() {
  const { onUnauthorized } = useAdminSession();
  const [items, setItems] = useState<MediaAsset[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [uploading, setUploading] = useState(false);
  const [importText, setImportText] = useState("");
  const [importing, setImporting] = useState(false);
  const [importMsg, setImportMsg] = useState<string | null>(null);
  const [exporting, setExporting] = useState(false);
  const [exportMsg, setExportMsg] = useState<string | null>(null);

  useDocumentMeta("Admin · Media", "Upload and manage media assets.");

  const load = useCallback(() => {
    setLoading(true);
    void apiGet<MediaAsset[]>("/api/admin/media")
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

  function onFile(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    setUploading(true);
    setError(null);
    void apiUpload<MediaAsset>("/api/admin/media", file)
      .then(() => {
        setUploading(false);
        e.target.value = "";
        load();
      })
      .catch((err: unknown) => {
        if (handleAdminUnauthorized(err, onUnauthorized)) return;
        setError(err instanceof ApiError ? err.message : "Upload failed");
        setUploading(false);
      });
  }

  function onDelete(id: number) {
    if (!window.confirm("Delete this media asset?")) return;
    void apiDelete<OkResponse>(`/api/admin/media/${id}`)
      .then(() => load())
      .catch((err: unknown) => {
        if (handleAdminUnauthorized(err, onUnauthorized)) return;
        setError(err instanceof ApiError ? err.message : "Delete failed");
      });
  }

  function onImport() {
    setImporting(true);
    setImportMsg(null);
    let payload: unknown;
    try {
      payload = JSON.parse(importText) as unknown;
    } catch {
      setImportMsg("Invalid JSON");
      setImporting(false);
      return;
    }
    void apiPost<ImportResult>("/api/admin/import", payload)
      .then((result) => {
        setImporting(false);
        setImportMsg(
          `Imported: settings ${result.settings_upserted}, pages ${result.pages_upserted}, work ${result.work_created}, studio ${result.studio_created}, sections ${result.sections_created}, entries ${result.entries_created}`,
        );
      })
      .catch((err: unknown) => {
        if (handleAdminUnauthorized(err, onUnauthorized)) return;
        setImportMsg(err instanceof ApiError ? err.message : "Import failed");
        setImporting(false);
      });
  }

  function onExport() {
    setExporting(true);
    setExportMsg(null);
    void apiPost<Record<string, unknown>>("/api/admin/export", {})
      .then((dump) => {
        const text = JSON.stringify(dump, null, 2);
        const blob = new Blob([text], { type: "application/json" });
        const url = URL.createObjectURL(blob);
        const a = document.createElement("a");
        const stamp = new Date().toISOString().slice(0, 10);
        a.href = url;
        a.download = `website-content-${stamp}.json`;
        a.click();
        URL.revokeObjectURL(url);
        setImportText(text);
        setExporting(false);
        setExportMsg("Downloaded JSON dump. Media files are not included — upload those separately on the target host.");
      })
      .catch((err: unknown) => {
        if (handleAdminUnauthorized(err, onUnauthorized)) return;
        setExportMsg(err instanceof ApiError ? err.message : "Export failed");
        setExporting(false);
      });
  }

  if (loading) return <LoadingState />;

  return (
    <div className="max-w-3xl space-y-10">
      <div>
        <h1 className="font-display text-2xl font-semibold">Media</h1>
        <p className="mt-1 font-mono text-xs text-ink-600 dark:text-ink-400">
          Upload PDF / images. Public URL: /media/&#123;id&#125;
        </p>
        <label className="mt-4 inline-block">
          <span className="rounded-chip border border-ink-900 bg-ink-900 px-3 py-1.5 font-mono text-xs tracking-[0.14em] text-paper uppercase dark:border-paper dark:bg-paper dark:text-ink-950">
            {uploading ? "Uploading…" : "Upload file"}
          </span>
          <input type="file" className="sr-only" onChange={onFile} disabled={uploading} />
        </label>
        {error ? (
          <div className="mt-4">
            <ErrorState message={error} />
          </div>
        ) : null}
        {items.length === 0 ? (
          <div className="mt-6">
            <EmptyState message="No media yet." />
          </div>
        ) : (
          <ul className="mt-6 space-y-2">
            {items.map((item) => (
              <li
                key={item.id}
                className="flex flex-wrap items-center justify-between gap-2 border-b border-ink-200 py-2 font-mono text-xs dark:border-ink-800"
              >
                <div>
                  <span className="text-ink-900 dark:text-ink-200">
                    #{item.id} {item.original_name}
                  </span>
                  <span className="ml-2 text-ink-500">{item.mime}</span>
                  <a
                    href={item.url}
                    target="_blank"
                    rel="noreferrer"
                    className="ml-2 text-ember-600 dark:text-ember-400"
                  >
                    {item.url}
                  </a>
                </div>
                <button
                  type="button"
                  onClick={() => onDelete(item.id)}
                  className="tracking-[0.12em] text-ink-500 uppercase"
                >
                  Delete
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>

      <div>
        <h2 className="font-display text-xl font-semibold">Content export / import</h2>
        <p className="mt-1 font-mono text-xs text-ink-600 dark:text-ink-400">
          Export local content as JSON, then import on another environment (e.g. production).
          Media binaries are not included — upload PDF/stills separately and remapping{" "}
          <code className="text-ink-700 dark:text-ink-300">pdf_media_id</code> /{" "}
          <code className="text-ink-700 dark:text-ink-300">image_media_id</code> if needed.
        </p>
        <div className="mt-3 flex flex-wrap gap-2">
          <button
            type="button"
            onClick={onExport}
            disabled={exporting}
            className="rounded-chip border border-ink-900 bg-ink-900 px-3 py-1.5 font-mono text-xs tracking-[0.14em] text-paper uppercase transition-transform active:translate-y-0.5 disabled:opacity-50 dark:border-paper dark:bg-paper dark:text-ink-950"
          >
            {exporting ? "Exporting…" : "Export JSON"}
          </button>
          <button
            type="button"
            onClick={onImport}
            disabled={importing || !importText.trim()}
            className="rounded-chip border border-ink-200 px-3 py-1.5 font-mono text-xs tracking-[0.14em] text-ink-700 uppercase transition-transform active:translate-y-0.5 disabled:opacity-50 dark:border-ink-800 dark:text-ink-300"
          >
            {importing ? "Importing…" : "Import"}
          </button>
        </div>
        <textarea
          className="mt-3 min-h-40 w-full rounded-chip border border-ink-200 bg-paper px-2 py-1 font-mono text-xs outline-none focus:border-ember-600 dark:border-ink-800 dark:bg-ink-950 dark:focus:border-ember-400"
          value={importText}
          onChange={(e) => setImportText(e.target.value)}
          spellCheck={false}
          placeholder="{ ... }"
        />
        {exportMsg ? (
          <p className="mt-2 font-mono text-xs text-ink-600 dark:text-ink-400">{exportMsg}</p>
        ) : null}
        {importMsg ? (
          <p className="mt-2 font-mono text-xs text-ink-600 dark:text-ink-400">{importMsg}</p>
        ) : null}
      </div>
    </div>
  );
}
