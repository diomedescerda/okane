const currencyFormatters = new Map<string, Intl.NumberFormat>();

function currencyFormatter(currency: string): Intl.NumberFormat {
  let formatter = currencyFormatters.get(currency);
  if (!formatter) {
    // The Colombian peso is not used with decimal subunits in everyday amounts.
    const zeroDecimals = currency === "COP";
    formatter = new Intl.NumberFormat("es-CO", {
      style: "currency",
      currency,
      currencyDisplay: "narrowSymbol",
      ...(zeroDecimals ? { minimumFractionDigits: 0, maximumFractionDigits: 0 } : {}),
    });
    currencyFormatters.set(currency, formatter);
  }
  return formatter;
}

/** Formats a decimal string (e.g. "-1234.50") as currency. */
export function formatMoney(value: string | number, currency = "COP"): string {
  const n = typeof value === "number" ? value : Number(value);
  if (!Number.isFinite(n)) return "—";
  return currencyFormatter(currency).format(n);
}

/** Formats a signed amount with an explicit + / − prefix. */
export function formatSigned(value: string | number, currency = "COP"): string {
  const n = typeof value === "number" ? value : Number(value);
  if (!Number.isFinite(n)) return "—";
  const formatted = formatMoney(Math.abs(n), currency);
  return n < 0 ? `−${formatted}` : `+${formatted}`;
}

export function formatDate(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "—";
  return new Intl.DateTimeFormat("es-CO", { day: "2-digit", month: "short", year: "numeric" }).format(date);
}

/** Formats "2026-10" as "oct 2026". */
export function formatMonth(month: string): string {
  const [year, m] = month.split("-").map(Number);
  if (!year || !m) return month;
  return new Intl.DateTimeFormat("es-CO", { month: "short", year: "numeric" }).format(
    new Date(year, m - 1, 1),
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
