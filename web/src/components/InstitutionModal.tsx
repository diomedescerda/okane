import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState, type FormEvent } from "react";

import { INSTITUTION_TYPE_LABELS, financeApi, type InstitutionType } from "../api/finance";
import { ApiError } from "../api/client";

import { Modal } from "./Modal";

interface InstitutionModalProps {
  onClose: () => void;
}

const TYPES: InstitutionType[] = ["bank", "wallet", "other"];

export function InstitutionModal({ onClose }: InstitutionModalProps) {
  const queryClient = useQueryClient();

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

  const error = mutation.error instanceof ApiError ? mutation.error.message : null;

  return (
    <Modal
      title="Add a custom bank"
      onClose={onClose}
      footer={
        <>
          <button type="button" className="btn-secondary" onClick={onClose}>
            Cancel
          </button>
          <button type="submit" form="institution-form" className="btn-primary" disabled={mutation.isPending}>
            {mutation.isPending ? "Saving…" : "Add bank"}
          </button>
        </>
      }
    >
      <form id="institution-form" onSubmit={onSubmit} className="space-y-4">
        <label className="block">
          <span className="label">Name</span>
          <input
            className="input"
            value={displayName}
            onChange={(event) => setDisplayName(event.target.value)}
            placeholder="Cooperativa Financiera"
            autoFocus
            required
            maxLength={60}
          />
        </label>

        <div className="grid grid-cols-2 gap-4">
          <label className="block">
            <span className="label">Type</span>
            <select className="input" value={type} onChange={(event) => setType(event.target.value as InstitutionType)}>
              {TYPES.map((t) => (
                <option key={t} value={t}>
                  {INSTITUTION_TYPE_LABELS[t]}
                </option>
              ))}
            </select>
          </label>
          <label className="block">
            <span className="label">Icon</span>
            <input
              className="input"
              value={icon}
              onChange={(event) => setIcon(event.target.value)}
              placeholder="🏦"
              maxLength={8}
            />
          </label>
        </div>

        <div className="grid grid-cols-2 gap-4">
          <label className="block">
            <span className="label">Brand color</span>
            <div className="flex items-center gap-2">
              <input
                type="color"
                value={primaryColor}
                onChange={(event) => setPrimaryColor(event.target.value)}
                className="h-9 w-12 cursor-pointer rounded border border-slate-700 bg-slate-900"
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
            <span className="label">Website (optional)</span>
            <input
              className="input"
              value={website}
              onChange={(event) => setWebsite(event.target.value)}
              placeholder="https://…"
              maxLength={120}
            />
          </label>
        </div>

        <p className="text-xs text-slate-500">
          Custom banks show their emoji and color. Built-in Colombian banks use their real logos.
        </p>

        {error && (
          <p className="rounded-lg border border-rose-900/60 bg-rose-950/40 px-3 py-2 text-sm text-rose-300">{error}</p>
        )}
      </form>
    </Modal>
  );
}
