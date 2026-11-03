import { useEffect, type ReactNode } from "react";
import { Link, NavLink, useLocation, useNavigate } from "react-router-dom";

import { useAuth } from "../auth/AuthContext";
import { Brand } from "./Brand";
import { NavIcon } from "./NavIcon";
import { ConnectionStatus } from "./ConnectionStatus";

const NAV = [
  { to: "/", label: "Overview", mobileLabel: "Overview", icon: "overview" },
  {
    to: "/transactions",
    label: "Transactions",
    mobileLabel: "Activity",
    icon: "transactions",
  },
  {
    to: "/accounts",
    label: "Accounts",
    mobileLabel: "Accounts",
    icon: "accounts",
  },
  { to: "/banks", label: "Banks", mobileLabel: "Banks", icon: "banks" },
  { to: "/rules", label: "Rules", mobileLabel: "Rules", icon: "rules" },
] as const;

export function AppLayout({ children }: { children: ReactNode }) {
  const { user, logout } = useAuth();
  const navigate = useNavigate();
  const { pathname } = useLocation();
  useEffect(() => {
    document.title = `${NAV.find((item) => item.to === pathname)?.label ?? "Okane"} · おかね`;
  }, [pathname]);

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
        Skip to content
      </a>
      <aside className="sticky top-0 hidden h-dvh w-60 shrink-0 flex-col border-r border-slate-800 bg-slate-900/35 px-4 pb-6 pt-[max(2rem,env(safe-area-inset-top))] lg:flex">
        <Link
          to="/"
          aria-label="Okane overview"
          className="mb-10 self-start px-2"
        >
          <Brand />
        </Link>
        <div className="mb-3 px-3 text-[10px] font-medium uppercase tracking-[0.2em] text-slate-500">
          Your finances
        </div>
        <nav aria-label="Main navigation" className="space-y-1.5">
          {NAV.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.to === "/"}
              className={({ isActive }) =>
                `flex min-h-12 items-center gap-3 rounded-xl px-3 text-sm transition ${isActive ? "bg-emerald-400/10 font-medium text-emerald-300" : "text-slate-400 hover:bg-slate-800/70 hover:text-slate-100"}`
              }
            >
              <NavIcon name={item.icon} />
              {item.label}
            </NavLink>
          ))}
        </nav>
        <div className="mt-auto px-3 pt-8">
          <span
            className="brand-mark mb-3 size-7 text-emerald-400/40"
            aria-hidden="true"
          />
          <p className="text-xs leading-relaxed text-slate-500">
            A little clarity.
            <br />A little more peace of mind.
          </p>
        </div>
      </aside>

      <div className="flex min-w-0 flex-1 flex-col">
        <header className="app-header sticky top-0 z-20 flex min-h-19 items-center justify-between gap-3 border-b border-slate-800 bg-slate-950/95 px-4 pb-3 backdrop-blur-md sm:px-6 lg:px-8">
          <Link
            to="/"
            aria-label="Okane overview"
            className="shrink-0 lg:hidden"
          >
            <Brand />
          </Link>
          <div className="hidden min-w-0 text-sm text-slate-400 sm:block">
            <span className="block text-[10px] uppercase tracking-[0.18em] text-slate-500">
              A clearer view
            </span>
            <span className="block truncate">
              {user?.name ? `Welcome back, ${user.name}` : "Welcome back"}
            </span>
          </div>
          <button
            onClick={onLogout}
            className="btn-ghost ml-auto shrink-0 gap-2"
            aria-label="Sign out"
          >
            <NavIcon name="logout" />
            <span className="hidden sm:inline">Sign out</span>
          </button>
        </header>
        <ConnectionStatus />
        <main
          id="main-content"
          className="app-main min-w-0 flex-1 px-4 pt-6 sm:px-6 sm:pt-8 lg:px-8"
        >
          {children}
        </main>
      </div>

      <nav
        aria-label="Mobile navigation"
        className="mobile-nav fixed inset-x-0 bottom-0 z-30 border-t border-slate-700 bg-slate-950/95 backdrop-blur-md lg:hidden"
      >
        <div className="mx-auto grid max-w-xl grid-cols-5 gap-1">
          {NAV.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.to === "/"}
              aria-label={item.label}
              className={({ isActive }) =>
                `flex min-h-14 min-w-0 flex-col items-center justify-center gap-1 rounded-xl text-[11px] font-medium transition ${isActive ? "bg-emerald-400/10 text-emerald-300" : "text-slate-400 hover:bg-slate-900"}`
              }
            >
              <NavIcon name={item.icon} />
              <span>{item.mobileLabel}</span>
            </NavLink>
          ))}
        </div>
      </nav>
    </div>
  );
}
