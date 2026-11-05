import { useEffect, type ReactNode } from "react";
import { Link, NavLink, useLocation, useNavigate } from "react-router-dom";

import { useAuth } from "../auth/AuthContext";
import { useI18n } from "../i18n/I18nContext";
import type { TranslationKey } from "../lib/i18n";
import { Brand } from "./Brand";
import { ConnectionStatus } from "./ConnectionStatus";
import { LanguageSwitcher } from "./LanguageSwitcher";
import { NavIcon } from "./NavIcon";

const NAV: ReadonlyArray<{
  to: string;
  labelKey: TranslationKey;
  mobileLabelKey: TranslationKey;
  icon: "overview" | "transactions" | "accounts" | "banks" | "rules";
}> = [
  { to: "/", labelKey: "nav.overview", mobileLabelKey: "nav.overview", icon: "overview" },
  {
    to: "/transactions",
    labelKey: "nav.transactions",
    mobileLabelKey: "nav.activity",
    icon: "transactions",
  },
  {
    to: "/accounts",
    labelKey: "nav.accounts",
    mobileLabelKey: "nav.accounts",
    icon: "accounts",
  },
  { to: "/banks", labelKey: "nav.banks", mobileLabelKey: "nav.banks", icon: "banks" },
  { to: "/rules", labelKey: "nav.rules", mobileLabelKey: "nav.rules", icon: "rules" },
];

export function AppLayout({ children }: { children: ReactNode }) {
  const { user, logout } = useAuth();
  const { language, t } = useI18n();
  const navigate = useNavigate();
  const { pathname } = useLocation();

  useEffect(() => {
    const active = NAV.find((item) => item.to === pathname);
    const pageTitle = active ? t(active.labelKey) : "Okane";
    document.title = `${pageTitle} · おかね`;
  }, [pathname, language, t]);

  async function onLogout() {
    await logout();
    navigate("/login", { replace: true });
  }

  return (
    <div className="app-shell flex">
      <a
        href="#main-content"
        className="sr-only z-50 rounded-lg bg-emerald-400 p-3 text-slate-950 focus:not-sr-only focus:absolute focus:left-4 focus:top-4"
      >
        {t("layout.skipToContent")}
      </a>
      <aside className="sticky top-0 hidden h-dvh w-60 shrink-0 flex-col border-r border-slate-800/80 bg-slate-950/65 px-4 pb-6 pt-[max(2rem,env(safe-area-inset-top))] backdrop-blur-xl lg:flex">
        <Link
          to="/"
          aria-label={t("layout.okaneOverview")}
          className="mb-10 self-start px-2"
        >
          <Brand />
        </Link>
        <div className="mb-3 px-3 text-[10px] font-medium uppercase tracking-[0.2em] text-slate-500">
          {t("layout.yourFinances")}
        </div>
        <nav aria-label={t("layout.mainNavigation")} className="space-y-1.5">
          {NAV.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.to === "/"}
              className={({ isActive }) =>
                `flex min-h-12 items-center gap-3 rounded-xl px-3 text-sm transition ${isActive ? "border border-emerald-500/25 bg-emerald-400/12 font-medium text-emerald-300 shadow-xs shadow-black/20" : "border border-transparent text-slate-400 hover:bg-slate-800/60 hover:text-slate-100"}`
              }
            >
              <NavIcon name={item.icon} />
              {t(item.labelKey)}
            </NavLink>
          ))}
        </nav>
        <div className="mt-auto px-3 pt-8">
          <span
            className="brand-mark mb-3 size-7 text-emerald-400/40"
            aria-hidden="true"
          />
          <p className="text-xs leading-relaxed text-slate-500">
            {t("layout.taglineLine1")}
            <br />
            {t("layout.taglineLine2")}
          </p>
        </div>
      </aside>

      <div className="flex min-w-0 flex-1 flex-col">
        <header className="app-header sticky top-0 z-20 flex min-h-18 items-center justify-between gap-2 border-b border-slate-800/80 bg-slate-950/85 px-3.5 pb-3 backdrop-blur-xl sm:gap-3 sm:px-6 lg:px-8">
          <Link
            to="/"
            aria-label={t("layout.okaneOverview")}
            className="shrink-0 lg:hidden"
          >
            <Brand />
          </Link>
          <div className="hidden min-w-0 text-sm text-slate-400 sm:block">
            <span className="block text-[10px] uppercase tracking-[0.18em] text-slate-500">
              {t("layout.clearerView")}
            </span>
            <span className="block truncate">
              {user?.name
                ? t("layout.welcomeBackName", { name: user.name })
                : t("layout.welcomeBack")}
            </span>
          </div>
          <div className="ml-auto flex shrink-0 items-center gap-1.5 sm:gap-2">
            <LanguageSwitcher />
            <button
              onClick={onLogout}
              className="btn-ghost shrink-0 gap-2 px-2.5 sm:px-3"
              aria-label={t("layout.signOut")}
            >
              <NavIcon name="logout" />
              <span className="hidden sm:inline">{t("layout.signOut")}</span>
            </button>
          </div>
        </header>
        <ConnectionStatus />
        <main
          id="main-content"
          className="app-main min-w-0 flex-1 px-3.5 pt-5 sm:px-6 sm:pt-8 lg:px-8"
        >
          {children}
        </main>
      </div>

      <nav
        aria-label={t("layout.mobileNavigation")}
        className="mobile-nav fixed inset-x-0 bottom-0 z-30 border-t border-slate-800/90 bg-slate-950/90 backdrop-blur-xl lg:hidden"
      >
        <div className="mx-auto grid max-w-xl grid-cols-5 gap-0.5 sm:gap-1">
          {NAV.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.to === "/"}
              aria-label={t(item.labelKey)}
              className={({ isActive }) =>
                `flex min-h-14 min-w-0 flex-col items-center justify-center gap-1 rounded-xl px-1 text-[10px] font-medium leading-tight transition sm:text-[11px] ${isActive ? "bg-emerald-400/12 text-emerald-300" : "text-slate-400 hover:bg-slate-900"}`
              }
            >
              <NavIcon name={item.icon} />
              <span className="max-w-full truncate">{t(item.mobileLabelKey)}</span>
            </NavLink>
          ))}
        </div>
      </nav>
    </div>
  );
}
