import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState, type FormEvent } from "react";

import { ApiError } from "../api/client";
import {
  categoryGroups,
  financeApi,
  type Account,
  type Category,
  type Direction,
  type Transaction,
} from "../api/finance";
import { dateInputToISO, isoToDateInput, todayInputValue } from "../lib/format";

import { Modal } from "./Modal";

interface TransactionModalProps {
  accounts: Account[];
  categories: Category[];
  transaction?: Transaction;
  defaultAccountId?: string;
  onClose: () => void;
}

export function TransactionModal({ accounts, categories, transaction, defaultAccountId, onClose }: TransactionModalProps) {
  const queryClient = useQueryClient();
  const editing = Boolean(transaction);

  const [direction, setDirection] = useState<Direction>(transaction?.direction ?? "out");
  const [amount, setAmount] = useState(transaction ? String(Math.abs(Number(transaction.amount))) : "");
  const [description, setDescription] = useState(transaction?.description ?? "");
  const [accountId, setAccountId] = useState(transaction?.account_id ?? defaultAccountId ?? accounts[0]?.id ?? "");
  const [categoryId, setCategoryId] = useState(transaction?.category_id ?? "");
  const [merchant, setMerchant] = useState(transaction?.merchant ?? "");
  const [notes, setNotes] = useState(transaction?.notes ?? "");
  const [date, setDate] = useState(transaction ? isoToDateInput(transaction.occurred_at) : todayInputValue());

  const groups = categoryGroups(categories, direction);

  const mutation = useMutation({
    mutationFn: () => {
      const trimmedAmount = amount.trim();
      if (editing) {
        const patch: Record<string, unknown> = {};
        if (description.trim() !== transaction!.description) patch.description = description.trim();
        if (Number(trimmedAmount) !== Number(transaction!.amount) || direction !== transaction!.direction) {
          patch.amount = trimmedAmount;
          patch.direction = direction;
        }
        if (categoryId !== (transaction!.category_id ?? "")) patch.category_id = categoryId || null;
        if (merchant.trim() !== (transaction!.merchant ?? "")) patch.merchant = merchant.trim() || null;
        if (notes.trim() !== (transaction!.notes ?? "")) patch.notes = notes.trim() || null;
        return financeApi.updateTransaction(transaction!.id, patch);
      }
      return financeApi.createTransaction({
        account_id: accountId,
        description: description.trim(),
        amount: trimmedAmount,
        direction,
        category_id: categoryId || null,
        merchant: merchant.trim() || null,
        notes: notes.trim() || null,
        occurred_at: dateInputToISO(date),
      });
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["transactions"] });
      void queryClient.invalidateQueries({ queryKey: ["summary"] });
      void queryClient.invalidateQueries({ queryKey: ["months"] });
      void queryClient.invalidateQueries({ queryKey: ["accounts"] });
      onClose();
    },
  });

  function onSubmit(event: FormEvent) {
    event.preventDefault();
    if (mutation.isPending) return;
    const value = Number(amount);
    if (!Number.isFinite(value) || value <= 0 || !description.trim() || (!editing && !accountId)) return;
    mutation.mutate();
  }

  function switchDirection(next: Direction) {
    if (next === direction) return;
    setDirection(next);
    setCategoryId("");
  }

  const error = mutation.error instanceof ApiError ? mutation.error.message : null;

  return (
    <Modal
      title={editing ? "Edit transaction" : "Add transaction"}
      onClose={onClose}
      footer={
        <>
          <button type="button" className="btn-secondary" onClick={onClose}>
            Cancel
          </button>
          <button type="submit" form="transaction-form" className="btn-primary" disabled={mutation.isPending}>
            {mutation.isPending ? "Saving…" : editing ? "Save changes" : "Add transaction"}
          </button>
        </>
      }
    >
      <form id="transaction-form" onSubmit={onSubmit} className="space-y-4">
        <div className="grid grid-cols-2 gap-2 rounded-lg bg-slate-950 p-1">
          {(
            [
              { value: "out", label: "Expense" },
              { value: "in", label: "Income" },
            ] as const
          ).map((option) => (
            <button
              key={option.value}
              type="button"
              onClick={() => switchDirection(option.value)}
              className={`rounded-md px-3 py-1.5 text-sm font-medium transition ${
                direction === option.value
                  ? option.value === "out"
                    ? "bg-rose-500/20 text-rose-300"
                    : "bg-emerald-500/20 text-emerald-300"
                  : "text-slate-400 hover:text-slate-200"
              }`}
            >
              {option.label}
            </button>
          ))}
        </div>

        <div className="grid grid-cols-2 gap-4">
          <label className="block">
            <span className="label">Amount</span>
            <input
              className="input text-lg tabular-nums"
              value={amount}
              onChange={(event) => setAmount(event.target.value)}
              inputMode="decimal"
              placeholder="0.00"
              autoFocus
              required
            />
          </label>
          <label className="block">
            <span className="label">Date</span>
            <input
              type="date"
              className="input"
              value={date}
              onChange={(event) => setDate(event.target.value)}
              disabled={editing}
            />
          </label>
        </div>

        <label className="block">
          <span className="label">Description</span>
          <input
            className="input"
            value={description}
            onChange={(event) => setDescription(event.target.value)}
            placeholder="Supermercado"
            required
            maxLength={200}
          />
        </label>

        <div className="grid grid-cols-2 gap-4">
          <label className="block">
            <span className="label">Account</span>
            <select
              className="input"
              value={accountId}
              onChange={(event) => setAccountId(event.target.value)}
              disabled={editing}
              required
            >
              {accounts.map((account) => (
                <option key={account.id} value={account.id}>
                  {account.name}
                </option>
              ))}
            </select>
          </label>
          <label className="block">
            <span className="label">Category</span>
            <select className="input" value={categoryId} onChange={(event) => setCategoryId(event.target.value)}>
              <option value="">Uncategorized</option>
              {groups.map((group) => (
                <optgroup key={group.label} label={group.icon ? `${group.icon} ${group.label}` : group.label}>
                  {group.options.map((category) => (
                    <option key={category.id} value={category.id}>
                      {category.icon ? `${category.icon} ${category.name}` : category.name}
                    </option>
                  ))}
                </optgroup>
              ))}
            </select>
          </label>
        </div>

        <div className="grid grid-cols-2 gap-4">
          <label className="block">
            <span className="label">Merchant (optional)</span>
            <input
              className="input"
              value={merchant}
              onChange={(event) => setMerchant(event.target.value)}
              placeholder="Walmart"
              maxLength={120}
            />
          </label>
          <label className="block">
            <span className="label">Notes (optional)</span>
            <input
              className="input"
              value={notes}
              onChange={(event) => setNotes(event.target.value)}
              placeholder="Weekly groceries"
            />
          </label>
        </div>

        {error && (
          <p className="rounded-lg border border-rose-900/60 bg-rose-950/40 px-3 py-2 text-sm text-rose-300">{error}</p>
        )}
      </form>
    </Modal>
  );
}
