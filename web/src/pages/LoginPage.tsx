import { useEffect, useState } from "react";
import type { FormEvent, ReactNode } from "react";
import { Navigate, useNavigate } from "react-router-dom";

import { useAuth } from "../auth/AuthContext";
import { Brand } from "../components/Brand";
import { ConnectionStatus } from "../components/ConnectionStatus";

export function LoginPage() {
  const { user, login, register } = useAuth();
  const navigate = useNavigate();
  const [mode, setMode] = useState<"login" | "register">("login");
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => { document.title = "おかね · Okane"; }, []);

  if (user) return <Navigate to="/" replace />;

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      if (mode === "login") {
        await login(email, password);
      } else {
        await register(email, password, name);
      }
      navigate("/", { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Something went wrong");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex min-h-dvh items-center justify-center px-[max(1rem,env(safe-area-inset-left),env(safe-area-inset-right))] py-[max(2rem,env(safe-area-inset-top),env(safe-area-inset-bottom))]">
      <div className="w-full max-w-sm">
        <div className="mb-8 text-center">
          <Brand stacked />
          <p className="mt-4 text-sm text-slate-400">Your money, a little clearer.</p>
          <h1 className="mt-6 text-xl font-medium">
            {mode === "login" ? "Sign in to your accounts" : "Create your account"}
          </h1>
        </div>

        <ConnectionStatus />
        <form onSubmit={onSubmit} className="space-y-4 rounded-3xl border border-slate-800 bg-slate-900/60 p-5 sm:p-6">
          {mode === "register" && (
            <Field label="Name">
              <input
                className="input"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Your name"
                autoComplete="name"
              />
            </Field>
          )}
          <Field label="Email">
            <input
              className="input"
              type="email"
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="you@example.com"
              autoComplete="email"
            />
          </Field>
          <Field label="Password">
            <input
              className="input"
              type="password"
              required
              minLength={8}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="At least 8 characters"
              autoComplete={mode === "login" ? "current-password" : "new-password"}
            />
          </Field>

          {error && (
            <p role="alert" className="rounded-lg border border-rose-900/60 bg-rose-950/40 px-3 py-2 text-sm text-rose-300">
              {error}
            </p>
          )}

          <button className="btn-primary w-full" disabled={busy} type="submit">
            {busy ? "…" : mode === "login" ? "Sign in" : "Create account"}
          </button>

          <button
            type="button"
            className="min-h-11 w-full text-center text-sm text-slate-400 hover:text-slate-200"
            onClick={() => {
              setMode(mode === "login" ? "register" : "login");
              setError("");
            }}
          >
            {mode === "login" ? "No account yet? Create one" : "Already have an account? Sign in"}
          </button>
        </form>
      </div>
    </div>
  );
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="block">
      <span className="mb-1.5 block text-sm font-medium text-slate-300">{label}</span>
      {children}
    </label>
  );
}
