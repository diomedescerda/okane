import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Link } from "react-router-dom";

import {
  INSTITUTION_TYPE_LABELS,
  financeApi,
  type Institution,
  type InstitutionType,
} from "../api/finance";
import { ApiError } from "../api/client";
import { AccountModal } from "../components/AccountModal";
import { AppLayout } from "../components/AppLayout";
import { InstitutionIcon } from "../components/InstitutionIcon";
import { InstitutionModal } from "../components/InstitutionModal";
import { EmptyState, ErrorMessage, Loading } from "../components/State";

export function BanksPage() {
  const queryClient = useQueryClient();
  const [search, setSearch] = useState("");
  const [typeFilter, setTypeFilter] = useState<InstitutionType | "">("");
  const [customModal, setCustomModal] = useState(false);
  const [accountModal, setAccountModal] = useState<{ linkId?: string } | null>(null);
  const [error, setError] = useState<string | null>(null);

  const linksQuery = useQuery({ queryKey: ["links"], queryFn: financeApi.links });
  const institutionsQuery = useQuery({
    queryKey: ["institutions", { type: typeFilter, search }],
    queryFn: () => financeApi.institutions({ type: typeFilter, search }),
  });

  const links = linksQuery.data?.links ?? [];
  const institutions = institutionsQuery.data?.institutions ?? [];

  function invalidateAll() {
    for (const key of ["links", "institutions", "accounts", "summary"]) {
      void queryClient.invalidateQueries({ queryKey: [key] });
    }
  }

  const connectMutation = useMutation({
    mutationFn: (institutionId: string) => financeApi.createLink(institutionId),
    onSuccess: () => {
      setError(null);
      invalidateAll();
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not connect the bank."),
  });

  const detachMutation = useMutation({
    mutationFn: (id: string) => financeApi.deleteLink(id),
    onSuccess: () => {
      setError(null);
      invalidateAll();
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not detach the bank."),
  });

  const deleteInstitutionMutation = useMutation({
    mutationFn: (id: string) => financeApi.deleteInstitution(id),
    onSuccess: () => {
      setError(null);
      invalidateAll();
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not delete the bank."),
  });

  function onDetach(id: string, name: string) {
    if (
      window.confirm(
        `Detach "${name}"?\n\nIts accounts stay with all their transactions; they just stop being linked to the bank.`,
      )
    ) {
      detachMutation.mutate(id);
    }
  }

  function onDeleteInstitution(institution: Institution) {
    if (
      window.confirm(
        `Delete "${institution.display_name}"?\n\nIts connections are removed and their accounts are detached. Transactions are never deleted.`,
      )
    ) {
      deleteInstitutionMutation.mutate(institution.id);
    }
  }

  return (
    <AppLayout>
      <div className="mx-auto max-w-5xl space-y-8">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-semibold tracking-tight">Banks</h1>
            <p className="mt-1 text-sm text-slate-400">
              Connect the banks and wallets you use, then attach accounts to them.
            </p>
          </div>
          <button className="btn-secondary" onClick={() => setCustomModal(true)}>
            + Custom bank
          </button>
        </div>

        <ErrorMessage message={error} />

        <section className="space-y-3">
          <h2 className="text-sm font-medium uppercase tracking-wide text-slate-500">My banks</h2>
          {linksQuery.isLoading ? (
            <Loading />
          ) : links.length === 0 ? (
            <EmptyState
              icon="🏦"
              title="No banks connected yet"
              description="Pick one of your banks below to create a connection, then attach your accounts to it."
            />
          ) : (
            <ul className="divide-y divide-slate-800/80 overflow-hidden rounded-2xl border border-slate-800">
              {links.map((link) => (
                <li key={link.id} className="flex items-center gap-3 bg-slate-900/40 px-4 py-3">
                  <InstitutionIcon
                    name={link.institution_name}
                    logoUrl={link.institution_logo_url}
                    icon={link.institution_icon}
                    color={link.institution_primary_color}
                    size="lg"
                  />
                  <div className="min-w-0 flex-1">
                    <div className="font-medium">{link.institution_name}</div>
                    <div className="text-xs text-slate-500">
                      {link.account_count === 0
                        ? "No accounts attached yet"
                        : `${link.account_count} account${link.account_count === 1 ? "" : "s"} attached`}
                    </div>
                  </div>
                  <button className="btn-ghost" onClick={() => setAccountModal({ linkId: link.id })}>
                    + Add account
                  </button>
                  <Link to="/accounts" className="btn-ghost">
                    Accounts
                  </Link>
                  <button
                    className="btn-ghost text-rose-400 hover:text-rose-300"
                    onClick={() => onDetach(link.id, link.institution_name)}
                    disabled={detachMutation.isPending}
                  >
                    Detach
                  </button>
                </li>
              ))}
            </ul>
          )}
        </section>

        <section className="space-y-3">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <h2 className="text-sm font-medium uppercase tracking-wide text-slate-500">Add a bank</h2>
            <div className="flex gap-2">
              <input
                className="input w-56"
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                placeholder="Search banks…"
              />
              <select
                className="input w-32"
                value={typeFilter}
                onChange={(event) => setTypeFilter(event.target.value as InstitutionType | "")}
              >
                <option value="">All types</option>
                <option value="bank">Banks</option>
                <option value="wallet">Wallets</option>
                <option value="other">Other</option>
              </select>
            </div>
          </div>

          {institutionsQuery.isLoading ? (
            <Loading />
          ) : institutions.length === 0 ? (
            <p className="text-sm text-slate-500">No banks match your search.</p>
          ) : (
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
              {institutions.map((institution) => {
                const connected = institution.link_count > 0;
                return (
                  <div
                    key={institution.id}
                    className="flex items-center gap-3 rounded-2xl border border-slate-800 bg-slate-900/40 p-4"
                  >
                    <InstitutionIcon
                      name={institution.display_name}
                      logoUrl={institution.logo_url}
                      icon={institution.icon}
                      color={institution.primary_color}
                      size="lg"
                    />
                    <div className="min-w-0 flex-1">
                      <div className="truncate font-medium">{institution.display_name}</div>
                      <div className="text-xs text-slate-500">
                        {INSTITUTION_TYPE_LABELS[institution.type]}
                        {institution.website ? ` · ${new URL(institution.website).hostname}` : ""}
                      </div>
                      {!institution.is_system && <div className="text-xs text-slate-600">Custom</div>}
                    </div>
                    {connected ? (
                      <span className="rounded-lg bg-emerald-950/60 px-2 py-1 text-xs text-emerald-400">Connected</span>
                    ) : (
                      <button
                        className="btn-primary"
                        onClick={() => connectMutation.mutate(institution.id)}
                        disabled={connectMutation.isPending}
                      >
                        Connect
                      </button>
                    )}
                    {!institution.is_system && (
                      <button
                        className="btn-ghost text-rose-400 hover:text-rose-300"
                        onClick={() => onDeleteInstitution(institution)}
                        disabled={deleteInstitutionMutation.isPending}
                        title="Delete custom bank"
                      >
                        ✕
                      </button>
                    )}
                  </div>
                );
              })}
            </div>
          )}
        </section>
      </div>

      {customModal && <InstitutionModal onClose={() => setCustomModal(false)} />}
      {accountModal && (
        <AccountModal defaultLinkId={accountModal.linkId} onClose={() => setAccountModal(null)} />
      )}
    </AppLayout>
  );
}
