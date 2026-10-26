import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";

import { financeApi } from "../api/finance";
import { AppLayout } from "../components/AppLayout";
import { EmptyState, Loading } from "../components/State";
import { formatDate, formatMoney, formatMonth, formatSigned } from "../lib/format";

function thisMonthLabel(): string {
  return new Intl.DateTimeFormat("es-MX", { month: "long", year: "numeric" }).format(new Date());
}

export function DashboardPage() {
  const accountsQuery = useQuery({ queryKey: ["accounts"], queryFn: financeApi.accounts });
  const summaryQuery = useQuery({ queryKey: ["summary"], queryFn: () => financeApi.summary() });
  const monthsQuery = useQuery({ queryKey: ["months", 6], queryFn: () => financeApi.months(6) });
  const recentQuery = useQuery({
    queryKey: ["transactions", { recent: true }],
    queryFn: () => financeApi.transactions({ limit: 5 }),
  });

  const accounts = accountsQuery.data?.accounts ?? [];
  const summary = summaryQuery.data;
  const months = monthsQuery.data?.months ?? [];
  const recent = recentQuery.data?.transactions ?? [];
  const currency = accounts[0]?.currency ?? "COP";

  const expenseTotals = (summary?.by_category ?? []).filter((c) => c.direction === "out");
  const maxExpense = Math.max(1, ...expenseTotals.map((c) => Math.abs(Number(c.total))));
  const maxMonth = Math.max(1, ...months.flatMap((m) => [Number(m.income), Number(m.expenses)]));

  if (accountsQuery.isLoading) {
    return (
      <AppLayout>
        <Loading />
      </AppLayout>
    );
  }

  if (accounts.length === 0) {
    return (
      <AppLayout>
        <div className="mx-auto max-w-5xl space-y-6">
          <h1 className="text-2xl font-semibold tracking-tight">Overview</h1>
          <EmptyState
            icon="🏦"
            title="No accounts yet"
            description="Add an account and record your first transactions. Bank sync arrives in the next milestone."
          >
            <Link to="/accounts" className="btn-primary">
              Add an account
            </Link>
          </EmptyState>
        </div>
      </AppLayout>
    );
  }

  const cards = [
    { label: "Expenses", value: summary?.expenses ?? "0", tone: "text-rose-400", prefix: "−" },
    { label: "Income", value: summary?.income ?? "0", tone: "text-emerald-400", prefix: "+" },
    { label: "Net", value: summary?.net ?? "0", tone: "text-slate-100", prefix: "" },
    { label: "Net worth", value: summary?.net_worth ?? "0", tone: "text-slate-100", prefix: "" },
  ];

  return (
    <AppLayout>
      <div className="mx-auto max-w-5xl space-y-6">
        <div className="flex items-baseline justify-between">
          <h1 className="text-2xl font-semibold tracking-tight">Overview</h1>
          <span className="text-sm capitalize text-slate-500">{thisMonthLabel()}</span>
        </div>

        <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
          {cards.map((card) => (
            <div key={card.label} className="rounded-2xl border border-slate-800 bg-slate-900/60 p-4">
              <div className="text-xs font-medium uppercase tracking-wide text-slate-500">{card.label}</div>
              <div className={`mt-2 text-2xl font-semibold tabular-nums ${card.tone}`}>
                {card.prefix}
                {formatMoney(Math.abs(Number(card.value)), currency)}
              </div>
            </div>
          ))}
        </div>

        <div className="grid gap-4 lg:grid-cols-5">
          <div className="rounded-2xl border border-slate-800 bg-slate-900/60 p-5 lg:col-span-3">
            <h2 className="font-medium">Spending by category</h2>
            {expenseTotals.length === 0 ? (
              <p className="mt-4 text-sm text-slate-500">No expenses this month.</p>
            ) : (
              <ul className="mt-4 space-y-3">
                {expenseTotals.map((category) => (
                  <li key={category.category_id ?? "uncategorized"}>
                    <div className="flex items-baseline justify-between text-sm">
                      <span>
                        {category.icon} {category.name}
                        <span className="ml-2 text-xs text-slate-500">
                          {category.count} transaction{category.count === 1 ? "" : "s"}
                        </span>
                      </span>
                      <span className="tabular-nums text-rose-400">
                        −{formatMoney(Math.abs(Number(category.total)), currency)}
                      </span>
                    </div>
                    <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-slate-800">
                      <div
                        className="h-full rounded-full"
                        style={{
                          width: `${(Math.abs(Number(category.total)) / maxExpense) * 100}%`,
                          backgroundColor: category.color ?? "#34d399",
                        }}
                      />
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </div>

          <div className="rounded-2xl border border-slate-800 bg-slate-900/60 p-5 lg:col-span-2">
            <h2 className="font-medium">Last 6 months</h2>
            {months.every((m) => Number(m.income) === 0 && Number(m.expenses) === 0) ? (
              <p className="mt-4 text-sm text-slate-500">No activity yet.</p>
            ) : (
              <div className="mt-4 flex h-36 items-end gap-3">
                {months.map((month) => (
                  <div key={month.month} className="flex flex-1 flex-col items-center gap-1">
                    <div className="flex h-28 w-full items-end justify-center gap-0.5">
                      <div
                        className="w-1/2 rounded-t bg-emerald-500/80"
                        style={{ height: `${(Number(month.income) / maxMonth) * 100}%` }}
                        title={`Income ${formatMoney(month.income, currency)}`}
                      />
                      <div
                        className="w-1/2 rounded-t bg-rose-500/80"
                        style={{ height: `${(Number(month.expenses) / maxMonth) * 100}%` }}
                        title={`Expenses ${formatMoney(month.expenses, currency)}`}
                      />
                    </div>
                    <span className="text-[10px] capitalize text-slate-500">
                      {formatMonth(month.month).split(" ")[0]}
                    </span>
                  </div>
                ))}
              </div>
            )}
            <div className="mt-3 flex gap-4 text-xs text-slate-500">
              <span className="flex items-center gap-1">
                <span className="h-2 w-2 rounded-full bg-emerald-500/80" /> Income
              </span>
              <span className="flex items-center gap-1">
                <span className="h-2 w-2 rounded-full bg-rose-500/80" /> Expenses
              </span>
            </div>
          </div>
        </div>

        <div className="rounded-2xl border border-slate-800 bg-slate-900/60 p-5">
          <div className="flex items-center justify-between">
            <h2 className="font-medium">Recent transactions</h2>
            <Link to="/transactions" className="text-sm text-emerald-400 hover:text-emerald-300">
              View all →
            </Link>
          </div>
          {recent.length === 0 ? (
            <p className="mt-4 text-sm text-slate-500">Nothing recorded yet.</p>
          ) : (
            <ul className="mt-2 divide-y divide-slate-800/80">
              {recent.map((transaction) => (
                <li key={transaction.id} className="flex items-center justify-between gap-4 py-3 text-sm">
                  <div className="min-w-0">
                    <div className="truncate font-medium">{transaction.description}</div>
                    <div className="text-xs text-slate-500">
                      {formatDate(transaction.occurred_at)} · {transaction.account_name}
                      {transaction.category_name ? ` · ${transaction.category_name}` : ""}
                    </div>
                  </div>
                  <span
                    className={`shrink-0 tabular-nums ${
                      transaction.direction === "in" ? "text-emerald-400" : "text-rose-400"
                    }`}
                  >
                    {formatSigned(transaction.amount, transaction.currency)}
                  </span>
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>
    </AppLayout>
  );
}
