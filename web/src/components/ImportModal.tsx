import { useRef, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";

import { ApiError } from "../api/client";
import { financeApi, type Account, type ImportPreview, type ImportResult } from "../api/finance";
import { formatSigned } from "../lib/format";
import { Modal } from "./Modal";

export function ImportModal({ account, onClose }: { account: Account; onClose: () => void }) {
  const queryClient = useQueryClient();
  const inputRef = useRef<HTMLInputElement>(null);
  const [file, setFile] = useState<File | null>(null);
  const [preview, setPreview] = useState<ImportPreview | null>(null);
  const [result, setResult] = useState<ImportResult | null>(null);
  const [error, setError] = useState<string | null>(null);

  function fail(err: unknown) {
    setError(err instanceof ApiError ? err.message : "Something went wrong");
  }

  const previewMutation = useMutation({
    mutationFn: (f: File) => financeApi.importStatement(account.id, f, false),
    onSuccess: (data) => {
      setError(null);
      setPreview(data.preview ?? null);
    },
    onError: fail,
  });

  const commitMutation = useMutation({
    mutationFn: (f: File) => financeApi.importStatement(account.id, f, true),
    onSuccess: (data) => {
      setError(null);
      setResult(data.result ?? null);
      void queryClient.invalidateQueries({ queryKey: ["transactions"] });
      void queryClient.invalidateQueries({ queryKey: ["accounts"] });
      void queryClient.invalidateQueries({ queryKey: ["summary"] });
      void queryClient.invalidateQueries({ queryKey: ["summary", "months"] });
    },
    onError: fail,
  });

  function onPick(f: File | null) {
    setFile(f);
    setPreview(null);
    setResult(null);
    setError(null);
  }

  const pending = previewMutation.isPending || commitMutation.isPending;

  return (
    <Modal
      title={`Import statement · ${account.name}`}
      onClose={onClose}
      footer={
        result ? (
          <button type="button" className="btn-primary" onClick={onClose}>
            Done
          </button>
        ) : (
          <>
            <button type="button" className="btn-ghost" onClick={onClose} disabled={pending}>
              Cancel
            </button>
            <button
              type="button"
              className="btn-primary"
              disabled={!file || pending}
              onClick={() => {
                if (!file) return;
                if (preview) {
                  commitMutation.mutate(file);
                } else {
                  previewMutation.mutate(file);
                }
              }}
            >
              {pending ? "Working…" : preview ? `Import ${preview.total} transactions` : "Preview"}
            </button>
          </>
        )
      }
    >
      <div className="space-y-4">
        <p className="text-sm text-slate-400">
          Upload a CSV or OFX statement exported from {account.institution_name || "your bank"}.
          Nothing is saved until you confirm the preview.
        </p>

        <input
          ref={inputRef}
          type="file"
          accept=".csv,.ofx,.qfx,text/csv,application/x-ofx"
          className="block w-full text-sm text-slate-300 file:mr-3 file:rounded-lg file:border-0 file:bg-slate-800 file:px-3 file:py-2 file:text-sm file:text-slate-200 hover:file:bg-slate-700"
          onChange={(e) => onPick(e.target.files?.[0] ?? null)}
        />

        {error && <p className="text-sm text-rose-400">{error}</p>}

        {preview && !result && (
          <div className="space-y-3">
            <div className="flex flex-wrap gap-x-4 gap-y-1 text-sm text-slate-400">
              <span>{preview.total} transactions found</span>
              {preview.date_from && <span>{preview.date_from} → {preview.date_to}</span>}
              {preview.existing > 0 && <span className="text-amber-400">{preview.existing} already imported</span>}
              {preview.repeated > 0 && <span className="text-amber-400">{preview.repeated} repeated in the file</span>}
              {preview.skipped > 0 && <span>{preview.skipped} rows skipped</span>}
            </div>

            <div className="max-h-72 overflow-y-auto rounded-lg border border-slate-800">
              <table className="w-full text-sm">
                <thead className="sticky top-0 bg-slate-900 text-left text-xs uppercase tracking-wide text-slate-500">
                  <tr>
                    <th className="px-3 py-2">Date</th>
                    <th className="px-3 py-2">Description</th>
                    <th className="px-3 py-2 text-right">Amount</th>
                  </tr>
                </thead>
                <tbody>
                  {preview.rows.map((row, index) => (
                    <tr key={index} className="border-t border-slate-800">
                      <td className="whitespace-nowrap px-3 py-2 text-slate-400">{row.date}</td>
                      <td className="px-3 py-2">
                        <span className={row.duplicate ? "text-slate-500" : "text-slate-200"}>{row.description}</span>
                        {row.duplicate && (
                          <span className="ml-2 rounded bg-amber-500/10 px-1.5 py-0.5 text-[10px] uppercase text-amber-400">
                            already imported
                          </span>
                        )}
                        {row.repeated && (
                          <span className="ml-2 rounded bg-slate-800 px-1.5 py-0.5 text-[10px] uppercase text-slate-400">
                            in file
                          </span>
                        )}
                      </td>
                      <td className="whitespace-nowrap px-3 py-2 text-right tabular-nums">{formatSigned(row.amount, account.currency)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            {preview.truncated && <p className="text-xs text-slate-500">Showing the first {preview.rows.length} rows of {preview.total}.</p>}
          </div>
        )}

        {result && (
          <div className="rounded-lg border border-emerald-500/30 bg-emerald-500/5 p-4 text-sm">
            <p className="font-medium text-emerald-400">Import finished</p>
            <p className="mt-1 text-slate-300">
              {result.imported} imported · {result.duplicate} already existed · {result.skipped} skipped
            </p>
          </div>
        )}
      </div>
    </Modal>
  );
}
