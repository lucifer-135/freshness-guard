import { useQuery } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { getSummary, type ApiError } from '@/api/client'
import type { Summary } from '@/api/types'
import { percent } from '@/claims/format'
import { statsKey } from '@/live/keys'
import { SpotlightCard } from '@/motion/SpotlightCard'
import { Button } from '@/components/ui/button'

// Meter shows use against a budget. It turns amber at 90%, where the budget
// is about to stop new checks.
function Meter({ label, used, budget }: { label: string; used: number; budget: number }) {
  const share = budget > 0 ? Math.min(used / budget, 1) : 0
  return (
    <div
      role="meter"
      aria-label={label}
      aria-valuemin={0}
      aria-valuemax={budget}
      aria-valuenow={used}
      className="mt-2 h-1.5 overflow-hidden rounded-full bg-surface-3"
    >
      <div
        className="h-full rounded-full"
        style={{ width: `${share * 100}%`, backgroundColor: share >= 0.9 ? 'var(--status-unverified)' : 'var(--accent)' }}
      />
    </div>
  )
}

function Stat({
  label,
  value,
  detail,
  meter,
}: {
  label: string
  value: ReactNode
  detail?: string
  meter?: { used: number; budget: number }
}) {
  return (
    <SpotlightCard>
      <p className="text-sm text-text-muted">{label}</p>
      <p className="mt-1 text-3xl font-semibold tabular-nums">{value}</p>
      {meter && <Meter label={label} used={meter.used} budget={meter.budget} />}
      <p className="mt-1 min-h-5 text-sm text-text-muted">{detail}</p>
    </SpotlightCard>
  )
}

// SummaryRow shows the four numbers at the top of the dashboard (DESIGN.md section 7.2).
export function SummaryRow() {
  const summary = useQuery<Summary, ApiError>({ queryKey: [...statsKey, 'summary'], queryFn: getSummary })

  if (summary.isPending) {
    return (
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4" aria-busy="true">
        {[0, 1, 2, 3].map((i) => (
          <div key={i} className="h-28 animate-pulse rounded-xl border border-border bg-surface-2" />
        ))}
        <p className="sr-only">Loading the summary.</p>
      </div>
    )
  }
  if (summary.isError) {
    return (
      <div className="flex flex-wrap items-center gap-3 rounded-xl border border-border bg-surface-2 p-4">
        <p className="text-status-outdated">The summary could not be loaded.</p>
        <Button variant="secondary" onClick={() => void summary.refetch()}>
          Try again
        </Button>
      </div>
    )
  }

  const s = summary.data
  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
      <Stat label="Answers checked in the last 24 hours" value={s.answers} />
      <Stat
        label="Share of checked claims that were outdated"
        value={s.checked_claims === 0 ? 'None' : percent(s.outdated_claims / s.checked_claims)}
        detail={s.checked_claims === 0 ? 'No claims have been checked yet.' : `of ${s.checked_claims} checked claims`}
      />
      <Stat label="Tavily credits used today" value={`${s.credits.used} of ${s.credits.budget}`} meter={s.credits} />
      <Stat
        label="Cached answers removed in the last 24 hours"
        value={s.cache_purge_enabled ? s.cache_purges : 'Off'}
        detail={s.cache_purge_enabled ? undefined : 'Cache purge is switched off.'}
      />
    </div>
  )
}
