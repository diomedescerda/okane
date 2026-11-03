const paths = {
  overview: (
    <>
      <rect x="3" y="3" width="7" height="7" rx="2" />
      <rect x="14" y="3" width="7" height="7" rx="2" />
      <rect x="3" y="14" width="7" height="7" rx="2" />
      <rect x="14" y="14" width="7" height="7" rx="2" />
    </>
  ),
  transactions: <path d="M7 3v17m-4-4 4 4 4-4M17 21V4m-4 4 4-4 4 4" />,
  accounts: (
    <>
      <rect x="3" y="5" width="18" height="15" rx="3" />
      <path d="M3 9h18m-6 5h3" />
    </>
  ),
  banks: <path d="m3 9 9-6 9 6H3Zm0 12h18M6 12v6m6-6v6m6-6v6" />,
  rules: (
    <>
      <path d="M4 6h16M4 12h16M4 18h16" />
      <circle cx="9" cy="6" r="2" />
      <circle cx="15" cy="12" r="2" />
      <circle cx="9" cy="18" r="2" />
    </>
  ),
  logout: (
    <path d="M9 4H5a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h4m5-12 4 4-4 4m-6-4h12" />
  ),
} as const;

export function NavIcon({ name }: { name: keyof typeof paths }) {
  return (
    <svg
      aria-hidden="true"
      width="20"
      height="20"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      {paths[name]}
    </svg>
  );
}
