// Logo is the mark and the name. The mark is the score ring, drained at the top
// right like a fact losing its freshness, ending in a dot that marks today, as on
// the decay curve. It uses the text colour only, because saturated colour is
// reserved for verdicts.
export function Logo({ className = '' }: { className?: string }) {
  return (
    <span data-logo className={`inline-flex items-center gap-2 font-semibold tracking-[-0.01em] ${className}`}>
      <svg aria-hidden="true" viewBox="0 0 24 24" className="size-5 shrink-0" fill="none" stroke="currentColor" strokeLinecap="butt">
        <path d="M20.08 9.37A8.5 8.5 0 1 1 12 3.5" strokeWidth="2.5" strokeLinecap="round" />
        <circle cx="20.08" cy="9.37" r="2.1" fill="currentColor" stroke="none" />
      </svg>
      Freshness Guard
    </span>
  )
}
