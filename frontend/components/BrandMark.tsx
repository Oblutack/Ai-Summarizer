// The Inkling mark: an "i" (a dot and a stem) in paper colour on an ink square. It follows the theme.
export default function BrandMark({ className = "h-8 w-8" }: { className?: string }) {
  return (
    <svg viewBox="0 0 32 32" className={className} aria-hidden="true" focusable="false">
      <rect width="32" height="32" rx="7" fill="rgb(var(--ink))" />
      <circle cx="16" cy="9.5" r="2.6" fill="rgb(var(--canvas))" />
      <rect x="13.4" y="14" width="5.2" height="10.5" rx="1.5" fill="rgb(var(--canvas))" />
    </svg>
  );
}
