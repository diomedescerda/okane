import type { Account, CategoryTotal, MonthTotal, Transaction } from "../api/finance";

export interface CategoryParetoItem {
  key: string;
  slug?: string;
  name: string;
  icon: string;
  color: string;
  total: number;
  count: number;
  avgPerTransaction: number;
  sharePct: number;
  cumulativePct: number;
  isVitalFew: boolean;
}

export interface CategoryParetoResult {
  items: CategoryParetoItem[];
  totalExpenses: number;
  vitalFewCount: number;
  vitalFewSharePct: number;
}

const FALLBACK_COLORS = [
  "#9bc386",
  "#38bdf8",
  "#f59e0b",
  "#f43f5e",
  "#a78bfa",
  "#2dd4bf",
  "#fb923c",
  "#e879f9",
];

/**
 * Computes category share, average ticket, and 80/20 Pareto "Vital Few" flags.
 */
export function computeCategoryPareto(categories: CategoryTotal[]): CategoryParetoResult {
  const expenses = categories
    .filter((c) => c.direction === "out" && Math.abs(Number(c.total)) > 0)
    .map((c) => ({
      ...c,
      absTotal: Math.abs(Number(c.total)),
    }))
    .sort((a, b) => b.absTotal - a.absTotal);

  const totalExpenses = expenses.reduce((sum, c) => sum + c.absTotal, 0);
  if (totalExpenses <= 0) {
    return { items: [], totalExpenses: 0, vitalFewCount: 0, vitalFewSharePct: 0 };
  }

  let running = 0;
  let vitalFewCount = 0;
  let vitalFewShareRaw = 0;

  const items: CategoryParetoItem[] = expenses.map((c, idx) => {
    const prevCumulative = (running / totalExpenses) * 100;
    running += c.absTotal;
    const shareRaw = (c.absTotal / totalExpenses) * 100;
    const cumulativeRaw = (running / totalExpenses) * 100;
    // Categories that bring cumulative spend up to the 80% threshold (at least 1).
    const isVitalFew = idx === 0 || prevCumulative < 78;
    if (isVitalFew) {
      vitalFewCount += 1;
      vitalFewShareRaw = cumulativeRaw;
    }

    return {
      key: c.category_id ?? c.slug ?? `cat-${idx}`,
      slug: c.slug,
      name: c.name,
      icon: c.icon || "🏷️",
      color: c.color || FALLBACK_COLORS[idx % FALLBACK_COLORS.length],
      total: c.absTotal,
      count: Math.max(1, c.count),
      avgPerTransaction: c.absTotal / Math.max(1, c.count),
      sharePct: Math.max(1, Math.round(shareRaw)),
      cumulativePct: Math.min(100, Math.round(cumulativeRaw)),
      isVitalFew,
    };
  });

  return {
    items,
    totalExpenses,
    vitalFewCount,
    vitalFewSharePct: Math.min(100, Math.round(vitalFewShareRaw)),
  };
}

export interface DailyBurnPoint {
  day: number;
  dailySpend: number;
  cumulativeSpend: number | null;
  projectedSpend: number | null;
}

export interface MonthlyBurnPace {
  daysInMonth: number;
  elapsedDays: number;
  isCurrentMonth: boolean;
  totalSpent: number;
  income: number;
  dailyBurnRate: number;
  projectedMonthEnd: number;
  safeDailyAllowance: number | null;
  peakDay: { day: number; amount: number } | null;
  points: DailyBurnPoint[];
}

/**
 * Computes day-by-day spending trajectory, daily burn rate, projected month-end total,
 * and remaining safe daily allowance for a given "YYYY-MM" month.
 */
export function computeMonthlyBurnPace(
  month: string,
  transactions: Transaction[],
  monthlyIncome: number,
  now: Date = new Date(),
): MonthlyBurnPace {
  const [y, m] = month.split("-").map(Number);
  const year = y || now.getFullYear();
  const monthIdx = (m || now.getMonth() + 1) - 1;
  const daysInMonth = new Date(year, monthIdx + 1, 0).getDate();

  const currentYM = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}`;
  const isCurrentMonth = month === currentYM;
  const elapsedDays = isCurrentMonth
    ? Math.min(daysInMonth, Math.max(1, now.getDate()))
    : daysInMonth;

  const dailyTotals = new Array<number>(daysInMonth).fill(0);

  for (const tx of transactions) {
    if (tx.direction !== "out") continue;
    const d = new Date(tx.occurred_at);
    if (Number.isNaN(d.getTime())) continue;
    if (d.getFullYear() === year && d.getMonth() === monthIdx) {
      const dayIdx = d.getDate() - 1;
      if (dayIdx >= 0 && dayIdx < daysInMonth) {
        dailyTotals[dayIdx] += Math.abs(Number(tx.amount) || 0);
      }
    }
  }

  let peakDay: { day: number; amount: number } | null = null;
  let running = 0;
  for (let i = 0; i < daysInMonth; i++) {
    const amt = dailyTotals[i];
    if (amt > 0 && (!peakDay || amt > peakDay.amount)) {
      peakDay = { day: i + 1, amount: amt };
    }
    if (i < elapsedDays) {
      running += amt;
    }
  }

  const totalSpent = dailyTotals.reduce((a, b) => a + b, 0);
  const dailyBurnRate = elapsedDays > 0 ? running / elapsedDays : 0;
  const projectedMonthEnd = isCurrentMonth
    ? running + dailyBurnRate * (daysInMonth - elapsedDays)
    : totalSpent;

  const remainingDays = Math.max(1, daysInMonth - elapsedDays);
  const safeDailyAllowance =
    monthlyIncome > totalSpent ? (monthlyIncome - totalSpent) / remainingDays : null;

  let cum = 0;
  const points: DailyBurnPoint[] = dailyTotals.map((amt, idx) => {
    const day = idx + 1;
    cum += amt;
    if (!isCurrentMonth || day <= elapsedDays) {
      return {
        day,
        dailySpend: amt,
        cumulativeSpend: cum,
        projectedSpend: day === elapsedDays && isCurrentMonth ? cum : null,
      };
    }
    return {
      day,
      dailySpend: 0,
      cumulativeSpend: null,
      projectedSpend: running + dailyBurnRate * (day - elapsedDays),
    };
  });

  return {
    daysInMonth,
    elapsedDays,
    isCurrentMonth,
    totalSpent,
    income: monthlyIncome,
    dailyBurnRate,
    projectedMonthEnd,
    safeDailyAllowance,
    peakDay,
    points,
  };
}

export interface WeekdayBucket {
  /** 0 = Monday .. 6 = Sunday */
  weekdayIndex: number;
  total: number;
  count: number;
  sharePct: number;
  isWeekend: boolean;
  isPeak: boolean;
}

export interface WeekdayRhythmResult {
  buckets: WeekdayBucket[];
  weekendSharePct: number;
  weekdaySharePct: number;
  peakIndex: number | null;
  totalSpend: number;
}

/**
 * Computes Mon..Sun spending distribution and weekday vs. weekend (Fri..Sun) share.
 */
export function computeWeekdayRhythm(transactions: Transaction[]): WeekdayRhythmResult {
  const totals = new Array<number>(7).fill(0);
  const counts = new Array<number>(7).fill(0);

  for (const tx of transactions) {
    if (tx.direction !== "out") continue;
    const d = new Date(tx.occurred_at);
    if (Number.isNaN(d.getTime())) continue;
    // JS getDay(): 0 = Sun, 1 = Mon .. 6 = Sat -> convert to 0 = Mon .. 6 = Sun
    const monIdx = (d.getDay() + 6) % 7;
    const amt = Math.abs(Number(tx.amount) || 0);
    if (amt > 0) {
      totals[monIdx] += amt;
      counts[monIdx] += 1;
    }
  }

  const totalSpend = totals.reduce((a, b) => a + b, 0);
  let peakIndex: number | null = null;
  for (let i = 0; i < 7; i++) {
    if (totals[i] > 0 && (peakIndex === null || totals[i] > totals[peakIndex])) {
      peakIndex = i;
    }
  }

  // Fri (4), Sat (5), Sun (6) are weekend social spend days
  const weekendSpend = totals[4] + totals[5] + totals[6];
  const weekendSharePct = totalSpend > 0 ? Math.round((weekendSpend / totalSpend) * 100) : 0;
  const weekdaySharePct = totalSpend > 0 ? 100 - weekendSharePct : 0;

  const buckets: WeekdayBucket[] = totals.map((tot, idx) => ({
    weekdayIndex: idx,
    total: tot,
    count: counts[idx],
    sharePct: totalSpend > 0 ? Math.round((tot / totalSpend) * 100) : 0,
    isWeekend: idx >= 4,
    isPeak: idx === peakIndex,
  }));

  return {
    buckets,
    weekendSharePct,
    weekdaySharePct,
    peakIndex,
    totalSpend,
  };
}

export interface TopMerchantItem {
  name: string;
  total: number;
  count: number;
  sharePct: number;
  categorySlug?: string;
  categoryName?: string;
  categoryIcon?: string;
}

/**
 * Groups outgoing transactions by merchant (or normalized description) to surface top payees.
 */
export function computeTopMerchants(
  transactions: Transaction[],
  limit = 5,
): TopMerchantItem[] {
  const map = new Map<
    string,
    {
      displayName: string;
      total: number;
      count: number;
      categorySlug?: string;
      categoryName?: string;
      categoryIcon?: string;
    }
  >();

  let grandTotal = 0;
  for (const tx of transactions) {
    if (tx.direction !== "out") continue;
    const amt = Math.abs(Number(tx.amount) || 0);
    if (amt <= 0) continue;
    grandTotal += amt;

    const rawName = (tx.merchant || tx.description || "").trim();
    if (!rawName) continue;
    const key = rawName.toLowerCase();
    const existing = map.get(key);
    if (existing) {
      existing.total += amt;
      existing.count += 1;
    } else {
      map.set(key, {
        displayName: rawName,
        total: amt,
        count: 1,
        categorySlug: tx.category_slug,
        categoryName: tx.category_name,
        categoryIcon: tx.category_icon,
      });
    }
  }

  if (grandTotal <= 0) return [];

  return Array.from(map.values())
    .sort((a, b) => b.total - a.total)
    .slice(0, limit)
    .map((item) => ({
      name: item.displayName,
      total: item.total,
      count: item.count,
      sharePct: Math.max(1, Math.round((item.total / grandTotal) * 100)),
      categorySlug: item.categorySlug,
      categoryName: item.categoryName,
      categoryIcon: item.categoryIcon,
    }));
}

export interface LiquiditySummary {
  liquidCash: number;
  investments: number;
  liabilitiesDebt: number;
  totalAssets: number;
  debtToAssetRatioPct: number;
  liquidSharePct: number;
  investmentSharePct: number;
  debtSharePct: number;
}

/**
 * Computes liquid cash vs. investments vs. credit card / loan debt across active accounts.
 */
export function computeLiquiditySummary(accounts: Account[]): LiquiditySummary {
  let liquidCash = 0;
  let investments = 0;
  let liabilitiesDebt = 0;

  for (const acc of accounts) {
    if (!acc.is_active) continue;
    const bal = Number(acc.balance_current) || 0;
    if (acc.type === "credit_card" || acc.type === "loan" || bal < 0) {
      liabilitiesDebt += Math.abs(bal);
    } else if (acc.type === "investment") {
      investments += bal;
    } else {
      liquidCash += bal;
    }
  }

  const totalAssets = liquidCash + investments;
  const grandPool = Math.max(1, totalAssets + liabilitiesDebt);
  const debtToAssetRatioPct =
    totalAssets > 0 ? Math.round((liabilitiesDebt / totalAssets) * 100) : liabilitiesDebt > 0 ? 100 : 0;

  return {
    liquidCash,
    investments,
    liabilitiesDebt,
    totalAssets,
    debtToAssetRatioPct,
    liquidSharePct: Math.round((liquidCash / grandPool) * 100),
    investmentSharePct: Math.round((investments / grandPool) * 100),
    debtSharePct: Math.round((liabilitiesDebt / grandPool) * 100),
  };
}

/**
 * Computes visual bar height % (0..100) with a soft power curve so small non-zero values
 * remain clearly visible alongside large spikes (fixing the 1px sliver bar issue).
 */
export function computeBarHeightPct(value: number, maxValue: number): number {
  const abs = Math.abs(value);
  if (abs <= 0 || maxValue <= 0) return 0;
  const ratio = Math.min(1, abs / maxValue);
  // Blend linear (65%) + square-root (35%) and enforce a 9% floor for any non-zero bar.
  const blended = ratio * 0.65 + Math.sqrt(ratio) * 0.35;
  return Math.max(9, Math.round(blended * 100));
}

export function computeSavingsRate(month?: MonthTotal | { income: string | number; expenses: string | number }): {
  ratePct: number | null;
  isOverspent: boolean;
} {
  if (!month) return { ratePct: null, isOverspent: false };
  const inc = Math.abs(Number(month.income) || 0);
  const exp = Math.abs(Number(month.expenses) || 0);
  if (inc <= 0) {
    return { ratePct: null, isOverspent: exp > 0 };
  }
  const rate = Math.round(((inc - exp) / inc) * 100);
  return {
    ratePct: rate,
    isOverspent: exp > inc,
  };
}
