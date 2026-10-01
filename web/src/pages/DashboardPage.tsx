import { AppLayout } from "../components/AppLayout";

const CARDS = [
  { label: "Net this month", value: "—", tone: "text-slate-100" },
  { label: "Income", value: "—", tone: "text-emerald-400" },
  { label: "Expenses", value: "—", tone: "text-rose-400" },
  { label: "Net worth", value: "—", tone: "text-slate-100" },
];

export function DashboardPage() {
  return (
    <AppLayout>
      <div className="mx-auto max-w-5xl space-y-6">
        <h1 className="text-2xl font-semibold tracking-tight">Overview</h1>

        <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
          {CARDS.map((card) => (
            <div key={card.label} className="rounded-2xl border border-slate-800 bg-slate-900/60 p-4">
              <div className="text-xs font-medium uppercase tracking-wide text-slate-500">{card.label}</div>
              <div className={`mt-2 text-2xl font-semibold tabular-nums ${card.tone}`}>{card.value}</div>
            </div>
          ))}
        </div>

        <div className="rounded-2xl border border-dashed border-slate-800 bg-slate-900/30 p-10 text-center">
          <div className="text-3xl">🏦</div>
          <h2 className="mt-3 text-lg font-medium">No data yet</h2>
          <p className="mx-auto mt-1 max-w-md text-sm text-slate-400">
            The next milestones wire up accounts, transactions and bank sync. Once a bank is connected,
            your balances and spending by category will show up here.
          </p>
        </div>

        <div className="grid gap-4 md:grid-cols-3">
          {[
            { icon: "🔗", title: "Connect banks", body: "Belvo sandbox first, then your real Mexican banks." },
            { icon: "🏷️", title: "Auto-categorize", body: "Rules first, AI for the tricky ones." },
            { icon: "✨", title: "AI insights", body: "Monthly digest and savings suggestions from your data." },
          ].map((f) => (
            <div key={f.title} className="rounded-2xl border border-slate-800 bg-slate-900/60 p-4">
              <div className="text-xl">{f.icon}</div>
              <div className="mt-2 font-medium">{f.title}</div>
              <div className="mt-1 text-sm text-slate-400">{f.body}</div>
            </div>
          ))}
        </div>
      </div>
    </AppLayout>
  );
}
