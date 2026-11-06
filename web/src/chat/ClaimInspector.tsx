import type { Claim, Report, Verdict } from '@/api/types'
import { ExplainCard } from '@/claims/ExplainCard'
import { ScoreRing } from '@/claims/ScoreRing'
import { segmentAnswer } from '@/claims/segments'
import { STATUS_META, uiStatus } from '@/claims/status'
import StatusMark from '@/components/reactbits/StatusMark'

const rank: Record<Verdict, number> = { WRONG: 0, STALE: 1, UNVERIFIABLE: 2, FRESH: 3, NOT_CHECKED: 4 }

// eslint-disable-next-line react-refresh/only-export-components -- the report page sorts claims the same way
export function byRisk(claims: Claim[]): Claim[] {
  return [...claims].sort((a, b) => rank[a.verdict] - rank[b.verdict] || b.p_changed - a.p_changed)
}

interface ClaimInspectorProps {
  report: Report
  // The claim whose full explanation is open, if any.
  selectedId: string | null
  onSelect(claimId: string): void
}

// ClaimInspector lists a message's claims, riskiest first (DESIGN.md section 7.1).
// The selected claim opens its full explanation under its row.
export function ClaimInspector({ report, selectedId, onSelect }: ClaimInspectorProps) {
  const unplaced = new Set(segmentAnswer(report.answer_text, report.claims).unplaced.map((c) => c.id))
  if (report.claims.length === 0) return <p className="text-sm text-text-muted">This answer has no claims to list.</p>
  return (
    <ul className="divide-y divide-border overflow-hidden rounded-lg border border-border">
      {byRisk(report.claims).map((c) => {
        const status = uiStatus(c, report.status)
        const meta = STATUS_META[status]
        const selected = c.id === selectedId
        const outdated = c.verdict === 'STALE' || c.verdict === 'WRONG'
        return (
          <li key={c.id}>
            <button
              type="button"
              onClick={() => onSelect(c.id)}
              disabled={unplaced.has(c.id)}
              aria-expanded={selected}
              className="flex w-full items-start gap-3 p-3 text-left transition-colors duration-[var(--dur-hover)] focus-visible:outline-offset-[-4px] enabled:hover:bg-surface-2 aria-expanded:bg-accent-soft aria-expanded:shadow-[inset_2px_0_0_var(--accent)] disabled:cursor-default"
            >
              {status === 'pending' ? (
                <span aria-hidden="true" className="inline-flex size-7 shrink-0 items-center justify-center">
                  <StatusMark status="running" size={28} color="var(--text-muted)" />
                </span>
              ) : (
                <ScoreRing score={c.score} size="sm" />
              )}
              <span className="min-w-0 space-y-0.5 text-sm">
                <span className="block [overflow-wrap:anywhere]">{c.quote}</span>
                <span className="block text-xs text-text-muted">{meta.label}</span>
                {outdated && c.current_value && (
                  <span className="block font-mono text-xs text-text-muted [overflow-wrap:anywhere]">{`${c.value} → ${c.current_value}`}</span>
                )}
                {unplaced.has(c.id) && (
                  <span className="block text-xs text-text-subtle">This claim could not be located in the answer text.</span>
                )}
              </span>
            </button>
            {selected && (
              <div className="border-t border-border bg-surface-1 p-3">
                <ExplainCard claim={c} report={report} />
              </div>
            )}
          </li>
        )
      })}
    </ul>
  )
}
