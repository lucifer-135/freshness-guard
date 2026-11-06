// CreditMeter shows today's Tavily credit use against the daily budget.
export function CreditMeter({ used, budget }: { used: number; budget: number }) {
  const label = `Tavily credits today: ${used} of ${budget}`
  const share = budget > 0 ? Math.min(used / budget, 1) : 0
  return (
    <div className="flex items-center gap-2">
      <svg role="img" aria-label={label} width="64" height="6" viewBox="0 0 64 6" className="shrink-0">
        <rect width="64" height="6" rx="3" fill="var(--surface-3)" />
        {/* Neutral fill: blue is for focus, links and selection; amber warns that the budget is nearly used. */}
        <rect width={64 * share} height="6" rx="3" fill={share >= 0.9 ? 'var(--status-unverified)' : 'var(--text-muted)'} />
      </svg>
      <span aria-hidden="true" className="text-xs text-text-muted">
        Credits <span className="font-mono tabular-nums">{`${used} of ${budget}`}</span>
      </span>
    </div>
  )
}
