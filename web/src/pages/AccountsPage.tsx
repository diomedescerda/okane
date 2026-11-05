import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Link } from "react-router-dom";

import { financeApi, type Account } from "../api/finance";
import { AccountModal } from "../components/AccountModal";
import { ImportModal } from "../components/ImportModal";
import { AppLayout } from "../components/AppLayout";
import { InstitutionIcon } from "../components/InstitutionIcon";
import { EmptyState, ErrorMessage, Loading } from "../components/State";
import { useI18n } from "../i18n/I18nContext";
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
  const { t, tAccountType, tError } = useI18n();
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
    onError: (err) => {
      setError(tError(err, "accounts.deleteFailed"));
    },
  });

  const accounts = accountsQuery.data?.accounts ?? [];

  function onDelete(account: Account) {
    if (window.confirm(t("accounts.deleteConfirm", { name: account.name }))) {
      deleteMutation.mutate(account.id);
    }
  }

  return (
    <AppLayout>
      <div className="mx-auto max-w-5xl space-y-6 animate-fade-up">
        <div className="page-header">
          <h1 className="text-2xl font-semibold tracking-tight">{t("nav.accounts")}</h1>
          <button className="btn-primary" onClick={() => setModal({})}>
            {t("accounts.newAccount")}
          </button>
        </div>

        <ErrorMessage message={error} />

        {accountsQuery.isLoading ? (
          <Loading />
        ) : accounts.length === 0 ? (
          <EmptyState
            icon="🏦"
            title={t("accounts.emptyTitle")}
            description={t("accounts.emptyDescription")}
          >
            <button className="btn-primary" onClick={() => setModal({})}>
              {t("accounts.addFirstAccount")}
            </button>
          </EmptyState>
        ) : (
          <div className="surface-card overflow-x-auto rounded-2xl">
            <table role="table" className="responsive-table text-sm">
              <thead className="bg-slate-900/80 text-left text-xs uppercase tracking-wide text-slate-500">
                <tr>
                  <th className="px-4 py-3 font-medium">{t("accounts.colAccount")}</th>
                  <th className="px-4 py-3 font-medium">{t("accounts.colType")}</th>
                  <th className="px-4 py-3 text-right font-medium">{t("accounts.colBalance")}</th>
                  <th className="px-4 py-3 text-right font-medium">
                    {t("accounts.colTransactions")}
                  </th>
                  <th className="px-4 py-3" />
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/80">
                {accounts.map((account) => (
                  <tr key={account.id} className={account.is_active ? "" : "opacity-50"}>
                    <td
                      data-col="account"
                      data-label={t("accounts.colAccount")}
                      className="px-4 py-3"
                    >
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
                        <span className="min-w-0 break-words">{account.name}</span>
                        {!account.is_active && (
                          <span className="text-xs text-slate-500">{t("accounts.inactive")}</span>
                        )}
                      </div>
                      <div className="text-xs text-slate-500">
                        {account.institution_name || t("accounts.manual")}
                        {account.link_id ? ` · ${t("accounts.linked")}` : ""}
                      </div>
                    </td>
                    <td
                      data-col="type"
                      data-label={t("accounts.colType")}
                      className="px-4 py-3 text-slate-400"
                    >
                      {tAccountType(account.type)}
                    </td>
                    <td
                      data-col="balance"
                      data-label={t("accounts.colBalance")}
                      className={`px-4 py-3 text-right tabular-nums ${
                        Number(account.balance_current) < 0 ? "text-rose-400" : "text-slate-100"
                      }`}
                    >
                      {formatMoney(account.balance_current, account.currency)}
                    </td>
                    <td
                      data-col="transactions"
                      data-label={t("accounts.colTransactions")}
                      className="px-4 py-3 text-right tabular-nums text-slate-400"
                    >
                      {account.transaction_count}
                    </td>
                    <td
                      data-col="actions"
                      data-label={t("accounts.colActions")}
                      className="px-4 py-3"
                    >
                      <div className="flex justify-end gap-1">
                        <button className="btn-ghost" onClick={() => setImportModal({ account })}>
                          {t("accounts.import")}
                        </button>
                        <button className="btn-ghost" onClick={() => setModal({ account })}>
                          {t("common.edit")}
                        </button>
                        <button
                          className="btn-ghost text-rose-400 hover:text-rose-300"
                          onClick={() => onDelete(account)}
                          disabled={deleteMutation.isPending}
                        >
                          {t("common.delete")}
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
            {t("accounts.footerNotePrefix")}{" "}
            <Link to="/transactions" className="text-emerald-400 hover:text-emerald-300">
              {t("accounts.footerNoteLink")}
            </Link>
            .
          </p>
        )}
      </div>

      {modal && <AccountModal account={modal.account} onClose={() => setModal(null)} />}
      {importModal && (
        <ImportModal account={importModal.account} onClose={() => setImportModal(null)} />
      )}
    </AppLayout>
  );
}
