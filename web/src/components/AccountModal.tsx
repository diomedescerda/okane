import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { ApiError } from "../api/client";
import { ACCOUNT_TYPE_LABELS, financeApi } from "../api/finance";
import type { Account, AccountPatch, AccountType } from "../api/finance";
import { Modal } from "./Modal";

const TYPES: AccountType[] = ["checking", "savings", "credit_card", "loan", "investment", "other"];

export function AccountModal({
  account,
  defaultLinkId,
  onClose,
}: {
  account?: Account;
  defaultLinkId?: string;
  onClose: () => void;
}) {
  const editing = Boolean(account);
  const queryClient = useQueryClient();
  const linksQuery = useQuery({ queryKey: ["links"], queryFn: financeApi.links });
  const links = linksQuery.data?.links ?? [];

  const [name, setName] = useState(account?.name ?? "");
  const [type, setType] = useState(account?.type ?? "checking");
  const [linkId, setLinkId] = useState(account ? account.link_id ?? "" : defaultLinkId ?? "");
  const [openingBalance, setOpeningBalance] = useState(
    account ? String(Number(account.opening_balance)) : "0",
  );
  const [isActive, setIsActive] = useState(account?.is_active ?? true);

  const manual = linkId === "";

  const mutation = useMutation({
    mutationFn: () => {
      const link = links.find((l) => l.id === linkId);
      if (account) {
        const patch: AccountPatch = { name: name.trim(), type };
        const initialLinkId = account.link_id ?? "";
        if (linkId !== initialLinkId) patch.link_id = linkId === "" ? null : linkId;
        if (manual) {
          patch.opening_balance = openingBalance;
        }
        if (isActive !== account.is_active) patch.is_active = isActive;
        return financeApi.updateAccount(account.id, patch);
      }
      return financeApi.createAccount({
        name: name.trim(),
        type,
        link_id: link?.id,
        institution_name: link?.institution_name,
        opening_balance: manual ? openingBalance : undefined,
      });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["accounts"] });
      queryClient.invalidateQueries({ queryKey: ["summary"] });
      queryClient.invalidateQueries({ queryKey: ["links"] });
      onClose();
    },
  });

  const error = mutation.error instanceof ApiError ? mutation.error.message : null;

  return (
    <Modal title={editing ? "Edit account" : "Add account"} onClose={onClose}>
      <form
        className="space-y-4"
        onSubmit={(e) => {
          e.preventDefault();
          mutation.mutate();
        }}
      >
        <div>
          <label className="label" htmlFor="account-name">
            Name
          </label>
          <input
            id="account-name"
            className="input"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="BBVA Nómina"
            maxLength={80}
            autoFocus
          />
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="label" htmlFor="account-type">
              Type
            </label>
            <select id="account-type" className="input" value={type} onChange={(e) => setType(e.target.value as AccountType)}>
              {TYPES.map((t) => (
                <option key={t} value={t}>
                  {ACCOUNT_TYPE_LABELS[t] ?? t}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label className="label" htmlFor="account-bank">
              Bank
            </label>
            <select
              id="account-bank"
              className="input"
              value={linkId}
              onChange={(e) => setLinkId(e.target.value)}
              disabled={linksQuery.isLoading}
            >
              <option value="">No bank (manual)</option>
              {links.map((l) => (
                <option key={l.id} value={l.id}>
                  {l.institution_name}
                </option>
              ))}
            </select>
          </div>
        </div>

        {manual ? (
          <div>
            <label className="label" htmlFor="account-opening">
              Opening balance
            </label>
            <input
              id="account-opening"
              className="input"
              type="number"
              step="0.01"
              value={openingBalance}
              onChange={(e) => setOpeningBalance(e.target.value)}
            />
            <p className="mt-1 text-xs text-slate-500">
              The balance you had before starting to track this account.
            </p>
          </div>
        ) : (
          <p className="rounded-lg bg-slate-900 px-3 py-2 text-xs text-slate-400">
            Linked accounts take their balance from the bank. You can connect a bank from the Banks
            page.
          </p>
        )}

        {editing && (
          <label className="flex items-center gap-2 text-sm text-slate-300">
            <input type="checkbox" checked={isActive} onChange={(e) => setIsActive(e.target.checked)} />
            Active
          </label>
        )}

        {error && <p className="text-sm text-rose-400">{error}</p>}

        <div className="flex justify-end gap-2 pt-2">
          <button type="button" className="btn-ghost" onClick={onClose}>
            Cancel
          </button>
          <button type="submit" className="btn-primary" disabled={!name.trim() || mutation.isPending}>
            {mutation.isPending ? "Saving…" : editing ? "Save changes" : "Add account"}
          </button>
        </div>
      </form>
    </Modal>
  );
}
