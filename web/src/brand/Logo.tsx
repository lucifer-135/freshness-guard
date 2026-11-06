// Logo is the mark and the name. The mark is the score ring: a faint full track,
// the remaining freshness drawn over it and drained at the top right, ending in a
// dot that marks today, as on the decay curve. The track keeps it a gauge, not a
// loading spinner. It uses the text colour only, because saturated colour is
// reserved for verdicts.
export function Logo({ className = '' }: { className?: string }) {
  return (
    <span data-logo className={`inline-flex items-center gap-2 font-semibold tracking-[-0.01em] ${className}`}>
      <svg aria-hidden="true" viewBox="0 0 24 24" className="size-5 shrink-0" fill="none" stroke="currentColor" strokeLinecap="butt">
        <circle cx="12" cy="12" r="8.5" strokeWidth="2.5" opacity="0.3" />
        <path d="M20.08 9.37A8.5 8.5 0 1 1 12 3.5" strokeWidth="2.5" strokeLinecap="round" />
        <circle cx="20.08" cy="9.37" r="2.1" fill="currentColor" stroke="none" />
      </svg>
      Freshness Guard
    </span>
  )
}
