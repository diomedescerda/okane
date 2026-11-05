import type { Language } from "./i18n";

const LOCALES: Record<Language, string> = {
  en: "en-US",
  es: "es-CO",
  ja: "ja-JP",
};

let activeLanguage: Language = "en";

export function setFormatLanguage(lang: Language): void {
  activeLanguage = lang;
}

export function getFormatLanguage(): Language {
  return activeLanguage;
}

function resolveLocale(lang?: Language): string {
  return LOCALES[lang ?? activeLanguage] ?? LOCALES.en;
}

const currencyFormatters = new Map<string, Intl.NumberFormat>();

function currencyFormatter(currency: string, lang?: Language): Intl.NumberFormat {
  const locale = resolveLocale(lang);
  const key = `${locale}:${currency}`;
  let formatter = currencyFormatters.get(key);
  if (!formatter) {
    // The Colombian peso and Japanese yen do not use decimal subunits in everyday amounts.
    const zeroDecimals = currency === "COP" || currency === "JPY";
    formatter = new Intl.NumberFormat(locale, {
      style: "currency",
      currency,
      currencyDisplay: "narrowSymbol",
      ...(zeroDecimals ? { minimumFractionDigits: 0, maximumFractionDigits: 0 } : {}),
    });
    currencyFormatters.set(key, formatter);
  }
  return formatter;
}

/** Formats a decimal string (e.g. "-1234.50") as currency. */
export function formatMoney(value: string | number, currency = "COP", lang?: Language): string {
  const n = typeof value === "number" ? value : Number(value);
  if (!Number.isFinite(n)) return "—";
  return currencyFormatter(currency, lang).format(n);
}

/** Formats a signed amount with an explicit + / − prefix. */
export function formatSigned(value: string | number, currency = "COP", lang?: Language): string {
  const n = typeof value === "number" ? value : Number(value);
  if (!Number.isFinite(n)) return "—";
  const formatted = formatMoney(Math.abs(n), currency, lang);
  return n < 0 ? `−${formatted}` : `+${formatted}`;
}

export function formatDate(iso: string, lang?: Language): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "—";
  return new Intl.DateTimeFormat(resolveLocale(lang), {
    day: "2-digit",
    month: "short",
    year: "numeric",
  }).format(date);
}

/** Formats "2026-10" as a localized short month + year. */
export function formatMonth(month: string, lang?: Language): string {
  const [year, m] = month.split("-").map(Number);
  if (!year || !m) return month;
  return new Intl.DateTimeFormat(resolveLocale(lang), {
    month: "short",
    year: "numeric",
  }).format(new Date(year, m - 1, 1));
}

/** Formats "2026-10" as a single-line short month label for chart axes ("Oct", "oct", "10月"). */
export function formatMonthShort(month: string, lang?: Language): string {
  const [year, m] = month.split("-").map(Number);
  if (!year || !m) return month;
  return new Intl.DateTimeFormat(resolveLocale(lang), {
    month: "short",
  })
    .format(new Date(year, m - 1, 1))
    .replace(/\.$/, "");
}

/** Formats "2026-10" as a localized long month + year ("October 2026", "2026年10月"). */
export function formatMonthLong(month: string, lang?: Language): string {
  const [year, m] = month.split("-").map(Number);
  if (!year || !m) return month;
  return new Intl.DateTimeFormat(resolveLocale(lang), {
    month: "long",
    year: "numeric",
  }).format(new Date(year, m - 1, 1));
}

/** Formats a currency value in compact notation for chart axes and badges. */
export function formatCompactMoney(
  value: string | number,
  currency = "COP",
  lang?: Language,
): string {
  const n = typeof value === "number" ? value : Number(value);
  if (!Number.isFinite(n)) return "—";
  if (Math.abs(n) < 1000) return formatMoney(n, currency, lang);
  return new Intl.NumberFormat(resolveLocale(lang), {
    style: "currency",
    currency,
    currencyDisplay: "narrowSymbol",
    notation: "compact",
    maximumFractionDigits: 1,
  }).format(n);
}

/** Formats a Date as a localized long month + year (for the Dashboard header). */
export function formatCurrentMonthLong(date: Date = new Date(), lang?: Language): string {
  return new Intl.DateTimeFormat(resolveLocale(lang), {
    month: "long",
    year: "numeric",
  }).format(date);
}

/** Returns localized short weekday labels starting Monday (index 0 = Monday .. 6 = Sunday). */
export function getWeekdayLabels(lang?: Language): string[] {
  const locale = resolveLocale(lang);
  const formatter = new Intl.DateTimeFormat(locale, { weekday: "short" });
  // 2026-01-05 is a Monday.
  return Array.from({ length: 7 }, (_, i) =>
    formatter.format(new Date(2026, 0, 5 + i)).replace(/\.$/, ""),
  );
}

/** Current local date as "YYYY-MM-DD" for <input type="date">. */
export function todayInputValue(): string {
  const now = new Date();
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}-${String(
    now.getDate(),
  ).padStart(2, "0")}`;
}

/** "YYYY-MM-DD" from a date input (local) to an ISO timestamp at local noon. */
export function dateInputToISO(value: string): string {
  const [year, month, day] = value.split("-").map(Number);
  if (!year || !month || !day) return new Date().toISOString();
  return new Date(year, month - 1, day, 12, 0, 0).toISOString();
}

/** ISO timestamp to "YYYY-MM-DD" in local time. */
export function isoToDateInput(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return todayInputValue();
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}-${String(
    date.getDate(),
  ).padStart(2, "0")}`;
}

/** Current local month as "YYYY-MM" for <input type="month">. */
export function currentMonthValue(): string {
  const now = new Date();
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}`;
}
