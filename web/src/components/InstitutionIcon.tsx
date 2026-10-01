const SIZES = {
  sm: "h-6 w-6 text-[10px]",
  md: "h-9 w-9 text-xs",
  lg: "h-12 w-12 text-sm",
} as const;

function initials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return "?";
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
  return (parts[0][0] + parts[1][0]).toUpperCase();
}

interface InstitutionIconProps {
  name: string;
  logoUrl?: string;
  icon?: string;
  color?: string;
  size?: keyof typeof SIZES;
  className?: string;
}

/** Bank/wallet avatar: real logo on a light tile, emoji, or colored initials. */
export function InstitutionIcon({ name, logoUrl, icon, color, size = "md", className }: InstitutionIconProps) {
  const base = `flex shrink-0 items-center justify-center overflow-hidden rounded-full ring-1 ring-slate-700/60 ${SIZES[size]} ${className ?? ""}`;

  if (logoUrl) {
    return (
      <span className={`${base} bg-white`}>
        <img src={logoUrl} alt="" className="h-full w-full object-contain p-0.5" loading="lazy" />
      </span>
    );
  }
  if (icon) {
    return (
      <span className={base} style={{ backgroundColor: color || "#334155" }}>
        {icon}
      </span>
    );
  }
  return (
    <span className={base} style={{ backgroundColor: color || "#334155" }}>
      <span className="font-semibold text-white">{initials(name)}</span>
    </span>
  );
}
