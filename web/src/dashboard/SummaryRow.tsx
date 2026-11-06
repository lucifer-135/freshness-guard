import { useQuery } from '@tanstack/react-query'
import { useId, type ReactNode } from 'react'
import { getSummary, type ApiError } from '@/api/client'
import type { Summary } from '@/api/types'
import { percent } from '@/claims/format'
import { statsKey } from '@/live/keys'
import { Button } from '@/components/ui/button'

const BUDGET_REACHED =
  'The daily Tavily credit budget has been reached. New claims will be checked again tomorrow, or you can raise the budget in the plugin settings.'

// Meter shows use against a budget. It turns amber at 90%, where the budget
// is about to stop new checks.
function Meter({ labelledBy, used, budget }: { labelledBy: string; used: number; budget: number }) {
  const share = budget > 0 ? Math.min(used / budget, 1) : 0
  return (
    // aria-valuenow may not exceed aria-valuemax, so it is capped; the text gives the real numbers.
    <div
      role="meter"
      aria-labelledby={labelledBy}
      aria-valuemin={0}
      aria-valuemax={budget}
      aria-valuenow={Math.min(used, budget)}
      aria-valuetext={`${used} of ${budget}`}
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
  const labelId = useId()
  return (
    <div className="py-4 lg:row-span-3 lg:grid lg:grid-rows-subgrid lg:px-6 lg:first:pl-0">
      <dt id={labelId} className="text-sm text-text-muted">
        {label}
      </dt>
      <dd className="mt-1 text-2xl font-semibold tabular-nums">{value}</dd>
      <dd className="mt-1 min-h-5 text-sm text-text-muted">
        {meter && <Meter labelledBy={labelId} used={meter.used} budget={meter.budget} />}
        {detail}
      </dd>
    </div>
  )
}

// SummaryRow shows the four numbers at the top of the dashboard (DESIGN.md section 7.2).
export function SummaryRow() {
  const summary = useQuery<Summary, ApiError>({ queryKey: [...statsKey, 'summary'], queryFn: getSummary })

  if (summary.isPending) {
    return (
      <div className="grid border-y border-border sm:grid-cols-2 lg:grid-cols-4 lg:divide-x lg:divide-border" aria-busy="true">
        {[0, 1, 2, 3].map((i) => (
          <div key={i} className="space-y-2 py-4 lg:px-6 lg:first:pl-0">
            <div className="h-4 w-48 animate-pulse rounded-sm bg-surface-2" />
            <div className="h-7 w-20 animate-pulse rounded-sm bg-surface-2" />
            <div className="h-5" />
          </div>
        ))}
        <p className="sr-only">Loading the summary.</p>
      </div>
    )
  }
  if (summary.isError) {
    return (
      <div className="flex flex-wrap items-center gap-3 border-y border-border py-4">
        <p className="text-status-outdated">The summary could not be loaded.</p>
        <Button variant="secondary" onClick={() => void summary.refetch()}>
          Try again
        </Button>
      </div>
    )
  }

  const s = summary.data
  return (
    // One strip of figures divided by hairlines; each figure reads as label, value, detail.
    <dl className="grid border-y border-border sm:grid-cols-2 lg:grid-cols-4 lg:grid-rows-[auto_auto_auto] lg:divide-x lg:divide-border">
      <Stat label="Answers checked in the last 24 hours" value={s.answers} />
      <Stat
        label="Share of checked claims that were outdated"
        value={s.checked_claims === 0 ? 'None' : percent(s.outdated_claims / s.checked_claims)}
        detail={s.checked_claims === 0 ? 'No claims have been checked yet.' : `of ${s.checked_claims} checked claims`}
      />
      <Stat
        label="Tavily credits used today"
        value={`${s.credits.used} of ${s.credits.budget}`}
        meter={s.credits}
        // DESIGN.md section 8. A budget of 0 counts as used up: no claim can be checked.
        detail={s.credits.used >= s.credits.budget ? BUDGET_REACHED : undefined}
      />
      <Stat
        label="Cached answers removed in the last 24 hours"
        value={s.cache_purge_enabled ? s.cache_purges : 'Off'}
        detail={s.cache_purge_enabled ? undefined : 'Cache purge is switched off.'}
      />
    </dl>
  )
}
