import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { ApiError } from "../api/client";
import { financeApi } from "../api/finance";
import type { Account, AccountPatch, AccountType } from "../api/finance";
import { useI18n } from "../i18n/I18nContext";
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
  const { t, tAccountType, tError } = useI18n();
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

  const error = mutation.error instanceof ApiError ? tError(mutation.error) : null;

  return (
    <Modal title={editing ? t("accountModal.editTitle") : t("accountModal.addTitle")} onClose={onClose}>
      <form
        className="space-y-4"
        onSubmit={(e) => {
          e.preventDefault();
          mutation.mutate();
        }}
      >
        <div>
          <label className="label" htmlFor="account-name">
            {t("accountModal.nameLabel")}
          </label>
          <input
            id="account-name"
            className="input"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder={t("accountModal.namePlaceholder")}
            maxLength={80}
            autoFocus
          />
        </div>

        <div className="form-grid">
          <div>
            <label className="label" htmlFor="account-type">
              {t("accountModal.typeLabel")}
            </label>
            <select
              id="account-type"
              className="input"
              value={type}
              onChange={(e) => setType(e.target.value as AccountType)}
            >
              {TYPES.map((accountType) => (
                <option key={accountType} value={accountType}>
                  {tAccountType(accountType)}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label className="label" htmlFor="account-bank">
              {t("accountModal.bankLabel")}
            </label>
            <select
              id="account-bank"
              className="input"
              value={linkId}
              onChange={(e) => setLinkId(e.target.value)}
              disabled={linksQuery.isLoading}
            >
              <option value="">{t("accountModal.noBankManual")}</option>
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
              {t("accountModal.openingBalanceLabel")}
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
              {t("accountModal.openingBalanceHint")}
            </p>
          </div>
        ) : (
          <p className="rounded-lg bg-slate-900 px-3 py-2 text-xs text-slate-400">
            {t("accountModal.linkedHintPrefix")} {t("accountModal.linkedHintLink")}{" "}
            {t("accountModal.linkedHintSuffix")}
          </p>
        )}

        {editing && (
          <label className="flex items-center gap-2 text-sm text-slate-300">
            <input type="checkbox" checked={isActive} onChange={(e) => setIsActive(e.target.checked)} />
            {t("accountModal.activeLabel")}
          </label>
        )}

        {error && <p className="text-sm text-rose-400">{error}</p>}

        <div className="flex justify-end gap-2 pt-2">
          <button type="button" className="btn-ghost" onClick={onClose}>
            {t("common.cancel")}
          </button>
          <button type="submit" className="btn-primary" disabled={!name.trim() || mutation.isPending}>
            {mutation.isPending
              ? t("common.saving")
              : editing
                ? t("common.saveChanges")
                : t("accountModal.addTitle")}
          </button>
        </div>
      </form>
    </Modal>
  );
}
