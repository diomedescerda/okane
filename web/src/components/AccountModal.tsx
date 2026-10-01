import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState, type FormEvent } from "react";

import { ACCOUNT_TYPE_LABELS, financeApi, type Account, type AccountType } from "../api/finance";
import { ApiError } from "../api/client";

import { Modal } from "./Modal";

type Mode = "create" | "edit";

interface AccountModalProps {
  account?: Account;
  onClose: () => void;
}

const TYPES: AccountType[] = ["checking", "savings", "credit_card", "loan", "investment", "other"];

export function AccountModal({ account, onClose }: AccountModalProps) {
  const queryClient = useQueryClient();
  const mode: Mode = account ? "edit" : "create";
  const manual = !account?.link_id;

  const [name, setName] = useState(account?.name ?? "");
  const [type, setType] = useState<AccountType>(account?.type ?? "checking");
  const [institution, setInstitution] = useState(account?.institution_name ?? "");
  const [openingBalance, setOpeningBalance] = useState(account ? String(Number(account.opening_balance)) : "0");
  const [isActive, setIsActive] = useState(account?.is_active ?? true);

  const mutation = useMutation({
    mutationFn: () => {
      const body = {
        name: name.trim(),
        type,
        institution_name: institution.trim(),
        ...(manual ? { opening_balance: openingBalance.trim() || "0" } : {}),
        ...(mode === "edit" ? { is_active: isActive } : {}),
      };
      return mode === "create"
        ? financeApi.createAccount(body)
        : financeApi.updateAccount(account!.id, body);
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["accounts"] });
      void queryClient.invalidateQueries({ queryKey: ["summary"] });
      onClose();
    },
  });

  function onSubmit(event: FormEvent) {
    event.preventDefault();
    if (!mutation.isPending) mutation.mutate();
  }

  const error = mutation.error instanceof ApiError ? mutation.error.message : null;

  return (
    <Modal
      title={mode === "create" ? "New account" : "Edit account"}
      onClose={onClose}
      footer={
        <>
          <button type="button" className="btn-secondary" onClick={onClose}>
            Cancel
          </button>
          <button type="submit" form="account-form" className="btn-primary" disabled={mutation.isPending}>
            {mutation.isPending ? "Saving…" : mode === "create" ? "Create account" : "Save changes"}
          </button>
        </>
      }
    >
      <form id="account-form" onSubmit={onSubmit} className="space-y-4">
        <label className="block">
          <span className="label">Name</span>
          <input
            className="input"
            value={name}
            onChange={(event) => setName(event.target.value)}
            placeholder="BBVA Nómina"
            autoFocus
            required
            maxLength={80}
          />
        </label>

        <div className="grid grid-cols-2 gap-4">
          <label className="block">
            <span className="label">Type</span>
            <select className="input" value={type} onChange={(event) => setType(event.target.value as AccountType)}>
              {TYPES.map((t) => (
                <option key={t} value={t}>
                  {ACCOUNT_TYPE_LABELS[t]}
                </option>
              ))}
            </select>
          </label>
          <label className="block">
            <span className="label">Institution</span>
            <input
              className="input"
              value={institution}
              onChange={(event) => setInstitution(event.target.value)}
              placeholder="BBVA"
              maxLength={80}
            />
          </label>
        </div>

        {manual && (
          <label className="block">
            <span className="label">Opening balance</span>
            <input
              className="input"
              value={openingBalance}
              onChange={(event) => setOpeningBalance(event.target.value)}
              inputMode="decimal"
              placeholder="0.00"
            />
            <span className="mt-1 block text-xs text-slate-500">
              {mode === "create"
                ? "Starting balance before any tracked transactions."
                : "The current balance is this plus the sum of tracked transactions."}
            </span>
          </label>
        )}

        {mode === "edit" && (
          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={isActive}
              onChange={(event) => setIsActive(event.target.checked)}
              className="h-4 w-4 accent-emerald-500"
            />
            Active (include in net worth and summaries)
          </label>
        )}

        {error && (
          <p className="rounded-lg border border-rose-900/60 bg-rose-950/40 px-3 py-2 text-sm text-rose-300">{error}</p>
        )}
      </form>
    </Modal>
  );
}
