export function Brand({ stacked = false }: { stacked?: boolean }) {
  return (
    <span
      role="img"
      aria-label="Okane — おかね"
      className={`inline-flex ${stacked ? "flex-col gap-5" : "gap-2.5 sm:gap-3"} items-center`}
    >
      <span
        aria-hidden="true"
        className={`grid shrink-0 place-items-center bg-[var(--brand-paper)] text-[#073b21] shadow-sm shadow-black/40 ${stacked ? "size-24 rounded-[1.75rem]" : "size-10 rounded-xl sm:size-11"}`}
      >
        <span className={`brand-mark ${stacked ? "size-19" : "size-8 sm:size-9"}`} />
      </span>
      <span
        aria-hidden="true"
        className={`brand-wordmark text-slate-100 ${stacked ? "h-12 w-36" : "h-7 w-20 sm:h-8 sm:w-24"}`}
      />
    </span>
  );
}
