import assert from "node:assert/strict";
import test from "node:test";

import {
  computeBarHeightPct,
  computeCategoryPareto,
  computeLiquiditySummary,
  computeMonthlyBurnPace,
  computeSavingsRate,
  computeTopMerchants,
  computeWeekdayRhythm,
} from "../src/lib/analytics.ts";
import {
  formatCompactMoney,
  formatMonthShort,
  getWeekdayLabels,
} from "../src/lib/format.ts";

test("formatMonthShort returns single-line month labels across en, es, ja", () => {
  assert.equal(formatMonthShort("2026-06", "ja"), "6月");
  assert.equal(formatMonthShort("2026-10", "en"), "Oct");
  assert.match(formatMonthShort("2026-10", "es"), /oct/i);
});

test("formatCompactMoney formats large numbers compactly", () => {
  assert.match(formatCompactMoney(1500000, "COP", "en"), /1\.5M/);
  assert.match(formatCompactMoney(250000, "JPY", "ja"), /25万/);
});

test("getWeekdayLabels returns 7 Monday-first labels", () => {
  const enDays = getWeekdayLabels("en");
  const jaDays = getWeekdayLabels("ja");
  assert.equal(enDays.length, 7);
  assert.equal(jaDays.length, 7);
  assert.match(enDays[0], /^Mon/i);
  assert.match(enDays[6], /^Sun/i);
});

test("computeCategoryPareto calculates shares, average ticket, and 80/20 Vital Few", () => {
  const result = computeCategoryPareto([
    {
      category_id: "c1",
      slug: "rent",
      name: "Rent",
      icon: "🏠",
      color: "#ef4444",
      direction: "out",
      total: "600",
      count: 1,
    },
    {
      category_id: "c2",
      slug: "groceries",
      name: "Groceries",
      icon: "🛒",
      color: "#22c55e",
      direction: "out",
      total: "250",
      count: 5,
    },
    {
      category_id: "c3",
      slug: "coffee",
      name: "Coffee",
      icon: "☕",
      color: "#f59e0b",
      direction: "out",
      total: "150",
      count: 10,
    },
  ]);

  assert.equal(result.totalExpenses, 1000);
  assert.equal(result.items.length, 3);
  assert.equal(result.items[0].sharePct, 60);
  assert.equal(result.items[0].avgPerTransaction, 600);
  assert.equal(result.items[0].isVitalFew, true);

  assert.equal(result.items[1].sharePct, 25);
  assert.equal(result.items[1].avgPerTransaction, 50);
  assert.equal(result.items[1].isVitalFew, true);

  assert.equal(result.items[2].sharePct, 15);
  assert.equal(result.items[2].isVitalFew, false);

  assert.equal(result.vitalFewCount, 2);
  assert.equal(result.vitalFewSharePct, 85);
});

test("computeMonthlyBurnPace projects month-end spend and safe daily allowance", () => {
  const now = new Date(2026, 9, 10, 12, 0, 0); // Oct 10, 2026 (10 days elapsed of 31)
  const transactions = [
    {
      id: "t1",
      account_id: "a1",
      description: "Rent",
      amount: "500",
      direction: "out",
      currency: "USD",
      status: "posted",
      occurred_at: new Date(2026, 9, 2, 12, 0, 0).toISOString(),
      tags: [],
      account_name: "Checking",
      created_at: "",
      updated_at: "",
    },
    {
      id: "t2",
      account_id: "a1",
      description: "Market",
      amount: "500",
      direction: "out",
      currency: "USD",
      status: "posted",
      occurred_at: new Date(2026, 9, 5, 12, 0, 0).toISOString(),
      tags: [],
      account_name: "Checking",
      created_at: "",
      updated_at: "",
    },
  ];

  const pace = computeMonthlyBurnPace("2026-10", transactions, 3100, now);
  assert.equal(pace.daysInMonth, 31);
  assert.equal(pace.elapsedDays, 10);
  assert.equal(pace.isCurrentMonth, true);
  assert.equal(pace.totalSpent, 1000);
  assert.equal(pace.dailyBurnRate, 100);
  assert.equal(pace.projectedMonthEnd, 3100);
  // Remaining budget: 3100 - 1000 = 2100 over 21 remaining days -> 100/day
  assert.equal(pace.safeDailyAllowance, 100);
  assert.deepEqual(pace.peakDay, { day: 2, amount: 500 });
});

test("computeWeekdayRhythm separates weekday and weekend spending", () => {
  // 2026-01-05 is Monday, 2026-01-10 is Saturday
  const transactions = [
    {
      id: "t1",
      account_id: "a1",
      description: "Lunch",
      amount: "40",
      direction: "out",
      currency: "USD",
      status: "posted",
      occurred_at: new Date(2026, 0, 5, 12, 0, 0).toISOString(),
      tags: [],
      account_name: "Checking",
      created_at: "",
      updated_at: "",
    },
    {
      id: "t2",
      account_id: "a1",
      description: "Saturday Dinner",
      amount: "60",
      direction: "out",
      currency: "USD",
      status: "posted",
      occurred_at: new Date(2026, 0, 10, 20, 0, 0).toISOString(),
      tags: [],
      account_name: "Checking",
      created_at: "",
      updated_at: "",
    },
  ];

  const rhythm = computeWeekdayRhythm(transactions);
  assert.equal(rhythm.totalSpend, 100);
  assert.equal(rhythm.weekdaySharePct, 40);
  assert.equal(rhythm.weekendSharePct, 60);
  assert.equal(rhythm.peakIndex, 5); // Saturday (index 5)
});

test("computeTopMerchants ranks merchants and counts repeat visits", () => {
  const transactions = [
    {
      id: "1",
      account_id: "a1",
      description: "D1 Market",
      merchant: "D1",
      amount: "100",
      direction: "out",
      currency: "COP",
      status: "posted",
      occurred_at: "2026-10-01T12:00:00Z",
      tags: [],
      account_name: "Main",
      created_at: "",
      updated_at: "",
    },
    {
      id: "2",
      account_id: "a1",
      description: "D1 Market 2",
      merchant: "D1",
      amount: "150",
      direction: "out",
      currency: "COP",
      status: "posted",
      occurred_at: "2026-10-03T12:00:00Z",
      tags: [],
      account_name: "Main",
      created_at: "",
      updated_at: "",
    },
    {
      id: "3",
      account_id: "a1",
      description: "Rent",
      amount: "750",
      direction: "out",
      currency: "COP",
      status: "posted",
      occurred_at: "2026-10-01T12:00:00Z",
      tags: [],
      account_name: "Main",
      created_at: "",
      updated_at: "",
    },
  ];

  const top = computeTopMerchants(transactions, 5);
  assert.equal(top.length, 2);
  assert.equal(top[0].name, "Rent");
  assert.equal(top[0].total, 750);
  assert.equal(top[0].count, 1);
  assert.equal(top[0].sharePct, 75);

  assert.equal(top[1].name, "D1");
  assert.equal(top[1].total, 250);
  assert.equal(top[1].count, 2);
  assert.equal(top[1].sharePct, 25);
});

test("computeLiquiditySummary separates liquid cash, investments, and debt", () => {
  const summary = computeLiquiditySummary([
    {
      id: "1",
      user_id: "u1",
      provider: "manual",
      name: "Checking",
      type: "checking",
      currency: "COP",
      balance_current: "600",
      opening_balance: "0",
      institution_name: "Bank",
      is_active: true,
      transaction_count: 1,
      created_at: "",
      updated_at: "",
    },
    {
      id: "2",
      user_id: "u1",
      provider: "manual",
      name: "Brokerage",
      type: "investment",
      currency: "COP",
      balance_current: "400",
      opening_balance: "0",
      institution_name: "Broker",
      is_active: true,
      transaction_count: 1,
      created_at: "",
      updated_at: "",
    },
    {
      id: "3",
      user_id: "u1",
      provider: "manual",
      name: "Visa",
      type: "credit_card",
      currency: "COP",
      balance_current: "-200",
      opening_balance: "0",
      institution_name: "Bank",
      is_active: true,
      transaction_count: 1,
      created_at: "",
      updated_at: "",
    },
  ]);

  assert.equal(summary.liquidCash, 600);
  assert.equal(summary.investments, 400);
  assert.equal(summary.liabilitiesDebt, 200);
  assert.equal(summary.totalAssets, 1000);
  assert.equal(summary.debtToAssetRatioPct, 20);
});

test("computeBarHeightPct ensures small non-zero values remain visible (>= 9%)", () => {
  assert.equal(computeBarHeightPct(0, 1000), 0);
  assert.equal(computeBarHeightPct(1000, 1000), 100);
  const tinyHeight = computeBarHeightPct(5, 1000);
  assert.ok(tinyHeight >= 9, `expected tiny bar height >= 9, got ${tinyHeight}`);
});

test("computeSavingsRate computes savings rate and overspent state", () => {
  assert.deepEqual(computeSavingsRate({ income: "1000", expenses: "650" }), {
    ratePct: 35,
    isOverspent: false,
  });
  assert.deepEqual(computeSavingsRate({ income: "1000", expenses: "1200" }), {
    ratePct: -20,
    isOverspent: true,
  });
});
