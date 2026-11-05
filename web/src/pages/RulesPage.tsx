import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import {
  RULE_MATCH_TYPES,
  categoryGroups,
  financeApi,
  type MatchField,
  type MatchType,
  type Rule,
} from "../api/finance";
import { AppLayout } from "../components/AppLayout";
import { EmptyState, ErrorMessage, Loading } from "../components/State";
import { useI18n } from "../i18n/I18nContext";
import { formatDate } from "../lib/format";

const FIELDS: MatchField[] = ["description", "merchant", "amount", "reference"];

function RuleForm({ onDone }: { onDone: () => void }) {
  const queryClient = useQueryClient();
  const { t, tCategory, tMatchField, tMatchType, tError } = useI18n();
  const [field, setField] = useState<MatchField>("description");
  const [matchType, setMatchType] = useState<MatchType>("contains");
  const [pattern, setPattern] = useState("");
  const [categoryId, setCategoryId] = useState("");
  const [priority, setPriority] = useState("100");
  const [error, setError] = useState<string | null>(null);

  const categoriesQuery = useQuery({ queryKey: ["categories"], queryFn: financeApi.categories });
  const groups = categoryGroups(categoriesQuery.data?.categories ?? [], "out").concat(
    categoryGroups(categoriesQuery.data?.categories ?? [], "in"),
  );

  const createMutation = useMutation({
    mutationFn: () =>
      financeApi.createRule({
        category_id: categoryId,
        field,
        match_type: matchType,
        pattern,
        priority: Number(priority) || 100,
      }),
    onSuccess: () => {
      setError(null);
      setPattern("");
      void queryClient.invalidateQueries({ queryKey: ["rules"] });
      onDone();
    },
    onError: (mutationError) => {
      setError(tError(mutationError, "rules.createFailed"));
    },
  });

  function onFieldChange(next: MatchField) {
    setField(next);
    if (!RULE_MATCH_TYPES[next].includes(matchType)) {
      setMatchType(RULE_MATCH_TYPES[next][0]);
    }
  }

  const patternPlaceholder =
    field === "amount"
      ? t("rules.patternAmountPlaceholder")
      : matchType === "regex"
        ? t("rules.patternRegexPlaceholder")
        : t("rules.patternTextPlaceholder");

  return (
    <form
      className="grid gap-3 rounded-2xl border border-slate-800 bg-slate-900/60 p-4 sm:grid-cols-2"
      onSubmit={(event) => {
        event.preventDefault();
        createMutation.mutate();
      }}
    >
      <label className="block">
        <span className="label">{t("rules.fieldLabel")}</span>
        <select
          className="input"
          value={field}
          onChange={(event) => onFieldChange(event.target.value as MatchField)}
        >
          {FIELDS.map((key) => (
            <option key={key} value={key}>
              {tMatchField(key)}
            </option>
          ))}
        </select>
      </label>
      <label className="block">
        <span className="label">{t("rules.matchLabel")}</span>
        <select
          className="input"
          value={matchType}
          onChange={(event) => setMatchType(event.target.value as MatchType)}
        >
          {RULE_MATCH_TYPES[field].map((value) => (
            <option key={value} value={value}>
              {tMatchType(value)}
            </option>
          ))}
        </select>
      </label>
      <label className="block">
        <span className="label">{t("rules.patternLabel")}</span>
        <input
          className="input"
          value={pattern}
          onChange={(event) => setPattern(event.target.value)}
          placeholder={patternPlaceholder}
        />
      </label>
      <label className="block">
        <span className="label">{t("rules.categoryLabel")}</span>
        <select
          className="input"
          value={categoryId}
          onChange={(event) => setCategoryId(event.target.value)}
        >
          <option value="">{t("rules.chooseCategory")}</option>
          {groups.map((group) => {
            const groupLabel = tCategory(group.slug, group.label);
            return (
              <optgroup
                key={group.slug ?? group.label}
                label={group.icon ? `${group.icon} ${groupLabel}` : groupLabel}
              >
                {group.options.map((option) => {
                  const catLabel = tCategory(option.slug, option.name);
                  return (
                    <option key={option.id} value={option.id}>
                      {option.icon} {catLabel}
                    </option>
                  );
                })}
              </optgroup>
            );
          })}
        </select>
      </label>
      <label className="block">
        <span className="label">{t("rules.priorityLabel")}</span>
        <input
          className="input"
          type="number"
          min={1}
          max={100000}
          value={priority}
          onChange={(event) => setPriority(event.target.value)}
        />
      </label>
      <div className="flex items-end justify-end gap-2">
        <button type="button" className="btn-ghost" onClick={onDone}>
          {t("common.cancel")}
        </button>
        <button
          type="submit"
          className="btn-primary"
          disabled={!categoryId || !pattern || createMutation.isPending}
        >
          {createMutation.isPending ? t("common.saving") : t("rules.createRule")}
        </button>
      </div>
      {error && <p className="text-sm text-rose-400 sm:col-span-2">{error}</p>}
    </form>
  );
}

export function RulesPage() {
  const queryClient = useQueryClient();
  const { t, tCategory, tMatchField, tMatchType, tError } = useI18n();
  const [adding, setAdding] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const rulesQuery = useQuery({ queryKey: ["rules"], queryFn: financeApi.rules });
  const categoriesQuery = useQuery({ queryKey: ["categories"], queryFn: financeApi.categories });
  const rules = rulesQuery.data?.rules ?? [];
  const categoryById = new Map((categoriesQuery.data?.categories ?? []).map((c) => [c.id, c]));

  function categoryLabel(rule: Rule): string {
    const category = categoryById.get(rule.category_id);
    if (!category) return rule.category_id;
    return `${category.icon} ${tCategory(category.slug, category.name)}`;
  }

  const toggleMutation = useMutation({
    mutationFn: (rule: Rule) => financeApi.updateRule(rule.id, { enabled: !rule.enabled }),
    onSuccess: () => {
      setError(null);
      void queryClient.invalidateQueries({ queryKey: ["rules"] });
    },
    onError: (mutationError) => {
      setError(tError(mutationError, "rules.updateFailed"));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => financeApi.deleteRule(id),
    onSuccess: () => {
      setError(null);
      void queryClient.invalidateQueries({ queryKey: ["rules"] });
    },
    onError: (mutationError) => {
      setError(tError(mutationError, "rules.updateFailed"));
    },
  });

  function onDelete(rule: Rule) {
    if (window.confirm(t("rules.deleteConfirm", { pattern: rule.pattern }))) {
      deleteMutation.mutate(rule.id);
    }
  }

  return (
    <AppLayout>
      <div className="mx-auto max-w-4xl space-y-6">
        <div className="page-header">
          <h1 className="text-2xl font-semibold tracking-tight">{t("nav.rules")}</h1>
          {!adding && (
            <button className="btn-primary" onClick={() => setAdding(true)}>
              {t("rules.newRule")}
            </button>
          )}
        </div>

        <p className="text-sm text-slate-400">{t("rules.subtitle")}</p>

        {error && <ErrorMessage message={error} />}

        {adding && <RuleForm onDone={() => setAdding(false)} />}

        {rulesQuery.isLoading ? (
          <Loading />
        ) : rules.length === 0 ? (
          <EmptyState
            icon="⚡️"
            title={t("rules.emptyTitle")}
            description={t("rules.emptyDescription")}
          />
        ) : (
          <div className="overflow-x-auto rounded-2xl border border-slate-800">
            <table role="table" className="responsive-table text-sm">
              <thead className="bg-slate-900/80 text-left text-xs uppercase tracking-wide text-slate-500">
                <tr>
                  <th className="px-4 py-3 font-medium">{t("rules.colWhen")}</th>
                  <th className="px-4 py-3 font-medium">{t("rules.colCategory")}</th>
                  <th className="px-4 py-3 text-right font-medium">{t("rules.colPriority")}</th>
                  <th className="px-4 py-3 font-medium">{t("rules.colEnabled")}</th>
                  <th className="px-4 py-3" />
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/80">
                {rules.map((rule) => (
                  <tr key={rule.id} className={rule.enabled ? "" : "opacity-50"}>
                    <td data-col="when" data-label={t("rules.colWhen")} className="px-4 py-3">
                      <span className="text-slate-500">{tMatchField(rule.field)}</span>{" "}
                      <span className="text-slate-300">{tMatchType(rule.match_type)}</span>{" "}
                      <span className="font-mono text-slate-100">“{rule.pattern}”</span>
                    </td>
                    <td
                      data-col="category"
                      data-label={t("rules.colCategory")}
                      className="px-4 py-3"
                    >
                      {categoryLabel(rule)}
                    </td>
                    <td
                      data-col="priority"
                      data-label={t("rules.colPriority")}
                      className="px-4 py-3 text-right tabular-nums text-slate-400"
                    >
                      {rule.priority}
                    </td>
                    <td
                      data-col="enabled"
                      data-label={t("rules.colEnabled")}
                      className="px-4 py-3"
                    >
                      <button
                        role="switch"
                        aria-checked={rule.enabled}
                        aria-label={t("rules.ruleLabel", { pattern: rule.pattern })}
                        onClick={() => toggleMutation.mutate(rule)}
                        disabled={toggleMutation.isPending}
                        className="inline-flex size-11 items-center justify-center rounded-lg"
                      >
                        <span
                          aria-hidden="true"
                          className={`relative h-6 w-10 rounded-full transition ${
                            rule.enabled ? "bg-emerald-500" : "bg-slate-700"
                          }`}
                        >
                          <span
                            className={`absolute top-1 size-4 rounded-full bg-slate-50 transition-all ${
                              rule.enabled ? "left-5" : "left-1"
                            }`}
                          />
                        </span>
                      </button>
                    </td>
                    <td
                      data-col="actions"
                      data-label={t("rules.colActions")}
                      className="px-4 py-3"
                    >
                      <div className="flex justify-end gap-1">
                        <span className="self-center text-xs text-slate-500">
                          {formatDate(rule.created_at)}
                        </span>
                        <button
                          className="btn-ghost text-rose-400 hover:text-rose-300"
                          onClick={() => onDelete(rule)}
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
      </div>
    </AppLayout>
  );
}
