import type { ReactNode } from "react";
import { NavLink, useNavigate } from "react-router-dom";

import { useAuth } from "../auth/AuthContext";

const NAV = [
  { to: "/", label: "Overview", enabled: true },
  { to: "/transactions", label: "Transactions", enabled: true },
  { to: "/accounts", label: "Accounts", enabled: true },
  { to: "/banks", label: "Banks", enabled: true },
  { to: "/budgets", label: "Budgets", enabled: false },
  { to: "/insights", label: "Insights", enabled: false },
];

export function AppLayout({ children }: { children: ReactNode }) {
  const { user, logout } = useAuth();
  const navigate = useNavigate();

  async function onLogout() {
    await logout();
    navigate("/login", { replace: true });
  }

  return (
    <div className="flex min-h-screen">
      <aside className="hidden w-56 shrink-0 border-r border-slate-800 p-4 md:block">
        <div className="px-2 pb-6 pt-1 text-xl font-semibold tracking-tight">
          okane<span className="text-emerald-400">.</span>
        </div>
        <nav className="space-y-1">
          {NAV.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              className={({ isActive }) =>
                `flex items-center justify-between rounded-lg px-3 py-2 text-sm ${
                  item.enabled
                    ? isActive
                      ? "bg-slate-800 text-white"
                      : "text-slate-400 hover:bg-slate-900 hover:text-slate-200"
                    : "pointer-events-none text-slate-600"
                }`
              }
            >
              {item.label}
              {!item.enabled && (
                <span className="rounded bg-slate-900 px-1.5 py-0.5 text-[10px] uppercase tracking-wide text-slate-600">
                  soon
                </span>
              )}
            </NavLink>
          ))}
        </nav>
      </aside>

      <div className="flex min-w-0 flex-1 flex-col">
        <header className="flex items-center justify-between border-b border-slate-800 px-6 py-3">
          <div className="text-sm text-slate-400">
            {user?.name ? `Welcome back, ${user.name}` : "Welcome back"}
          </div>
          <button onClick={onLogout} className="text-sm text-slate-400 hover:text-slate-200">
            Sign out
          </button>
        </header>
        <main className="flex-1 p-6">{children}</main>
      </div>
    </div>
  );
}
