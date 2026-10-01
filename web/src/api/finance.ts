import { api } from "./client";

export type AccountType = "checking" | "savings" | "credit_card" | "loan" | "investment" | "other";
export type Direction = "in" | "out";
export type CategoryKind = "expense" | "income" | "transfer";
export type InstitutionType = "bank" | "wallet" | "other";

export interface Account {
  id: string;
  user_id: string;
  link_id?: string;
  provider: string;
  name: string;
  type: AccountType;
  subtype?: string;
  currency: string;
  balance_current: string;
  opening_balance: string;
  balance_available?: string;
  credit_limit?: string;
  institution_name: string;
  institution_logo_url?: string;
  institution_color?: string;
  institution_icon?: string;
  institution_type?: InstitutionType;
  is_active: boolean;
  transaction_count: number;
  created_at: string;
  updated_at: string;
}

export interface Institution {
  id: string;
  user_id?: string;
  provider: string;
  slug: string;
  name: string;
  display_name: string;
  country_code: string;
  type: InstitutionType;
  website: string;
  primary_color: string;
  logo_url: string;
  icon: string;
  sort_order: number;
  is_system: boolean;
  is_active: boolean;
  link_count: number;
  created_at: string;
  updated_at: string;
}

export interface Link {
  id: string;
  user_id: string;
  provider: string;
  institution_id?: string;
  institution: string;
  institution_name: string;
  access_mode: string;
  status: string;
  last_sync_at?: string;
  last_sync_error: string;
  institution_logo_url?: string;
  institution_primary_color?: string;
  institution_icon?: string;
  institution_type?: InstitutionType;
  account_count: number;
  created_at: string;
  updated_at: string;
}

export interface NewInstitution {
  display_name: string;
  type: InstitutionType;
  website?: string;
  primary_color?: string;
  icon?: string;
}

export interface InstitutionQuery {
  type?: InstitutionType | "";
  search?: string;
  country?: string;
}

export interface Category {
  id: string;
  user_id?: string;
  parent_id?: string;
  slug: string;
  name: string;
  kind: CategoryKind;
  icon?: string;
  color?: string;
  sort_order: number;
  is_system: boolean;
}

export interface Transaction {
  id: string;
  account_id: string;
  category_id?: string;
  description: string;
  merchant?: string;
  notes?: string;
  amount: string;
  direction: Direction;
  currency: string;
  status: "posted" | "pending";
  category_source?: string;
  occurred_at: string;
  tags: string[];
  account_name: string;
  category_name?: string;
  category_icon?: string;
  category_color?: string;
  category_slug?: string;
  created_at: string;
  updated_at: string;
}

export interface CategoryTotal {
  category_id?: string;
  name: string;
  icon?: string;
  color?: string;
  kind?: CategoryKind;
  direction: Direction;
  total: string;
  count: number;
}

export interface Summary {
  from: string;
  to: string;
  income: string;
  expenses: string;
  net: string;
  net_worth: string;
  by_category: CategoryTotal[];
}

export interface MonthTotal {
  month: string;
  income: string;
  expenses: string;
  net: string;
}

export interface TransactionList {
  transactions: Transaction[];
  total: number;
  limit: number;
  offset: number;
}

export interface TransactionQuery {
  account_id?: string;
  category_id?: string;
  direction?: Direction | "";
  search?: string;
  from?: string;
  to?: string;
  limit?: number;
  offset?: number;
}

export interface NewAccount {
  name: string;
  type: AccountType;
  currency?: string;
  opening_balance?: string;
  institution_name?: string;
  link_id?: string;
}

export interface AccountPatch {
  name?: string;
  type?: AccountType;
  opening_balance?: string;
  institution_name?: string;
  is_active?: boolean;
  link_id?: string | null;
}

export interface NewTransaction {
  account_id: string;
  description: string;
  amount: string;
  direction: Direction;
  category_id?: string | null;
  merchant?: string | null;
  notes?: string | null;
  occurred_at?: string;
  status?: string;
}

export interface TransactionPatch {
  description?: string;
  amount?: string;
  direction?: Direction;
  category_id?: string | null;
  merchant?: string | null;
  notes?: string | null;
}

function queryString(query: TransactionQuery): string {
  const params = new URLSearchParams();
  if (query.account_id) params.set("account_id", query.account_id);
  if (query.category_id) params.set("category_id", query.category_id);
  if (query.direction) params.set("direction", query.direction);
  if (query.search) params.set("search", query.search);
  if (query.from) params.set("from", query.from);
  if (query.to) params.set("to", query.to);
  if (query.limit) params.set("limit", String(query.limit));
  if (query.offset) params.set("offset", String(query.offset));
  const qs = params.toString();
  return qs ? `?${qs}` : "";
}

export function monthRange(month: string): { from: string; to: string } {
  const [year, m] = month.split("-").map(Number);
  const from = new Date(year, m - 1, 1);
  const to = new Date(year, m, 1);
  return { from: from.toISOString(), to: to.toISOString() };
}

export const financeApi = {
  accounts: () => api.get<{ accounts: Account[] }>("/api/v1/accounts"),
  account: (id: string) => api.get<Account>(`/api/v1/accounts/${id}`),
  createAccount: (body: NewAccount) => api.post<Account>("/api/v1/accounts", body),
  updateAccount: (id: string, body: AccountPatch) => api.patch<Account>(`/api/v1/accounts/${id}`, body),
  deleteAccount: (id: string) => api.delete<void>(`/api/v1/accounts/${id}`),

  categories: () => api.get<{ categories: Category[] }>("/api/v1/categories"),

  institutions: (query: InstitutionQuery = {}) => {
    const params = new URLSearchParams();
    if (query.type) params.set("type", query.type);
    if (query.search) params.set("search", query.search);
    if (query.country) params.set("country", query.country);
    const qs = params.toString();
    return api.get<{ institutions: Institution[] }>(`/api/v1/institutions${qs ? `?${qs}` : ""}`);
  },
  createInstitution: (body: NewInstitution) => api.post<Institution>("/api/v1/institutions", body),
  deleteInstitution: (id: string) => api.delete<void>(`/api/v1/institutions/${id}`),

  links: () => api.get<{ links: Link[] }>("/api/v1/links"),
  createLink: (institutionId: string) => api.post<Link>("/api/v1/links", { institution_id: institutionId }),
  deleteLink: (id: string) => api.delete<void>(`/api/v1/links/${id}`),

  transactions: (query: TransactionQuery = {}) =>
    api.get<TransactionList>(`/api/v1/transactions${queryString(query)}`),
  createTransaction: (body: NewTransaction) => api.post<Transaction>("/api/v1/transactions", body),
  updateTransaction: (id: string, body: TransactionPatch) =>
    api.patch<Transaction>(`/api/v1/transactions/${id}`, body),
  deleteTransaction: (id: string) => api.delete<void>(`/api/v1/transactions/${id}`),

  summary: (from?: string, to?: string) => {
    const params = new URLSearchParams();
    if (from) params.set("from", from);
    if (to) params.set("to", to);
    const qs = params.toString();
    return api.get<Summary>(`/api/v1/summary${qs ? `?${qs}` : ""}`);
  },
  months: (months = 6) => api.get<{ months: MonthTotal[] }>(`/api/v1/summary/months?months=${months}`),
};

export const ACCOUNT_TYPE_LABELS: Record<AccountType, string> = {
  checking: "Checking",
  savings: "Savings",
  credit_card: "Credit card",
  loan: "Loan",
  investment: "Investment",
  other: "Other",
};

export const INSTITUTION_TYPE_LABELS: Record<InstitutionType, string> = {
  bank: "Bank",
  wallet: "Wallet",
  other: "Other",
};

export interface CategoryGroup {
  label: string;
  icon?: string;
  options: Category[];
}

/** Builds category select groups for a direction (out → expense, in → income). */
export function categoryGroups(categories: Category[], direction: Direction): CategoryGroup[] {
  const kind: CategoryKind = direction === "in" ? "income" : "expense";
  const matching = categories.filter((c) => c.kind === kind);
  const byId = new Map(matching.map((c) => [c.id, c]));
  const children = new Map<string, Category[]>();
  const roots: Category[] = [];
  for (const category of matching) {
    if (category.parent_id && byId.has(category.parent_id)) {
      const list = children.get(category.parent_id) ?? [];
      list.push(category);
      children.set(category.parent_id, list);
    } else {
      roots.push(category);
    }
  }
  return roots.map((root) => {
    const options = children.get(root.id) ?? [];
    return { label: root.name, icon: root.icon, options: options.length > 0 ? options : [root] };
  });
}
