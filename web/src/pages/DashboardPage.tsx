import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";

import { financeApi, monthRange, type MonthTotal } from "../api/finance";
import { AppLayout } from "../components/AppLayout";
import { EmptyState, ErrorMessage, Loading } from "../components/State";
import { useI18n } from "../i18n/I18nContext";
import {
  computeBarHeightPct,
  computeCategoryPareto,
  computeLiquiditySummary,
  computeMonthlyBurnPace,
  computeSavingsRate,
  computeTopMerchants,
  computeWeekdayRhythm,
} from "../lib/analytics";
import {
  currentMonthValue,
  formatCompactMoney,
  formatDate,
  formatMoney,
  formatMonthLong,
  formatMonthShort,
  formatSigned,
  getWeekdayLabels,
} from "../lib/format";

export function DashboardPage() {
  const { language, t, tCategory, tAccountType, tError } = useI18n();
  const defaultMonth = useMemo(() => currentMonthValue(), []);
  const [selectedMonth, setSelectedMonth] = useState<string>(defaultMonth);

  const range = useMemo(
    () => (selectedMonth ? monthRange(selectedMonth) : { from: undefined, to: undefined }),
    [selectedMonth],
  );

  const accountsQuery = useQuery({ queryKey: ["accounts"], queryFn: financeApi.accounts });
  const summaryQuery = useQuery({
    queryKey: ["summary", selectedMonth],
    queryFn: () => financeApi.summary(range.from, range.to),
  });
  const monthsQuery = useQuery({ queryKey: ["months", 6], queryFn: () => financeApi.months(6) });

  // Load up to 200 transactions for the selected month so the Burn Pace, Weekday Rhythm,
  // and Top Outflow Drivers charts reflect the entire month accurately.
  const monthTransactionsQuery = useQuery({
    queryKey: ["transactions", "dashboard-month", selectedMonth],
    queryFn: () =>
      financeApi.transactions({
        from: range.from,
        to: range.to,
        limit: 200,
      }),
  });

  const recentQuery = useQuery({
    queryKey: ["transactions", { recent: true }],
    queryFn: () => financeApi.transactions({ limit: 6 }),
  });

  const accounts = accountsQuery.data?.accounts ?? [];
  const summary = summaryQuery.data;
  const months = monthsQuery.data?.months ?? [];
  const recent = recentQuery.data?.transactions ?? [];
  const currency = accounts[0]?.currency ?? "COP";

  if (accountsQuery.isLoading) {
    return (
      <AppLayout>
        <Loading />
      </AppLayout>
    );
  }

  if (accountsQuery.error) {
    return (
      <AppLayout>
        <ErrorMessage message={tError(accountsQuery.error, "common.somethingWentWrong")} />
      </AppLayout>
    );
  }

  if (accounts.length === 0) {
    return (
      <AppLayout>
        <div className="mx-auto max-w-5xl space-y-6 animate-fade-up">
          <div className="page-header">
            <h1 className="text-2xl font-semibold tracking-tight">{t("nav.overview")}</h1>
            <span className="text-sm capitalize text-slate-400">
              {formatMonthLong(selectedMonth, language)}
            </span>
          </div>
          <EmptyState
            icon="🏦"
            title={t("dashboard.noAccountsTitle")}
            description={t("dashboard.noAccountsDescription")}
          >
            <Link to="/accounts" className="btn-primary">
              {t("dashboard.addAccount")}
            </Link>
          </EmptyState>
        </div>
      </AppLayout>
    );
  }

  const totalIncome = summary?.income ?? "0";
  const totalExpenses = summary?.expenses ?? "0";
  const netString = summary?.net ?? "0";
  const netNumber = Number(netString);
  const netWorth = summary?.net_worth ?? "0";

  const activeMonthRecord: MonthTotal =
    months.find((m) => m.month === selectedMonth) ?? {
      month: selectedMonth,
      income: totalIncome,
      expenses: totalExpenses,
      net: netString,
    };

  const maxMonthValue = Math.max(
    1,
    ...months.flatMap((m) => [Math.abs(Number(m.income)), Math.abs(Number(m.expenses))]),
  );

  const savingsInfo = computeSavingsRate({
    income: totalIncome,
    expenses: totalExpenses,
  });

  const pareto = computeCategoryPareto(summary?.by_category ?? []);
  const monthTxItems = monthTransactionsQuery.data?.transactions ?? recent;

  const burnPace = computeMonthlyBurnPace(
    selectedMonth,
    monthTxItems,
    Math.abs(Number(totalIncome) || 0),
  );
  const weekdayRhythm = computeWeekdayRhythm(monthTxItems);
  const weekdayLabels = getWeekdayLabels(language);
  const topMerchants = computeTopMerchants(monthTxItems, 5);
  const liquidity = computeLiquiditySummary(accounts);

  return (
    <AppLayout>
      <div className="mx-auto max-w-5xl space-y-7 pb-4">
        {/* Header + Interactive Month Selector */}
        <header className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between animate-fade-up">
          <div>
            <p className="text-xs font-medium uppercase tracking-wider text-emerald-300/90">
              {formatMonthLong(selectedMonth, language)}
            </p>
            <h1 className="mt-1 text-2xl font-semibold tracking-tight text-slate-100 sm:text-3xl">
              {t("nav.overview")}
            </h1>
          </div>

          {months.length > 0 && (
            <div
              className="flex max-w-full items-center gap-1 overflow-x-auto rounded-xl border border-slate-800 bg-slate-900/80 p-1 text-xs backdrop-blur-md"
              role="group"
              aria-label={t("dashboard.lastSixMonths")}
            >
              {months.map((m) => {
                const isSelected = m.month === selectedMonth;
                return (
                  <button
                    key={m.month}
                    type="button"
                    onClick={() => setSelectedMonth(m.month)}
                    aria-pressed={isSelected}
                    className={`shrink-0 rounded-lg px-2.5 py-1.5 font-medium whitespace-nowrap transition ${
                      isSelected
                        ? "bg-emerald-400/20 text-emerald-200 ring-1 ring-inset ring-emerald-400/40"
                        : "text-slate-400 hover:bg-slate-800/70 hover:text-slate-200"
                    }`}
                  >
                    {formatMonthShort(m.month, language)}
                  </button>
                );
              })}
            </div>
          )}
        </header>

        {/* Primary KPI Strip */}
        <section className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4 animate-fade-up stagger-1">
          <MetricCard
            label={t("dashboard.expenses")}
            value={`−${formatMoney(Math.abs(Number(totalExpenses)), currency)}`}
            tone="negative"
            badge={
              pareto.items.length > 0
                ? (() => {
                    const totalCount = pareto.items.reduce((acc, item) => acc + item.count, 0);
                    return totalCount === 1
                      ? t("dashboard.transactionCount.one", { count: totalCount })
                      : t("dashboard.transactionCount.other", { count: totalCount });
                  })()
                : undefined
            }
          />
          <MetricCard
            label={t("dashboard.income")}
            value={`+${formatMoney(Math.abs(Number(totalIncome)), currency)}`}
            tone="positive"
          />
          <MetricCard
            label={t("dashboard.net")}
            value={formatMoney(netNumber, currency)}
            tone={netNumber > 0 ? "positive" : netNumber < 0 ? "negative" : "neutral"}
            badge={
              savingsInfo.ratePct !== null
                ? savingsInfo.ratePct >= 0
                  ? t("dashboard.savingsRateValue", { rate: savingsInfo.ratePct })
                  : t("dashboard.overspentBadge")
                : savingsInfo.isOverspent
                  ? t("dashboard.overspentBadge")
                  : t("dashboard.noIncomeRecorded")
            }
          />
          <MetricCard
            label={t("dashboard.netWorth")}
            value={formatMoney(Math.abs(Number(netWorth)), currency)}
            tone="neutral"
            badge={`${accounts.filter((a) => a.is_active).length} ${t("nav.accounts").toLowerCase()}`}
          />
        </section>

        {/* Primary Row: Spending by Category (Pareto 80/20) + Redesigned 6-Month Cash Flow */}
        <section className="grid grid-cols-1 gap-5 lg:grid-cols-5 animate-fade-up stagger-2">
          {/* Spending by Category */}
          <div className="surface-card flex flex-col justify-between rounded-2xl p-5 lg:col-span-3">
            <div>
              <div className="flex flex-wrap items-center justify-between gap-2">
                <h2 className="font-medium text-slate-100">{t("dashboard.spendingByCategory")}</h2>
                {pareto.vitalFewCount > 0 && pareto.items.length > 1 && (
                  <span className="inline-flex items-center gap-1.5 rounded-full border border-emerald-400/25 bg-emerald-400/10 px-2.5 py-0.5 text-[11px] font-medium text-emerald-200">
                    <span className="size-1.5 rounded-full bg-emerald-300" />
                    {t("dashboard.vitalFewHint", {
                      count: pareto.vitalFewCount,
                      share: pareto.vitalFewSharePct,
                    })}
                  </span>
                )}
              </div>

              {pareto.items.length === 0 ? (
                <p className="mt-6 py-6 text-center text-sm text-slate-500">
                  {t("dashboard.noExpensesThisMonth")}
                </p>
              ) : (
                <>
                  {/* Stacked Composition Bar */}
                  <div
                    className="mt-4 flex h-2.5 w-full overflow-hidden rounded-full bg-slate-950/90 p-0.5 ring-1 ring-inset ring-slate-800"
                    aria-hidden="true"
                  >
                    {pareto.items.map((cat) => (
                      <div
                        key={`stack-${cat.key}`}
                        style={{
                          width: `${Math.max(2, cat.sharePct)}%`,
                          backgroundColor: cat.color,
                        }}
                        className="h-full first:rounded-l-full last:rounded-r-full transition-all duration-500"
                        title={`${tCategory(cat.slug, cat.name)}: ${cat.sharePct}%`}
                      />
                    ))}
                  </div>

                  {/* Category List */}
                  <ul className="mt-5 space-y-3.5">
                    {pareto.items.slice(0, 7).map((cat) => {
                      const displayCategoryName = tCategory(cat.slug, cat.name);
                      return (
                        <li key={cat.key} className="space-y-1.5">
                          <div className="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1 text-sm">
                            <span className="flex min-w-0 items-center gap-2 text-slate-100">
                              <span className="shrink-0 text-base leading-none">{cat.icon}</span>
                              <span className="truncate font-medium">{displayCategoryName}</span>
                              <span className="shrink-0 rounded-md bg-slate-800/80 px-1.5 py-0.5 font-mono text-[11px] text-slate-300">
                                {t("dashboard.categoryShare", { share: cat.sharePct })}
                              </span>
                              {cat.isVitalFew && pareto.items.length > 2 && (
                                <span className="hidden rounded bg-amber-400/15 px-1.5 py-0.5 text-[10px] font-medium text-amber-300 sm:inline-block">
                                  {t("dashboard.vitalFewBadge")}
                                </span>
                              )}
                            </span>
                            <span className="shrink-0 font-mono text-sm font-semibold tabular-nums text-rose-300">
                              −{formatMoney(cat.total, currency)}
                            </span>
                          </div>

                          <div className="flex items-center gap-3">
                            <div className="h-2 flex-1 overflow-hidden rounded-full bg-slate-950/90 ring-1 ring-inset ring-slate-800/80">
                              <div
                                className="h-full rounded-full animate-bar-x"
                                style={{
                                  width: `${Math.min(100, Math.max(4, cat.sharePct))}%`,
                                  backgroundColor: cat.color,
                                }}
                              />
                            </div>
                            <span className="shrink-0 text-[11px] tabular-nums text-slate-400">
                              {cat.count === 1
                                ? t("dashboard.transactionCount.one", { count: cat.count })
                                : t("dashboard.transactionCount.other", { count: cat.count })}
                              {" · "}
                              {t("dashboard.avgPerTransaction", {
                                amount: formatCompactMoney(cat.avgPerTransaction, currency, language),
                              })}
                            </span>
                          </div>
                        </li>
                      );
                    })}
                  </ul>
                </>
              )}
            </div>
          </div>

          {/* Redesigned Last 6 Months Cash Flow Chart */}
          <div className="surface-card flex flex-col justify-between rounded-2xl p-5 lg:col-span-2">
            <div>
              <div className="flex items-center justify-between gap-2">
                <div>
                  <h2 className="font-medium text-slate-100">{t("dashboard.lastSixMonths")}</h2>
                  <p className="text-[11px] text-slate-400">{t("dashboard.cashFlowSubtitle")}</p>
                </div>
                <div className="flex items-center gap-3 text-xs text-slate-400">
                  <span className="inline-flex items-center gap-1.5">
                    <span className="h-2 w-2 rounded-full bg-emerald-400" />
                    {t("dashboard.income")}
                  </span>
                  <span className="inline-flex items-center gap-1.5">
                    <span className="h-2 w-2 rounded-full bg-rose-400" />
                    {t("dashboard.expenses")}
                  </span>
                </div>
              </div>

              {months.every((m) => Number(m.income) === 0 && Number(m.expenses) === 0) ? (
                <p className="mt-8 py-6 text-center text-sm text-slate-500">
                  {t("dashboard.noActivityYet")}
                </p>
              ) : (
                <>
                  {/* Selected Month Detail Banner */}
                  <div className="mt-3.5 flex flex-wrap items-center justify-between gap-2 rounded-xl border border-slate-800/90 bg-slate-950/65 px-3 py-2 text-xs">
                    <span className="font-medium text-slate-200">
                      {formatMonthLong(activeMonthRecord.month, language)}
                    </span>
                    <div className="flex items-center gap-2.5 font-mono tabular-nums">
                      <span className="text-emerald-300" title={t("dashboard.income")}>
                        +{formatCompactMoney(activeMonthRecord.income, currency, language)}
                      </span>
                      <span className="text-rose-300" title={t("dashboard.expenses")}>
                        −{formatCompactMoney(activeMonthRecord.expenses, currency, language)}
                      </span>
                      <span
                        className={`rounded px-1.5 py-0.5 text-[11px] font-semibold ${
                          Number(activeMonthRecord.net) >= 0
                            ? "bg-emerald-500/15 text-emerald-300"
                            : "bg-rose-500/15 text-rose-300"
                        }`}
                      >
                        {Number(activeMonthRecord.net) >= 0 ? "+" : "−"}
                        {formatCompactMoney(
                          Math.abs(Number(activeMonthRecord.net)),
                          currency,
                          language,
                        )}
                      </span>
                    </div>
                  </div>

                  {/* Structured Plot Area with Grid Reference Lines */}
                  <div className="relative mt-4 pt-2">
                    <div
                      className="pointer-events-none absolute inset-x-0 top-2 bottom-7 flex flex-col justify-between"
                      aria-hidden="true"
                    >
                      <div className="flex items-center gap-2">
                        <span className="w-10 shrink-0 text-right font-mono text-[10px] text-slate-500">
                          {formatCompactMoney(maxMonthValue, currency, language)}
                        </span>
                        <div className="flex-1 border-b border-dashed border-slate-800/80" />
                      </div>
                      <div className="flex items-center gap-2">
                        <span className="w-10 shrink-0 text-right font-mono text-[10px] text-slate-500">
                          {formatCompactMoney(maxMonthValue / 2, currency, language)}
                        </span>
                        <div className="flex-1 border-b border-dashed border-slate-800/60" />
                      </div>
                      <div className="flex items-center gap-2">
                        <span className="w-10 shrink-0 text-right font-mono text-[10px] text-slate-600">
                          0
                        </span>
                        <div className="flex-1 border-b border-slate-800" />
                      </div>
                    </div>

                    <div className="relative z-10 grid h-44 grid-cols-6 items-end gap-1 pl-11 sm:gap-2">
                      {months.map((month) => {
                        const incVal = Math.abs(Number(month.income) || 0);
                        const expVal = Math.abs(Number(month.expenses) || 0);
                        const incomeHeight = computeBarHeightPct(incVal, maxMonthValue);
                        const expenseHeight = computeBarHeightPct(expVal, maxMonthValue);
                        const isCurrentSelected = month.month === selectedMonth;
                        const isEmpty = incVal === 0 && expVal === 0;

                        return (
                          <button
                            key={month.month}
                            type="button"
                            onClick={() => setSelectedMonth(month.month)}
                            aria-pressed={isCurrentSelected}
                            title={`${formatMonthLong(month.month, language)} · ${t("dashboard.income")} ${formatMoney(incVal, currency)} · ${t("dashboard.expenses")} ${formatMoney(expVal, currency)}`}
                            className={`group flex h-full min-w-0 flex-col items-center justify-end rounded-xl px-1 pt-2 pb-1 transition ${
                              isCurrentSelected
                                ? "bg-emerald-400/12 ring-1 ring-inset ring-emerald-400/40"
                                : "hover:bg-slate-800/40"
                            }`}
                          >
                            <div className="flex h-full w-full items-end justify-center gap-1 px-0.5">
                              {isEmpty ? (
                                <div className="h-1.5 w-3 rounded-full bg-slate-800 transition group-hover:bg-slate-700" />
                              ) : (
                                <>
                                  {incVal > 0 && (
                                    <div
                                      className="w-2.5 sm:w-3 rounded-t-md bg-gradient-to-t from-emerald-500/80 to-emerald-300/95 shadow-[0_0_10px_rgba(52,211,153,0.18)] animate-bar-y"
                                      style={{ height: `${incomeHeight}%` }}
                                      title={`${t("dashboard.income")} ${formatMoney(month.income, currency)}`}
                                    />
                                  )}
                                  {expVal > 0 && (
                                    <div
                                      className="w-2.5 sm:w-3 rounded-t-md bg-gradient-to-t from-rose-500/80 to-rose-300/95 shadow-[0_0_10px_rgba(251,113,133,0.18)] animate-bar-y"
                                      style={{ height: `${expenseHeight}%` }}
                                      title={`${t("dashboard.expenses")} ${formatMoney(month.expenses, currency)}`}
                                    />
                                  )}
                                </>
                              )}
                            </div>

                            <span
                              className={`mt-2 block w-full truncate text-center text-[11px] font-medium leading-tight whitespace-nowrap ${
                                isCurrentSelected
                                  ? "font-semibold text-emerald-200"
                                  : "text-slate-400 group-hover:text-slate-200"
                              }`}
                            >
                              {formatMonthShort(month.month, language)}
                            </span>
                          </button>
                        );
                      })}
                    </div>
                  </div>
                </>
              )}
            </div>

            <div className="mt-3 flex items-center justify-between border-t border-slate-800/80 pt-2.5 text-[11px] text-slate-400">
              <span>{t("dashboard.savingsRate")}</span>
              <span className="font-mono font-medium text-slate-200">
                {savingsInfo.ratePct !== null
                  ? savingsInfo.ratePct >= 0
                    ? t("dashboard.savingsRateValue", { rate: savingsInfo.ratePct })
                    : t("dashboard.overspentBadge")
                  : t("dashboard.noIncomeRecorded")}
              </span>
            </div>
          </div>
        </section>

        {/* Decision Insights Section */}
        <section
          className="space-y-4 animate-fade-up stagger-3"
          aria-label={t("dashboard.decisionSectionTitle")}
        >
          <div>
            <h2 className="text-lg font-semibold tracking-tight text-slate-100">
              {t("dashboard.decisionSectionTitle")}
            </h2>
            <p className="mt-0.5 text-xs text-slate-400">
              {t("dashboard.decisionSectionSubtitle")}
            </p>
          </div>

          {/* Row 1: Monthly Burn Pace & Runway + Weekly Spending Rhythm */}
          <div className="grid grid-cols-1 gap-5 lg:grid-cols-12">
            {/* Card 1: Monthly Burn Pace & Runway (7 cols) */}
            <div className="surface-card flex flex-col justify-between rounded-2xl p-5 lg:col-span-7">
              <div>
                <div className="flex flex-wrap items-start justify-between gap-2">
                  <div>
                    <h3 className="font-medium text-slate-100">{t("dashboard.paceTitle")}</h3>
                    <p className="mt-0.5 text-xs text-slate-400">{t("dashboard.paceSubtitle")}</p>
                  </div>
                  {burnPace.peakDay && (
                    <span className="rounded-lg border border-amber-400/25 bg-amber-400/10 px-2.5 py-1 text-[11px] font-medium text-amber-200">
                      {t("dashboard.peakSpendDay")}:{" "}
                      {t("dashboard.dayOfMonth", { day: burnPace.peakDay.day })} (
                      {formatCompactMoney(burnPace.peakDay.amount, currency, language)})
                    </span>
                  )}
                </div>

                {/* 3 Decision KPIs for Burn Pace */}
                <div className="mt-4 grid grid-cols-1 gap-2.5 sm:grid-cols-3">
                  <div className="rounded-xl border border-slate-800/80 bg-slate-950/60 p-3">
                    <p className="text-[11px] text-slate-400">{t("dashboard.dailyBurnRate")}</p>
                    <p className="mt-1 font-mono text-base font-semibold tabular-nums text-slate-100">
                      {t("dashboard.perDay", {
                        amount: formatCompactMoney(burnPace.dailyBurnRate, currency, language),
                      })}
                    </p>
                  </div>
                  <div className="rounded-xl border border-slate-800/80 bg-slate-950/60 p-3">
                    <p className="text-[11px] text-slate-400">{t("dashboard.projectedMonthEnd")}</p>
                    <p
                      className={`mt-1 font-mono text-base font-semibold tabular-nums ${
                        burnPace.income > 0 && burnPace.projectedMonthEnd > burnPace.income
                          ? "text-rose-300"
                          : "text-amber-200"
                      }`}
                    >
                      {formatMoney(burnPace.projectedMonthEnd, currency)}
                    </p>
                  </div>
                  <div className="rounded-xl border border-slate-800/80 bg-slate-950/60 p-3">
                    <p className="text-[11px] text-slate-400">{t("dashboard.safeDailyBudget")}</p>
                    <p className="mt-1 font-mono text-base font-semibold tabular-nums text-emerald-300">
                      {burnPace.safeDailyAllowance !== null
                        ? t("dashboard.perDay", {
                            amount: formatCompactMoney(
                              burnPace.safeDailyAllowance,
                              currency,
                              language,
                            ),
                          })
                        : burnPace.income > 0
                          ? t("dashboard.overspentBadge")
                          : "—"}
                    </p>
                  </div>
                </div>

                {/* Cumulative Burn Trajectory SVG */}
                <BurnPaceChart pace={burnPace} currency={currency} language={language} />
              </div>

              {/* Legend */}
              <div className="mt-3 flex flex-wrap items-center justify-between gap-3 border-t border-slate-800/80 pt-3 text-[11px] text-slate-400">
                <div className="flex flex-wrap items-center gap-4">
                  <span className="inline-flex items-center gap-1.5">
                    <span className="h-0.5 w-3.5 rounded-full bg-emerald-400" />
                    {t("dashboard.paceActual")}
                  </span>
                  {burnPace.isCurrentMonth && (
                    <span className="inline-flex items-center gap-1.5">
                      <span className="h-0.5 w-3.5 border-b border-dashed border-amber-300" />
                      {t("dashboard.paceProjected")}
                    </span>
                  )}
                  {burnPace.income > 0 && (
                    <span className="inline-flex items-center gap-1.5">
                      <span className="h-0.5 w-3.5 border-b border-dotted border-emerald-300" />
                      {t("dashboard.paceIncomeCeiling")}
                    </span>
                  )}
                </div>
                <span className="font-mono text-slate-500">
                  {burnPace.elapsedDays}/{burnPace.daysInMonth}d
                </span>
              </div>
            </div>

            {/* Card 2: Weekly Spending Rhythm (5 cols) */}
            <div className="surface-card flex flex-col justify-between rounded-2xl p-5 lg:col-span-5">
              <div>
                <div className="flex flex-wrap items-start justify-between gap-2">
                  <div>
                    <h3 className="font-medium text-slate-100">{t("dashboard.weekdayTitle")}</h3>
                    <p className="mt-0.5 text-xs text-slate-400">
                      {t("dashboard.weekdaySubtitle")}
                    </p>
                  </div>
                  {weekdayRhythm.peakIndex !== null && (
                    <span className="rounded-lg border border-emerald-400/25 bg-emerald-400/10 px-2.5 py-1 text-[11px] font-medium text-emerald-200">
                      {t("dashboard.heaviestDay")}: {weekdayLabels[weekdayRhythm.peakIndex]}
                    </span>
                  )}
                </div>

                {weekdayRhythm.totalSpend <= 0 ? (
                  <p className="mt-10 py-8 text-center text-sm text-slate-500">
                    {t("dashboard.noExpensesThisMonth")}
                  </p>
                ) : (
                  <>
                    {/* 7-Day Vertical Bar Plot */}
                    <div className="mt-5 grid h-40 grid-cols-7 items-end gap-1.5 sm:gap-2 rounded-xl border border-slate-800/80 bg-slate-950/55 px-2.5 pt-4 pb-2.5">
                      {weekdayRhythm.buckets.map((b) => {
                        const maxDayTotal = Math.max(
                          1,
                          ...weekdayRhythm.buckets.map((x) => x.total),
                        );
                        const barH = computeBarHeightPct(b.total, maxDayTotal);
                        const label = weekdayLabels[b.weekdayIndex];

                        return (
                          <div
                            key={b.weekdayIndex}
                            className="flex h-full min-w-0 flex-col items-center justify-end"
                            title={`${label}: ${formatMoney(b.total, currency)} (${b.sharePct}%)`}
                          >
                            <span className="mb-1 font-mono text-[10px] tabular-nums text-slate-400">
                              {b.sharePct > 0 ? `${b.sharePct}%` : ""}
                            </span>
                            <div className="flex h-full w-full items-end justify-center">
                              {b.total > 0 ? (
                                <div
                                  className={`w-full max-w-6 rounded-t-md animate-bar-y ${
                                    b.isPeak
                                      ? "bg-gradient-to-t from-amber-500/80 to-amber-300 shadow-[0_0_12px_rgba(251,191,36,0.25)]"
                                      : b.isWeekend
                                        ? "bg-gradient-to-t from-sky-500/75 to-sky-300/90"
                                        : "bg-gradient-to-t from-emerald-500/75 to-emerald-300/90"
                                  }`}
                                  style={{ height: `${barH}%` }}
                                />
                              ) : (
                                <div className="h-1 w-3 rounded-full bg-slate-800" />
                              )}
                            </div>
                            <span
                              className={`mt-2 block w-full truncate text-center text-[11px] font-medium whitespace-nowrap ${
                                b.isPeak
                                  ? "font-semibold text-amber-200"
                                  : b.isWeekend
                                    ? "text-sky-300"
                                    : "text-slate-400"
                              }`}
                            >
                              {label}
                            </span>
                          </div>
                        );
                      })}
                    </div>

                    {/* Weekday vs Weekend Split Bar */}
                    <div className="mt-4 space-y-2">
                      <div className="flex flex-wrap items-center justify-between gap-2 text-xs">
                        <span className="flex items-center gap-1.5 text-slate-300">
                          <span className="size-2 rounded-full bg-emerald-300" />
                          {t("dashboard.weekdayShare")} ·{" "}
                          <strong className="font-mono">{weekdayRhythm.weekdaySharePct}%</strong>
                        </span>
                        <span className="flex items-center gap-1.5 text-slate-300">
                          <span className="size-2 rounded-full bg-sky-300" />
                          {t("dashboard.weekendShare")} ·{" "}
                          <strong className="font-mono">{weekdayRhythm.weekendSharePct}%</strong>
                        </span>
                      </div>
                      <div className="flex h-2 w-full overflow-hidden rounded-full bg-slate-950">
                        <div
                          className="h-full bg-emerald-400 transition-all duration-500"
                          style={{ width: `${weekdayRhythm.weekdaySharePct}%` }}
                        />
                        <div
                          className="h-full bg-sky-400 transition-all duration-500"
                          style={{ width: `${weekdayRhythm.weekendSharePct}%` }}
                        />
                      </div>
                      <p className="text-[11px] text-slate-400">
                        {weekdayRhythm.weekendSharePct >= 45
                          ? t("dashboard.weekendHeavyCallout", {
                              share: weekdayRhythm.weekendSharePct,
                            })
                          : t("dashboard.weekdaySteadyCallout", {
                              share: weekdayRhythm.weekdaySharePct,
                            })}
                      </p>
                    </div>
                  </>
                )}
              </div>
            </div>
          </div>

          {/* Row 2: Top Outflow Drivers + Liquidity & Balance Sheet Mix */}
          <div className="grid grid-cols-1 gap-5 lg:grid-cols-12">
            {/* Card 3: Top Outflow Drivers (6 cols) */}
            <div className="surface-card flex flex-col justify-between rounded-2xl p-5 lg:col-span-6">
              <div>
                <h3 className="font-medium text-slate-100">{t("dashboard.topMerchantsTitle")}</h3>
                <p className="mt-0.5 text-xs text-slate-400">
                  {t("dashboard.topMerchantsSubtitle")}
                </p>

                {topMerchants.length === 0 ? (
                  <p className="mt-8 py-6 text-center text-sm text-slate-500">
                    {t("dashboard.noExpensesThisMonth")}
                  </p>
                ) : (
                  <ul className="mt-4 space-y-2.5">
                    {topMerchants.map((merchant) => (
                      <li
                        key={merchant.name}
                        className="rounded-xl border border-slate-800/80 bg-slate-950/55 p-3"
                      >
                        <div className="flex items-center justify-between gap-2 text-sm">
                          <div className="flex min-w-0 items-center gap-2">
                            <span className="shrink-0 text-base">
                              {merchant.categoryIcon || "🧾"}
                            </span>
                            <span className="truncate font-medium text-slate-100">
                              {merchant.name}
                            </span>
                            {merchant.count > 1 && (
                              <span className="shrink-0 rounded bg-sky-400/15 px-1.5 py-0.5 text-[10px] font-medium text-sky-300">
                                {t("dashboard.repeatBadge", { count: merchant.count })}
                              </span>
                            )}
                          </div>
                          <span className="shrink-0 font-mono text-sm font-semibold tabular-nums text-slate-100">
                            −{formatMoney(merchant.total, currency)}
                          </span>
                        </div>

                        <div className="mt-2 flex items-center gap-3">
                          <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-slate-900">
                            <div
                              className="h-full rounded-full bg-gradient-to-r from-emerald-400 to-teal-300 animate-bar-x"
                              style={{ width: `${Math.min(100, Math.max(5, merchant.sharePct))}%` }}
                            />
                          </div>
                          <span className="shrink-0 font-mono text-[11px] tabular-nums text-slate-400">
                            {merchant.sharePct}%
                          </span>
                        </div>
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            </div>

            {/* Card 4: Liquidity & Balance Sheet Mix (6 cols) */}
            <div className="surface-card flex flex-col justify-between rounded-2xl p-5 lg:col-span-6">
              <div>
                <div className="flex flex-wrap items-start justify-between gap-2">
                  <div>
                    <h3 className="font-medium text-slate-100">{t("dashboard.liquidityTitle")}</h3>
                    <p className="mt-0.5 text-xs text-slate-400">
                      {t("dashboard.liquiditySubtitle")}
                    </p>
                  </div>
                  <span
                    className={`rounded-lg border px-2.5 py-1 text-[11px] font-medium ${
                      liquidity.liabilitiesDebt > 0
                        ? "border-rose-400/25 bg-rose-400/10 text-rose-200"
                        : "border-emerald-400/25 bg-emerald-400/10 text-emerald-200"
                    }`}
                  >
                    {liquidity.liabilitiesDebt > 0
                      ? t("dashboard.debtRatio", { ratio: liquidity.debtToAssetRatioPct })
                      : t("dashboard.debtFree")}
                  </span>
                </div>

                {/* 3 Pool Summary Boxes */}
                <div className="mt-4 grid grid-cols-1 gap-2.5 sm:grid-cols-3">
                  <div className="rounded-xl border border-emerald-400/20 bg-emerald-400/5 p-3">
                    <p className="text-[11px] text-emerald-200/80">{t("dashboard.liquidCash")}</p>
                    <p className="mt-1 truncate font-mono text-base font-semibold tabular-nums text-emerald-300">
                      {formatMoney(liquidity.liquidCash, currency)}
                    </p>
                  </div>
                  <div className="rounded-xl border border-sky-400/20 bg-sky-400/5 p-3">
                    <p className="text-[11px] text-sky-200/80">{t("dashboard.investments")}</p>
                    <p className="mt-1 truncate font-mono text-base font-semibold tabular-nums text-sky-300">
                      {formatMoney(liquidity.investments, currency)}
                    </p>
                  </div>
                  <div className="rounded-xl border border-rose-400/20 bg-rose-400/5 p-3">
                    <p className="text-[11px] text-rose-200/80">{t("dashboard.liabilitiesDebt")}</p>
                    <p className="mt-1 truncate font-mono text-base font-semibold tabular-nums text-rose-300">
                      {formatMoney(liquidity.liabilitiesDebt, currency)}
                    </p>
                  </div>
                </div>

                {/* Stacked Allocation Bar */}
                <div className="mt-4 flex h-2.5 w-full overflow-hidden rounded-full bg-slate-950 p-0.5 ring-1 ring-inset ring-slate-800">
                  {liquidity.liquidSharePct > 0 && (
                    <div
                      className="h-full bg-emerald-400 first:rounded-l-full last:rounded-r-full"
                      style={{ width: `${liquidity.liquidSharePct}%` }}
                      title={`${t("dashboard.liquidCash")}: ${liquidity.liquidSharePct}%`}
                    />
                  )}
                  {liquidity.investmentSharePct > 0 && (
                    <div
                      className="h-full bg-sky-400 first:rounded-l-full last:rounded-r-full"
                      style={{ width: `${liquidity.investmentSharePct}%` }}
                      title={`${t("dashboard.investments")}: ${liquidity.investmentSharePct}%`}
                    />
                  )}
                  {liquidity.debtSharePct > 0 && (
                    <div
                      className="h-full bg-rose-400 first:rounded-l-full last:rounded-r-full"
                      style={{ width: `${liquidity.debtSharePct}%` }}
                      title={`${t("dashboard.liabilitiesDebt")}: ${liquidity.debtSharePct}%`}
                    />
                  )}
                </div>

                {/* Active Accounts Mini Distribution */}
                <ul className="mt-4 space-y-2">
                  {accounts
                    .filter((a) => a.is_active)
                    .slice(0, 4)
                    .map((acc) => {
                      const bal = Number(acc.balance_current) || 0;
                      const isDebt = acc.type === "credit_card" || acc.type === "loan" || bal < 0;
                      return (
                        <li
                          key={acc.id}
                          className="flex items-center justify-between gap-2 rounded-lg bg-slate-950/55 px-3 py-2 text-xs"
                        >
                          <div className="flex min-w-0 items-center gap-2">
                            <span
                              className={`size-2 shrink-0 rounded-full ${
                                isDebt
                                  ? "bg-rose-400"
                                  : acc.type === "investment"
                                    ? "bg-sky-400"
                                    : "bg-emerald-400"
                              }`}
                            />
                            <span className="truncate font-medium text-slate-200">{acc.name}</span>
                            <span className="shrink-0 text-[11px] text-slate-500">
                              {tAccountType(acc.type)}
                            </span>
                          </div>
                          <span
                            className={`shrink-0 font-mono font-semibold tabular-nums ${
                              isDebt ? "text-rose-300" : "text-slate-100"
                            }`}
                          >
                            {formatMoney(bal, acc.currency)}
                          </span>
                        </li>
                      );
                    })}
                </ul>
              </div>
            </div>
          </div>
        </section>

        {/* Recent Transactions */}
        <section className="surface-card rounded-2xl p-5 animate-fade-up stagger-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <h2 className="font-medium text-slate-100">{t("dashboard.recentTransactions")}</h2>
            <Link to="/transactions" className="text-sm text-emerald-400 hover:text-emerald-300">
              {t("dashboard.viewAll")}
            </Link>
          </div>
          {recent.length === 0 ? (
            <p className="mt-4 text-sm text-slate-500">{t("dashboard.nothingRecordedYet")}</p>
          ) : (
            <ul className="mt-3 divide-y divide-slate-800/80">
              {recent.map((transaction) => (
                <li
                  key={transaction.id}
                  className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 py-3.5 text-sm"
                >
                  <div className="flex min-w-0 items-center gap-3">
                    <span className="flex size-9 shrink-0 items-center justify-center rounded-xl border border-slate-800 bg-slate-950/70 text-base">
                      {transaction.category_icon || (transaction.direction === "in" ? "↗" : "↙")}
                    </span>
                    <div className="min-w-0">
                      <div className="truncate font-medium text-slate-100">
                        {transaction.description}
                      </div>
                      <div className="break-words text-xs text-slate-500">
                        {formatDate(transaction.occurred_at)} · {transaction.account_name}
                        {transaction.category_name
                          ? ` · ${tCategory(transaction.category_slug, transaction.category_name)}`
                          : ""}
                      </div>
                    </div>
                  </div>
                  <span
                    className={`max-w-full break-words font-mono font-semibold tabular-nums [overflow-wrap:anywhere] ${
                      transaction.direction === "in" ? "text-emerald-400" : "text-rose-400"
                    }`}
                  >
                    {formatSigned(
                      transaction.direction === "in"
                        ? Math.abs(Number(transaction.amount))
                        : -Math.abs(Number(transaction.amount)),
                      transaction.currency,
                    )}
                  </span>
                </li>
              ))}
            </ul>
          )}
        </section>
      </div>
    </AppLayout>
  );
}

function MetricCard({
  label,
  value,
  tone,
  badge,
}: {
  label: string;
  value: string;
  tone: "positive" | "negative" | "neutral";
  badge?: string;
}) {
  const toneClass =
    tone === "positive"
      ? "text-emerald-300"
      : tone === "negative"
        ? "text-rose-300"
        : "text-slate-100";

  const glowColor =
    tone === "positive"
      ? "from-emerald-400/12"
      : tone === "negative"
        ? "from-rose-400/12"
        : "from-emerald-400/8";

  return (
    <div
      className={`surface-card relative min-w-0 overflow-hidden rounded-2xl bg-gradient-to-br ${glowColor} via-transparent to-transparent p-4 sm:p-5`}
    >
      <div className="flex items-center justify-between gap-2">
        <div className="truncate text-xs font-medium uppercase tracking-wide text-slate-400">
          {label}
        </div>
        {badge && (
          <span className="shrink-0 rounded-full border border-slate-700/80 bg-slate-950/70 px-2 py-0.5 text-[10px] font-medium text-slate-300">
            {badge}
          </span>
        )}
      </div>
      <div
        className={`mt-2.5 truncate text-xl font-semibold tracking-tight tabular-nums sm:text-2xl ${toneClass}`}
        title={value}
      >
        {value}
      </div>
    </div>
  );
}

function BurnPaceChart({
  pace,
  currency,
  language,
}: {
  pace: ReturnType<typeof computeMonthlyBurnPace>;
  currency: string;
  language: "en" | "es" | "ja";
}) {
  const width = 560;
  const height = 148;
  const padLeft = 8;
  const padRight = 8;
  const padTop = 14;
  const padBottom = 22;
  const plotW = width - padLeft - padRight;
  const plotH = height - padTop - padBottom;

  const maxY = Math.max(
    1,
    pace.totalSpent,
    pace.projectedMonthEnd,
    pace.income > 0 ? pace.income * 1.05 : 0,
  );

  const xForDay = (day: number) => {
    if (pace.daysInMonth <= 1) return padLeft;
    return padLeft + ((day - 1) / (pace.daysInMonth - 1)) * plotW;
  };

  const yForValue = (val: number) => {
    const clamped = Math.max(0, Math.min(maxY, val));
    return padTop + plotH - (clamped / maxY) * plotH;
  };

  const actualPoints = pace.points.filter((p) => p.cumulativeSpend !== null);
  const projectedPoints = pace.points.filter((p) => p.projectedSpend !== null);

  const actualLine = actualPoints
    .map(
      (p, idx) =>
        `${idx === 0 ? "M" : "L"}${xForDay(p.day).toFixed(1)},${yForValue(p.cumulativeSpend!).toFixed(1)}`,
    )
    .join(" ");

  const actualArea =
    actualPoints.length > 0
      ? `${actualLine} L${xForDay(actualPoints[actualPoints.length - 1].day).toFixed(1)},${(padTop + plotH).toFixed(1)} L${xForDay(actualPoints[0].day).toFixed(1)},${(padTop + plotH).toFixed(1)} Z`
      : "";

  const projectedLine = projectedPoints
    .map(
      (p, idx) =>
        `${idx === 0 ? "M" : "L"}${xForDay(p.day).toFixed(1)},${yForValue(p.projectedSpend!).toFixed(1)}`,
    )
    .join(" ");

  const incomeY = pace.income > 0 ? yForValue(pace.income) : null;
  const lastActual = actualPoints[actualPoints.length - 1];

  return (
    <div className="mt-4 rounded-xl border border-slate-800/80 bg-slate-950/55 p-3">
      <svg
        viewBox={`0 0 ${width} ${height}`}
        className="h-36 w-full overflow-visible"
        role="img"
        aria-label="Monthly spending trajectory"
      >
        <defs>
          <linearGradient id="burnAreaGrad" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="#9bc386" stopOpacity="0.32" />
            <stop offset="100%" stopColor="#9bc386" stopOpacity="0.0" />
          </linearGradient>
        </defs>

        <line
          x1={padLeft}
          y1={padTop + plotH / 2}
          x2={width - padRight}
          y2={padTop + plotH / 2}
          stroke="rgba(255,255,255,0.06)"
          strokeDasharray="3 3"
        />
        <line
          x1={padLeft}
          y1={padTop + plotH}
          x2={width - padRight}
          y2={padTop + plotH}
          stroke="rgba(255,255,255,0.12)"
        />

        {incomeY !== null && (
          <g>
            <line
              x1={padLeft}
              y1={incomeY}
              x2={width - padRight}
              y2={incomeY}
              stroke="#34d399"
              strokeWidth="1.25"
              strokeDasharray="2 4"
              opacity="0.75"
            />
            <text
              x={width - padRight - 2}
              y={Math.max(10, incomeY - 4)}
              textAnchor="end"
              className="fill-emerald-300/80 font-mono text-[9px]"
            >
              {formatCompactMoney(pace.income, currency, language)}
            </text>
          </g>
        )}

        {actualArea && <path d={actualArea} fill="url(#burnAreaGrad)" />}
        {actualLine && (
          <path
            d={actualLine}
            fill="none"
            stroke="#9bc386"
            strokeWidth="2.25"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        )}

        {projectedLine && projectedPoints.length > 1 && (
          <path
            d={projectedLine}
            fill="none"
            stroke="#fbbf24"
            strokeWidth="1.75"
            strokeDasharray="4 4"
            strokeLinecap="round"
          />
        )}

        {lastActual && (
          <g>
            <circle
              cx={xForDay(lastActual.day)}
              cy={yForValue(lastActual.cumulativeSpend!)}
              r="4"
              className="fill-emerald-300"
            />
            <circle
              cx={xForDay(lastActual.day)}
              cy={yForValue(lastActual.cumulativeSpend!)}
              r="8"
              className="fill-emerald-300/20"
            />
          </g>
        )}

        <text
          x={xForDay(1)}
          y={height - 4}
          textAnchor="start"
          className="fill-slate-500 font-mono text-[10px]"
        >
          1
        </text>
        <text
          x={xForDay(Math.round(pace.daysInMonth / 2))}
          y={height - 4}
          textAnchor="middle"
          className="fill-slate-500 font-mono text-[10px]"
        >
          {Math.round(pace.daysInMonth / 2)}
        </text>
        <text
          x={xForDay(pace.daysInMonth)}
          y={height - 4}
          textAnchor="end"
          className="fill-slate-500 font-mono text-[10px]"
        >
          {pace.daysInMonth}
        </text>
      </svg>
    </div>
  );
}
