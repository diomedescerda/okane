import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Link } from "react-router-dom";

import { financeApi, type Institution, type InstitutionType } from "../api/finance";
import { AccountModal } from "../components/AccountModal";
import { AppLayout } from "../components/AppLayout";
import { InstitutionIcon } from "../components/InstitutionIcon";
import { InstitutionModal } from "../components/InstitutionModal";
import { EmptyState, ErrorMessage, Loading } from "../components/State";
import { useI18n } from "../i18n/I18nContext";

export function BanksPage() {
  const queryClient = useQueryClient();
  const { t, tInstitutionType, tError } = useI18n();
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
    onError: (err) => setError(tError(err, "banks.connectFailed")),
  });

  const detachMutation = useMutation({
    mutationFn: (id: string) => financeApi.deleteLink(id),
    onSuccess: () => {
      setError(null);
      invalidateAll();
    },
    onError: (err) => setError(tError(err, "banks.detachFailed")),
  });

  const deleteInstitutionMutation = useMutation({
    mutationFn: (id: string) => financeApi.deleteInstitution(id),
    onSuccess: () => {
      setError(null);
      invalidateAll();
    },
    onError: (err) => setError(tError(err, "banks.deleteCustomFailed")),
  });

  function onDetach(id: string, name: string) {
    if (window.confirm(t("banks.detachConfirm", { name }))) {
      detachMutation.mutate(id);
    }
  }

  function onDeleteInstitution(institution: Institution) {
    if (window.confirm(t("banks.deleteCustomConfirm", { name: institution.display_name }))) {
      deleteInstitutionMutation.mutate(institution.id);
    }
  }

  return (
    <AppLayout>
      <div className="mx-auto max-w-5xl space-y-8 animate-fade-up">
        <div className="page-header">
          <div>
            <h1 className="text-2xl font-semibold tracking-tight">{t("nav.banks")}</h1>
            <p className="mt-1 text-sm text-slate-400">{t("banks.subtitle")}</p>
          </div>
          <button className="btn-secondary" onClick={() => setCustomModal(true)}>
            {t("banks.customBankButton")}
          </button>
        </div>

        <ErrorMessage message={error} />

        <section className="space-y-3">
          <h2 className="text-sm font-medium uppercase tracking-wide text-slate-500">
            {t("banks.myBanks")}
          </h2>
          {linksQuery.isLoading ? (
            <Loading />
          ) : links.length === 0 ? (
            <EmptyState
              icon="🏦"
              title={t("banks.emptyTitle")}
              description={t("banks.emptyDescription")}
            />
          ) : (
            <ul className="surface-card divide-y divide-slate-800/80 overflow-hidden rounded-2xl">
              {links.map((link) => (
                <li
                  key={link.id}
                  className="flex flex-wrap items-center gap-3 px-4 py-4"
                >
                  <InstitutionIcon
                    name={link.institution_name}
                    logoUrl={link.institution_logo_url}
                    icon={link.institution_icon}
                    color={link.institution_primary_color}
                    size="lg"
                  />
                  <div className="min-w-0 flex-1 basis-[calc(100%-4rem)] md:basis-0">
                    <div className="break-words font-medium">{link.institution_name}</div>
                    <div className="text-xs text-slate-500">
                      {link.account_count === 0
                        ? t("banks.noAccountsAttached")
                        : link.account_count === 1
                          ? t("banks.accountsAttached.one", { count: link.account_count })
                          : t("banks.accountsAttached.other", { count: link.account_count })}
                    </div>
                  </div>
                  <div className="flex w-full flex-wrap items-center justify-end gap-1 border-t border-slate-800/70 pt-2.5 md:w-auto md:border-t-0 md:pt-0">
                    <button
                      className="btn-ghost"
                      onClick={() => setAccountModal({ linkId: link.id })}
                    >
                      {t("banks.addAccountButton")}
                    </button>
                    <Link to="/accounts" className="btn-ghost">
                      {t("nav.accounts")}
                    </Link>
                    <button
                      className="btn-ghost text-rose-400 hover:text-rose-300"
                      onClick={() => onDetach(link.id, link.institution_name)}
                      disabled={detachMutation.isPending}
                    >
                      {t("banks.detach")}
                    </button>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </section>

        <section className="space-y-3">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <h2 className="text-sm font-medium uppercase tracking-wide text-slate-500">
              {t("banks.addABank")}
            </h2>
            <div className="grid w-full min-w-0 grid-cols-1 gap-2 min-[420px]:grid-cols-[minmax(0,1fr)_8rem] sm:w-auto">
              <input
                className="input sm:w-56"
                aria-label={t("banks.searchLabel")}
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                placeholder={t("banks.searchPlaceholder")}
              />
              <select
                className="input"
                aria-label={t("banks.bankTypeAria")}
                value={typeFilter}
                onChange={(event) => setTypeFilter(event.target.value as InstitutionType | "")}
              >
                <option value="">{t("banks.filterAll")}</option>
                <option value="bank">{t("banks.filterBank")}</option>
                <option value="wallet">{t("banks.filterWallet")}</option>
                <option value="other">{t("banks.filterOther")}</option>
              </select>
            </div>
          </div>

          {institutionsQuery.isLoading ? (
            <Loading />
          ) : institutions.length === 0 ? (
            <p className="text-sm text-slate-500">{t("banks.noMatches")}</p>
          ) : (
            <div className="grid gap-3 md:grid-cols-2 2xl:grid-cols-3">
              {institutions.map((institution) => {
                const connected = institution.link_count > 0;
                return (
                  <div
                    key={institution.id}
                    className="surface-card flex min-w-0 flex-wrap items-center gap-3 rounded-2xl p-4"
                  >
                    <InstitutionIcon
                      name={institution.display_name}
                      logoUrl={institution.logo_url}
                      icon={institution.icon}
                      color={institution.primary_color}
                      size="lg"
                    />
                    <div className="min-w-0 flex-1 basis-[calc(100%-4rem)]">
                      <div className="truncate font-medium">{institution.display_name}</div>
                      <div className="break-words text-xs text-slate-500">
                        {tInstitutionType(institution.type)}
                        {institution.website
                          ? ` · ${new URL(institution.website).hostname}`
                          : ""}
                      </div>
                      {!institution.is_system && (
                        <div className="text-xs text-slate-600">{t("banks.customBadge")}</div>
                      )}
                    </div>
                    {connected ? (
                      <span className="rounded-lg bg-emerald-950/60 px-2 py-1 text-xs text-emerald-400">
                        {t("banks.connectedBadge")}
                      </span>
                    ) : (
                      <button
                        className="btn-primary"
                        onClick={() => connectMutation.mutate(institution.id)}
                        disabled={connectMutation.isPending}
                      >
                        {t("banks.connectButton")}
                      </button>
                    )}
                    {!institution.is_system && (
                      <button
                        className="btn-ghost text-rose-400 hover:text-rose-300"
                        onClick={() => onDeleteInstitution(institution)}
                        disabled={deleteInstitutionMutation.isPending}
                        title={t("banks.deleteCustomAria")}
                        aria-label={`${t("common.delete")} ${institution.display_name}`}
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
