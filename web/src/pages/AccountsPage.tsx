import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Link } from "react-router-dom";

import { ACCOUNT_TYPE_LABELS, financeApi, type Account } from "../api/finance";
import { ApiError } from "../api/client";
import { AccountModal } from "../components/AccountModal";
import { ImportModal } from "../components/ImportModal";
import { AppLayout } from "../components/AppLayout";
import { InstitutionIcon } from "../components/InstitutionIcon";
import { formatMoney } from "../lib/format";

const TYPE_ICONS: Record<string, string> = {
  checking: "🏦",
  savings: "🐖",
  credit_card: "💳",
  loan: "🏛️",
  investment: "📈",
  other: "💼",
};

export function AccountsPage() {
  const queryClient = useQueryClient();
  const [modal, setModal] = useState<{ account?: Account } | null>(null);
  const [importModal, setImportModal] = useState<{ account: Account } | null>(null);
  const [error, setError] = useState<string | null>(null);

  const accountsQuery = useQuery({ queryKey: ["accounts"], queryFn: financeApi.accounts });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => financeApi.deleteAccount(id),
    onSuccess: () => {
      setError(null);
      void queryClient.invalidateQueries({ queryKey: ["accounts"] });
      void queryClient.invalidateQueries({ queryKey: ["transactions"] });
      void queryClient.invalidateQueries({ queryKey: ["summary"] });
      void queryClient.invalidateQueries({ queryKey: ["months"] });
    },
    onError: (error) => {
      setError(error instanceof ApiError ? error.message : "Could not delete the account.");
    },
  });

  const accounts = accountsQuery.data?.accounts ?? [];

  function onDelete(account: Account) {
    if (window.confirm(`Delete "${account.name}" and all of its transactions?`)) {
      deleteMutation.mutate(account.id);
    }
  }

  return (
    <AppLayout>
      <div className="mx-auto max-w-5xl space-y-6">
        <div className="flex items-center justify-between">
          <h1 className="text-2xl font-semibold tracking-tight">Accounts</h1>
          <button className="btn-primary" onClick={() => setModal({})}>
            + New account
          </button>
        </div>

        {error && (
          <p className="rounded-lg border border-rose-900/60 bg-rose-950/40 px-3 py-2 text-sm text-rose-300">{error}</p>
        )}

        {accountsQuery.isLoading ? (
          <p className="text-sm text-slate-500">Loading…</p>
        ) : accounts.length === 0 ? (
          <div className="rounded-2xl border border-dashed border-slate-800 bg-slate-900/30 p-10 text-center">
            <div className="text-3xl">🏦</div>
            <h2 className="mt-3 text-lg font-medium">No accounts yet</h2>
            <p className="mx-auto mt-1 max-w-md text-sm text-slate-400">
              Add the accounts you want to track. Connect a bank from the{" "}
              <Link to="/banks" className="text-emerald-400 hover:text-emerald-300">
                Banks
              </Link>{" "}
              page or add them manually.
            </p>
            <button className="btn-primary mt-4" onClick={() => setModal({})}>
              Add your first account
            </button>
          </div>
        ) : (
          <div className="overflow-hidden rounded-2xl border border-slate-800">
            <table className="w-full text-sm">
              <thead className="bg-slate-900/80 text-left text-xs uppercase tracking-wide text-slate-500">
                <tr>
                  <th className="px-4 py-3 font-medium">Account</th>
                  <th className="px-4 py-3 font-medium">Type</th>
                  <th className="px-4 py-3 text-right font-medium">Balance</th>
                  <th className="px-4 py-3 text-right font-medium">Transactions</th>
                  <th className="px-4 py-3" />
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/80">
                {accounts.map((account) => (
                  <tr key={account.id} className={account.is_active ? "" : "opacity-50"}>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2 font-medium">
                        {account.link_id ? (
                          <InstitutionIcon
                            name={account.institution_name || account.name}
                            logoUrl={account.institution_logo_url}
                            icon={account.institution_icon}
                            color={account.institution_color}
                            size="sm"
                          />
                        ) : (
                          <span aria-hidden>{TYPE_ICONS[account.type] ?? "💼"}</span>
                        )}
                        {account.name}
                        {!account.is_active && <span className="text-xs text-slate-500">(inactive)</span>}
                      </div>
                      <div className="text-xs text-slate-500">
                        {account.institution_name || "Manual"}
                        {account.link_id ? " · linked" : ""}
                      </div>
                    </td>
                    <td className="px-4 py-3 text-slate-400">{ACCOUNT_TYPE_LABELS[account.type]}</td>
                    <td
                      className={`px-4 py-3 text-right tabular-nums ${
                        Number(account.balance_current) < 0 ? "text-rose-400" : "text-slate-100"
                      }`}
                    >
                      {formatMoney(account.balance_current, account.currency)}
                    </td>
                    <td className="px-4 py-3 text-right tabular-nums text-slate-400">{account.transaction_count}</td>
                    <td className="px-4 py-3">
                      <div className="flex justify-end gap-1">
                        <button className="btn-ghost" onClick={() => setImportModal({ account })}>
                          Import
                        </button>
                        <button className="btn-ghost" onClick={() => setModal({ account })}>
                          Edit
                        </button>
                        <button
                          className="btn-ghost text-rose-400 hover:text-rose-300"
                          onClick={() => onDelete(account)}
                          disabled={deleteMutation.isPending}
                        >
                          Delete
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {accounts.length > 0 && (
          <p className="text-sm text-slate-500">
            Balances for manual accounts are the opening balance plus tracked transactions. Need to record something?{" "}
            <Link to="/transactions" className="text-emerald-400 hover:text-emerald-300">
              Add a transaction
            </Link>
            .
          </p>
        )}
      </div>

      {modal && <AccountModal account={modal.account} onClose={() => setModal(null)} />}
      {importModal && <ImportModal account={importModal.account} onClose={() => setImportModal(null)} />}
    </AppLayout>
  );
}
