/** Shared loading / empty / error states so pages stay consistent. */

export function Loading({ label = "Loading…" }: { label?: string }) {
  return <p className="text-sm text-slate-500">{label}</p>;
}

export function EmptyState({
  icon,
  title,
  description,
  children,
}: {
  icon?: string;
  title: string;
  description?: React.ReactNode;
  children?: React.ReactNode;
}) {
  return (
    <div className="rounded-2xl border border-dashed border-slate-800 bg-slate-900/30 p-10 text-center">
      {icon && <div className="text-3xl">{icon}</div>}
      <h2 className="mt-3 text-lg font-medium">{title}</h2>
      {description && <p className="mx-auto mt-1 max-w-md text-sm text-slate-400">{description}</p>}
      {children && <div className="mt-4">{children}</div>}
    </div>
  );
}

export function ErrorMessage({ message }: { message: string | null }) {
  if (!message) return null;
  return (
    <p role="alert" className="rounded-lg border border-rose-900/60 bg-rose-950/40 px-3 py-2 text-sm text-rose-300">
      {message}
    </p>
  );
}
