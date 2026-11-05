import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { Link } from "react-router-dom";

import { financeApi, monthRange, type Direction, type Transaction } from "../api/finance";
import { AppLayout } from "../components/AppLayout";
import { TransactionModal } from "../components/TransactionModal";
import { EmptyState, ErrorMessage, Loading } from "../components/State";
import { useI18n } from "../i18n/I18nContext";
import { currentMonthValue, formatDate, formatSigned } from "../lib/format";

const PAGE_SIZE = 50;

export function TransactionsPage() {
  const queryClient = useQueryClient();
  const { t, tCategory, tError } = useI18n();

  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [accountId, setAccountId] = useState("");
  const [direction, setDirection] = useState<Direction | "">("");
  const [uncategorized, setUncategorized] = useState(false);
  const [month, setMonth] = useState(currentMonthValue());
  const [offset, setOffset] = useState(0);
  const [modal, setModal] = useState<{ transaction?: Transaction } | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const timer = setTimeout(() => setDebouncedSearch(search), 300);
    return () => clearTimeout(timer);
  }, [search]);

  useEffect(() => {
    setOffset(0);
  }, [debouncedSearch, accountId, direction, uncategorized, month]);

  const accountsQuery = useQuery({ queryKey: ["accounts"], queryFn: financeApi.accounts });
  const categoriesQuery = useQuery({ queryKey: ["categories"], queryFn: financeApi.categories });
  const accounts = accountsQuery.data?.accounts ?? [];
  const categories = categoriesQuery.data?.categories ?? [];
  const range = month ? monthRange(month) : { from: undefined, to: undefined };

  const transactionsQuery = useQuery({
    queryKey: [
      "transactions",
      { accountId, direction, uncategorized, search: debouncedSearch, month, offset },
    ],
    queryFn: () =>
      financeApi.transactions({
        account_id: accountId || undefined,
        direction: direction || undefined,
        uncategorized,
        search: debouncedSearch || undefined,
        from: range.from,
        to: range.to,
        limit: PAGE_SIZE,
        offset,
      }),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => financeApi.deleteTransaction(id),
    onSuccess: () => {
      setError(null);
      void queryClient.invalidateQueries({ queryKey: ["transactions"] });
      void queryClient.invalidateQueries({ queryKey: ["summary"] });
      void queryClient.invalidateQueries({ queryKey: ["months"] });
      void queryClient.invalidateQueries({ queryKey: ["accounts"] });
    },
    onError: (mutationError) => {
      setError(tError(mutationError, "transactions.deleteFailed"));
    },
  });

  const data = transactionsQuery.data;
  const transactions = data?.transactions ?? [];
  const total = data?.total ?? 0;
  const page = Math.floor(offset / PAGE_SIZE) + 1;
  const pages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  function onDelete(transaction: Transaction) {
    if (window.confirm(t("transactions.deleteConfirm", { description: transaction.description }))) {
      deleteMutation.mutate(transaction.id);
    }
  }

  function resetFilters() {
    setSearch("");
    setAccountId("");
    setDirection("");
    setUncategorized(false);
    setMonth(currentMonthValue());
  }

  return (
    <AppLayout>
      <div className="mx-auto max-w-6xl space-y-6 animate-fade-up">
        <div className="page-header">
          <h1 className="text-2xl font-semibold tracking-tight">{t("nav.transactions")}</h1>
          {accounts.length > 0 ? (
            <button className="btn-primary" onClick={() => setModal({})}>
              {t("transactions.addTransaction")}
            </button>
          ) : (
            <Link to="/accounts" className="btn-primary">
              {t("transactions.addAccountFirst")}
            </Link>
          )}
        </div>

        <div className="surface-card grid grid-cols-1 items-end gap-3 rounded-2xl p-4 sm:grid-cols-2 xl:grid-cols-4">
          <label className="block min-w-0 sm:col-span-2">
            <span className="label">{t("transactions.searchLabel")}</span>
            <input
              className="input"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder={t("transactions.searchPlaceholder")}
            />
          </label>
          <label className="block min-w-0">
            <span className="label">{t("transactions.accountLabel")}</span>
            <select
              className="input"
              value={accountId}
              onChange={(event) => setAccountId(event.target.value)}
            >
              <option value="">{t("transactions.allAccounts")}</option>
              {accounts.map((account) => (
                <option key={account.id} value={account.id}>
                  {account.name}
                </option>
              ))}
            </select>
          </label>
          <label className="block min-w-0">
            <span className="label">{t("transactions.typeLabel")}</span>
            <select
              className="input"
              value={direction}
              onChange={(event) => setDirection(event.target.value as Direction | "")}
            >
              <option value="">{t("transactions.filterAll")}</option>
              <option value="out">{t("transactions.filterExpenses")}</option>
              <option value="in">{t("transactions.filterIncome")}</option>
            </select>
          </label>
          <label className="block min-w-0">
            <span className="label">{t("transactions.monthLabel")}</span>
            <input
              type="month"
              className="input"
              value={month}
              onChange={(event) => setMonth(event.target.value)}
            />
          </label>
          <div className="grid grid-cols-2 gap-2.5 sm:col-span-1 xl:col-span-2">
            <button
              className={`btn-secondary truncate ${uncategorized ? "border-emerald-500/60 text-emerald-300" : ""}`}
              aria-pressed={uncategorized}
              onClick={() => setUncategorized((value) => !value)}
            >
              {t("transactions.filterUncategorized")}
            </button>
            <button className="btn-secondary truncate" onClick={resetFilters}>
              {t("transactions.reset")}
            </button>
          </div>
        </div>

        {error && <ErrorMessage message={error} />}

        {transactionsQuery.isLoading ? (
          <Loading />
        ) : transactions.length === 0 ? (
          <EmptyState
            icon="🧾"
            title={t("transactions.emptyTitle")}
            description={
              accounts.length === 0
                ? t("transactions.emptyNoAccounts")
                : t("transactions.emptyWithAccounts")
            }
          />
        ) : (
          <>
            <div className="surface-card overflow-hidden rounded-2xl">
              <div className="overflow-x-auto">
                <table role="table" className="responsive-table transaction-table text-sm">
                  <thead className="bg-slate-900/80 text-left text-xs uppercase tracking-wide text-slate-500">
                    <tr>
                      <th className="px-4 py-3 font-medium">{t("transactions.colDate")}</th>
                      <th className="px-4 py-3 font-medium">{t("transactions.colDescription")}</th>
                      <th className="px-4 py-3 font-medium">{t("transactions.colCategory")}</th>
                      <th className="px-4 py-3 font-medium">{t("transactions.colAccount")}</th>
                      <th className="px-4 py-3 text-right font-medium">
                        {t("transactions.colAmount")}
                      </th>
                      <th className="px-4 py-3" />
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-800/80">
                    {transactions.map((transaction) => (
                      <tr key={transaction.id}>
                        <td
                          data-col="date"
                          data-label={t("transactions.colDate")}
                          className="whitespace-nowrap px-4 py-3 text-slate-400"
                        >
                          {formatDate(transaction.occurred_at)}
                        </td>
                        <td
                          data-col="description"
                          data-label={t("transactions.colDescription")}
                          className="px-4 py-3"
                        >
                          <div className="font-medium">{transaction.description}</div>
                          {transaction.merchant && (
                            <div className="text-xs text-slate-500">{transaction.merchant}</div>
                          )}
                        </td>
                        <td
                          data-col="category"
                          data-label={t("transactions.colCategory")}
                          className="px-4 py-3"
                        >
                          {transaction.category_name ? (
                            <span className="inline-flex max-w-full flex-wrap items-center gap-1.5 rounded-xl border border-slate-800 bg-slate-900 px-2 py-1 text-xs">
                              <span
                                className="h-2 w-2 rounded-full"
                                style={{
                                  backgroundColor: transaction.category_color ?? "#64748b",
                                }}
                              />
                              {transaction.category_icon}{" "}
                              {tCategory(transaction.category_slug, transaction.category_name)}
                              {transaction.category_source === "rule" && (
                                <span
                                  title={t("transactions.categorizedByRule")}
                                  className="rounded bg-sky-500/10 px-1 py-px text-[10px] font-medium uppercase text-sky-400"
                                >
                                  R
                                </span>
                              )}
                              {transaction.category_source === "ai" && (
                                <span
                                  title={t("transactions.categorizedByAI")}
                                  className="rounded bg-violet-500/10 px-1 py-px text-[10px] font-medium uppercase text-violet-400"
                                >
                                  AI
                                </span>
                              )}
                            </span>
                          ) : (
                            <span className="text-xs text-slate-500">
                              {t("common.uncategorized")}
                            </span>
                          )}
                        </td>
                        <td
                          data-col="account"
                          data-label={t("transactions.colAccount")}
                          className="px-4 py-3 text-slate-400"
                        >
                          {transaction.account_name}
                        </td>
                        <td
                          data-col="amount"
                          data-label={t("transactions.colAmount")}
                          className={`whitespace-nowrap px-4 py-3 text-right tabular-nums ${
                            transaction.direction === "in" ? "text-emerald-400" : "text-rose-400"
                          }`}
                        >
                          {formatSigned(transaction.amount, transaction.currency)}
                        </td>
                        <td
                          data-col="actions"
                          data-label={t("transactions.colActions")}
                          className="px-4 py-3"
                        >
                          <div className="flex justify-end gap-1">
                            <button
                              className="btn-ghost"
                              onClick={() => setModal({ transaction })}
                            >
                              {t("common.edit")}
                            </button>
                            <button
                              className="btn-ghost text-rose-400 hover:text-rose-300"
                              onClick={() => onDelete(transaction)}
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
            </div>

            <div className="flex flex-wrap items-center justify-between gap-4 text-sm text-slate-400">
              <span>
                {total === 1
                  ? t("transactions.pagination.one", { total, page, pages })
                  : t("transactions.pagination.other", { total, page, pages })}
              </span>
              <div className="flex gap-2">
                <button
                  className="btn-secondary"
                  disabled={offset === 0}
                  onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
                >
                  {t("transactions.previous")}
                </button>
                <button
                  className="btn-secondary"
                  disabled={page >= pages}
                  onClick={() => setOffset(offset + PAGE_SIZE)}
                >
                  {t("transactions.next")}
                </button>
              </div>
            </div>
          </>
        )}
      </div>

      {modal && accounts.length > 0 && (
        <TransactionModal
          accounts={accounts}
          categories={categories}
          transaction={modal.transaction}
          onClose={() => setModal(null)}
        />
      )}
    </AppLayout>
  );
}
