import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { ApiError } from "../api/client";
import {
  MATCH_TYPE_LABELS,
  RULE_MATCH_TYPES,
  categoryGroups,
  financeApi,
  type MatchField,
  type MatchType,
  type Rule,
} from "../api/finance";
import { AppLayout } from "../components/AppLayout";
import { EmptyState, ErrorMessage, Loading } from "../components/State";
import { formatDate } from "../lib/format";

const FIELD_LABELS: Record<MatchField, string> = {
  description: "Description",
  merchant: "Merchant",
  amount: "Amount",
  reference: "Reference",
};

const PATTERN_HINTS: Record<MatchField, string> = {
  description: "e.g. D1 or Pepe Ganga",
  merchant: "e.g. Rappi",
  amount: "Plain number, e.g. 45000",
  reference: "e.g. PSE1234",
};

function RuleForm({ onDone }: { onDone: () => void }) {
  const queryClient = useQueryClient();
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
      setError(mutationError instanceof ApiError ? mutationError.message : "Could not create the rule.");
    },
  });

  function onFieldChange(next: MatchField) {
    setField(next);
    if (!RULE_MATCH_TYPES[next].includes(matchType)) {
      setMatchType(RULE_MATCH_TYPES[next][0]);
    }
  }

  return (
    <form
      className="grid gap-3 rounded-2xl border border-slate-800 bg-slate-900/60 p-4 sm:grid-cols-2"
      onSubmit={(event) => {
        event.preventDefault();
        createMutation.mutate();
      }}
    >
      <label className="block">
        <span className="label">Field</span>
        <select className="input" value={field} onChange={(event) => onFieldChange(event.target.value as MatchField)}>
          {Object.keys(FIELD_LABELS).map((key) => (
            <option key={key} value={key}>
              {FIELD_LABELS[key as MatchField]}
            </option>
          ))}
        </select>
      </label>
      <label className="block">
        <span className="label">Match</span>
        <select className="input" value={matchType} onChange={(event) => setMatchType(event.target.value as MatchType)}>
          {RULE_MATCH_TYPES[field].map((value) => (
            <option key={value} value={value}>
              {MATCH_TYPE_LABELS[value]}
            </option>
          ))}
        </select>
      </label>
      <label className="block">
        <span className="label">Pattern</span>
        <input
          className="input"
          value={pattern}
          onChange={(event) => setPattern(event.target.value)}
          placeholder={PATTERN_HINTS[field]}
        />
      </label>
      <label className="block">
        <span className="label">Category</span>
        <select className="input" value={categoryId} onChange={(event) => setCategoryId(event.target.value)}>
          <option value="">Choose a category…</option>
          {groups.map((group) => (
            <optgroup key={group.label} label={group.label}>
              {group.options.map((option) => (
                <option key={option.id} value={option.id}>
                  {option.icon} {option.name}
                </option>
              ))}
            </optgroup>
          ))}
        </select>
      </label>
      <label className="block">
        <span className="label">Priority</span>
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
          Cancel
        </button>
        <button type="submit" className="btn-primary" disabled={!categoryId || !pattern || createMutation.isPending}>
          {createMutation.isPending ? "Saving…" : "Create rule"}
        </button>
      </div>
      {error && (
        <p className="text-sm text-rose-400 sm:col-span-2">
          {error}
        </p>
      )}
    </form>
  );
}

export function RulesPage() {
  const queryClient = useQueryClient();
  const [adding, setAdding] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const rulesQuery = useQuery({ queryKey: ["rules"], queryFn: financeApi.rules });
  const categoriesQuery = useQuery({ queryKey: ["categories"], queryFn: financeApi.categories });
  const rules = rulesQuery.data?.rules ?? [];
  const categoryById = new Map((categoriesQuery.data?.categories ?? []).map((c) => [c.id, c]));

  function categoryLabel(rule: Rule): string {
    const category = categoryById.get(rule.category_id);
    return category ? `${category.icon} ${category.name}` : rule.category_id;
  }

  const toggleMutation = useMutation({
    mutationFn: (rule: Rule) => financeApi.updateRule(rule.id, { enabled: !rule.enabled }),
    onSuccess: () => {
      setError(null);
      void queryClient.invalidateQueries({ queryKey: ["rules"] });
    },
    onError: (mutationError) => {
      setError(mutationError instanceof ApiError ? mutationError.message : "Could not update the rule.");
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => financeApi.deleteRule(id),
    onSuccess: () => {
      setError(null);
      void queryClient.invalidateQueries({ queryKey: ["rules"] });
    },
    onError: (mutationError) => {
      setError(mutationError instanceof ApiError ? mutationError.message : "Could not delete the rule.");
    },
  });

  function onDelete(rule: Rule) {
    if (window.confirm(`Delete rule "${rule.pattern}"?`)) {
      deleteMutation.mutate(rule.id);
    }
  }

  return (
    <AppLayout>
      <div className="mx-auto max-w-4xl space-y-6">
        <div className="flex items-center justify-between">
          <h1 className="text-2xl font-semibold tracking-tight">Rules</h1>
          {!adding && (
            <button className="btn-primary" onClick={() => setAdding(true)}>
              + New rule
            </button>
          )}
        </div>

        <p className="text-sm text-slate-400">
          Rules categorize transactions automatically. Lower priorities run first, and they only fill the category
          when it has not been set by hand.
        </p>

        {error && <ErrorMessage message={error} />}

        {adding && <RuleForm onDone={() => setAdding(false)} />}

        {rulesQuery.isLoading ? (
          <Loading />
        ) : rules.length === 0 ? (
          <EmptyState
            icon="⚡️"
            title="No rules yet"
            description="Create a rule like “description contains D1 → Groceries” and imported transactions will get categorized on arrival."
          />
        ) : (
          <div className="overflow-hidden rounded-2xl border border-slate-800">
            <table className="w-full text-sm">
              <thead className="bg-slate-900/80 text-left text-xs uppercase tracking-wide text-slate-500">
                <tr>
                  <th className="px-4 py-3 font-medium">When</th>
                  <th className="px-4 py-3 font-medium">Category</th>
                  <th className="px-4 py-3 text-right font-medium">Priority</th>
                  <th className="px-4 py-3 font-medium">Enabled</th>
                  <th className="px-4 py-3" />
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/80">
                {rules.map((rule) => (
                  <tr key={rule.id} className={rule.enabled ? "" : "opacity-50"}>
                    <td className="px-4 py-3">
                      <span className="text-slate-500">{FIELD_LABELS[rule.field]}</span>{" "}
                      <span className="text-slate-300">{MATCH_TYPE_LABELS[rule.match_type]}</span>{" "}
                      <span className="font-mono text-slate-100">“{rule.pattern}”</span>
                    </td>
                    <td className="px-4 py-3">{categoryLabel(rule)}</td>
                    <td className="px-4 py-3 text-right tabular-nums text-slate-400">{rule.priority}</td>
                    <td className="px-4 py-3">
                      <button
                        role="switch"
                        aria-checked={rule.enabled}
                        onClick={() => toggleMutation.mutate(rule)}
                        disabled={toggleMutation.isPending}
                        className={`relative h-5 w-9 rounded-full transition ${
                          rule.enabled ? "bg-emerald-500" : "bg-slate-700"
                        }`}
                      >
                        <span
                          className={`absolute top-0.5 h-4 w-4 rounded-full bg-white transition ${
                            rule.enabled ? "left-4.5" : "left-0.5"
                          }`}
                        />
                      </button>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex justify-end gap-1">
                        <span className="self-center text-xs text-slate-500">{formatDate(rule.created_at)}</span>
                        <button
                          className="btn-ghost text-rose-400 hover:text-rose-300"
                          onClick={() => onDelete(rule)}
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
        )}
      </div>
    </AppLayout>
  );
}
