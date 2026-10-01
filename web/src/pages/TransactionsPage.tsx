import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { Link } from "react-router-dom";

import { ApiError } from "../api/client";
import { financeApi, monthRange, type Direction, type Transaction } from "../api/finance";
import { AppLayout } from "../components/AppLayout";
import { TransactionModal } from "../components/TransactionModal";
import { currentMonthValue, formatDate, formatSigned } from "../lib/format";

const PAGE_SIZE = 50;

export function TransactionsPage() {
  const queryClient = useQueryClient();

  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [accountId, setAccountId] = useState("");
  const [direction, setDirection] = useState<Direction | "">("");
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
  }, [debouncedSearch, accountId, direction, month]);

  const accountsQuery = useQuery({ queryKey: ["accounts"], queryFn: financeApi.accounts });
  const categoriesQuery = useQuery({ queryKey: ["categories"], queryFn: financeApi.categories });
  const accounts = accountsQuery.data?.accounts ?? [];
  const categories = categoriesQuery.data?.categories ?? [];
  const range = month ? monthRange(month) : { from: undefined, to: undefined };

  const transactionsQuery = useQuery({
    queryKey: ["transactions", { accountId, direction, search: debouncedSearch, month, offset }],
    queryFn: () =>
      financeApi.transactions({
        account_id: accountId || undefined,
        direction: direction || undefined,
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
      setError(mutationError instanceof ApiError ? mutationError.message : "Could not delete the transaction.");
    },
  });

  const data = transactionsQuery.data;
  const transactions = data?.transactions ?? [];
  const total = data?.total ?? 0;
  const page = Math.floor(offset / PAGE_SIZE) + 1;
  const pages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  function onDelete(transaction: Transaction) {
    if (window.confirm(`Delete "${transaction.description}"?`)) {
      deleteMutation.mutate(transaction.id);
    }
  }

  function resetFilters() {
    setSearch("");
    setAccountId("");
    setDirection("");
    setMonth(currentMonthValue());
  }

  return (
    <AppLayout>
      <div className="mx-auto max-w-6xl space-y-6">
        <div className="flex items-center justify-between">
          <h1 className="text-2xl font-semibold tracking-tight">Transactions</h1>
          {accounts.length > 0 ? (
            <button className="btn-primary" onClick={() => setModal({})}>
              + Add transaction
            </button>
          ) : (
            <Link to="/accounts" className="btn-primary">
              Add an account first
            </Link>
          )}
        </div>

        <div className="flex flex-wrap items-end gap-3 rounded-2xl border border-slate-800 bg-slate-900/60 p-4">
          <label className="block min-w-52 flex-1">
            <span className="label">Search</span>
            <input
              className="input"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder="Description or merchant"
            />
          </label>
          <label className="block">
            <span className="label">Account</span>
            <select className="input" value={accountId} onChange={(event) => setAccountId(event.target.value)}>
              <option value="">All accounts</option>
              {accounts.map((account) => (
                <option key={account.id} value={account.id}>
                  {account.name}
                </option>
              ))}
            </select>
          </label>
          <label className="block">
            <span className="label">Type</span>
            <select
              className="input"
              value={direction}
              onChange={(event) => setDirection(event.target.value as Direction | "")}
            >
              <option value="">All</option>
              <option value="out">Expenses</option>
              <option value="in">Income</option>
            </select>
          </label>
          <label className="block">
            <span className="label">Month</span>
            <input
              type="month"
              className="input"
              value={month}
              onChange={(event) => setMonth(event.target.value)}
            />
          </label>
          <button className="btn-secondary" onClick={resetFilters}>
            Reset
          </button>
        </div>

        {error && (
          <p className="rounded-lg border border-rose-900/60 bg-rose-950/40 px-3 py-2 text-sm text-rose-300">{error}</p>
        )}

        {transactionsQuery.isLoading ? (
          <p className="text-sm text-slate-500">Loading…</p>
        ) : transactions.length === 0 ? (
          <div className="rounded-2xl border border-dashed border-slate-800 bg-slate-900/30 p-10 text-center">
            <div className="text-3xl">🧾</div>
            <h2 className="mt-3 text-lg font-medium">No transactions found</h2>
            <p className="mx-auto mt-1 max-w-md text-sm text-slate-400">
              {accounts.length === 0
                ? "Create an account, then start recording expenses and income."
                : "Try changing the filters, or add a transaction."}
            </p>
          </div>
        ) : (
          <>
            <div className="overflow-hidden rounded-2xl border border-slate-800">
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead className="bg-slate-900/80 text-left text-xs uppercase tracking-wide text-slate-500">
                    <tr>
                      <th className="px-4 py-3 font-medium">Date</th>
                      <th className="px-4 py-3 font-medium">Description</th>
                      <th className="px-4 py-3 font-medium">Category</th>
                      <th className="px-4 py-3 font-medium">Account</th>
                      <th className="px-4 py-3 text-right font-medium">Amount</th>
                      <th className="px-4 py-3" />
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-800/80">
                    {transactions.map((transaction) => (
                      <tr key={transaction.id}>
                        <td className="whitespace-nowrap px-4 py-3 text-slate-400">
                          {formatDate(transaction.occurred_at)}
                        </td>
                        <td className="px-4 py-3">
                          <div className="font-medium">{transaction.description}</div>
                          {transaction.merchant && (
                            <div className="text-xs text-slate-500">{transaction.merchant}</div>
                          )}
                        </td>
                        <td className="px-4 py-3">
                          {transaction.category_name ? (
                            <span className="inline-flex items-center gap-1.5 rounded-full border border-slate-800 bg-slate-900 px-2 py-0.5 text-xs">
                              <span
                                className="h-2 w-2 rounded-full"
                                style={{ backgroundColor: transaction.category_color ?? "#64748b" }}
                              />
                              {transaction.category_icon} {transaction.category_name}
                            </span>
                          ) : (
                            <span className="text-xs text-slate-500">Uncategorized</span>
                          )}
                        </td>
                        <td className="px-4 py-3 text-slate-400">{transaction.account_name}</td>
                        <td
                          className={`whitespace-nowrap px-4 py-3 text-right tabular-nums ${
                            transaction.direction === "in" ? "text-emerald-400" : "text-rose-400"
                          }`}
                        >
                          {formatSigned(transaction.amount, transaction.currency)}
                        </td>
                        <td className="px-4 py-3">
                          <div className="flex justify-end gap-1">
                            <button className="btn-ghost" onClick={() => setModal({ transaction })}>
                              Edit
                            </button>
                            <button
                              className="btn-ghost text-rose-400 hover:text-rose-300"
                              onClick={() => onDelete(transaction)}
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
            </div>

            <div className="flex items-center justify-between text-sm text-slate-400">
              <span>
                {total} transaction{total === 1 ? "" : "s"} · page {page} of {pages}
              </span>
              <div className="flex gap-2">
                <button
                  className="btn-secondary"
                  disabled={offset === 0}
                  onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
                >
                  Previous
                </button>
                <button
                  className="btn-secondary"
                  disabled={page >= pages}
                  onClick={() => setOffset(offset + PAGE_SIZE)}
                >
                  Next
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
