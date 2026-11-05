import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState, type FormEvent } from "react";

import { financeApi, type InstitutionType } from "../api/finance";
import { ApiError } from "../api/client";
import { useI18n } from "../i18n/I18nContext";

import { Modal } from "./Modal";

interface InstitutionModalProps {
  onClose: () => void;
}

const TYPES: InstitutionType[] = ["bank", "wallet", "other"];

export function InstitutionModal({ onClose }: InstitutionModalProps) {
  const queryClient = useQueryClient();
  const { t, tInstitutionType, tError } = useI18n();

  const [displayName, setDisplayName] = useState("");
  const [type, setType] = useState<InstitutionType>("bank");
  const [primaryColor, setPrimaryColor] = useState("#334155");
  const [icon, setIcon] = useState("🏦");
  const [website, setWebsite] = useState("");

  const mutation = useMutation({
    mutationFn: () =>
      financeApi.createInstitution({
        display_name: displayName.trim(),
        type,
        primary_color: primaryColor,
        icon: icon.trim(),
        website: website.trim(),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["institutions"] });
      onClose();
    },
  });

  function onSubmit(event: FormEvent) {
    event.preventDefault();
    if (!mutation.isPending) mutation.mutate();
  }

  const error = mutation.error instanceof ApiError ? tError(mutation.error) : null;

  return (
    <Modal
      title={t("institutionModal.title")}
      onClose={onClose}
      footer={
        <>
          <button type="button" className="btn-secondary" onClick={onClose}>
            {t("common.cancel")}
          </button>
          <button
            type="submit"
            form="institution-form"
            className="btn-primary"
            disabled={mutation.isPending}
          >
            {mutation.isPending ? t("common.saving") : t("institutionModal.addButton")}
          </button>
        </>
      }
    >
      <form id="institution-form" onSubmit={onSubmit} className="space-y-4">
        <label className="block">
          <span className="label">{t("institutionModal.nameLabel")}</span>
          <input
            className="input"
            value={displayName}
            onChange={(event) => setDisplayName(event.target.value)}
            placeholder={t("institutionModal.namePlaceholder")}
            autoFocus
            required
            maxLength={60}
          />
        </label>

        <div className="form-grid">
          <label className="block">
            <span className="label">{t("institutionModal.typeLabel")}</span>
            <select
              className="input"
              value={type}
              onChange={(event) => setType(event.target.value as InstitutionType)}
            >
              {TYPES.map((instType) => (
                <option key={instType} value={instType}>
                  {tInstitutionType(instType)}
                </option>
              ))}
            </select>
          </label>
          <label className="block">
            <span className="label">{t("institutionModal.iconLabel")}</span>
            <input
              className="input"
              value={icon}
              onChange={(event) => setIcon(event.target.value)}
              placeholder="🏦"
              maxLength={8}
            />
          </label>
        </div>

        <div className="form-grid">
          <label className="block">
            <span className="label">{t("institutionModal.brandColorLabel")}</span>
            <div className="flex items-center gap-2">
              <input
                type="color"
                value={primaryColor}
                onChange={(event) => setPrimaryColor(event.target.value)}
                aria-label={t("institutionModal.brandColorPickerAria")}
                className="h-11 w-12 shrink-0 cursor-pointer rounded border border-slate-700 bg-slate-900"
              />
              <input
                className="input"
                value={primaryColor}
                onChange={(event) => setPrimaryColor(event.target.value)}
                placeholder="#334155"
                maxLength={7}
              />
            </div>
          </label>
          <label className="block">
            <span className="label">{t("institutionModal.websiteLabel")}</span>
            <input
              className="input"
              value={website}
              onChange={(event) => setWebsite(event.target.value)}
              placeholder="https://…"
              maxLength={120}
            />
          </label>
        </div>

        <p className="text-xs text-slate-500">{t("institutionModal.hint")}</p>

        {error && (
          <p className="rounded-lg border border-rose-900/60 bg-rose-950/40 px-3 py-2 text-sm text-rose-300">
            {error}
          </p>
        )}
      </form>
    </Modal>
  );
}
