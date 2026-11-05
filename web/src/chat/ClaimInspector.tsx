import type { Claim, Report, Verdict } from '@/api/types'
import { ScoreRing } from '@/claims/ScoreRing'
import { segmentAnswer } from '@/claims/segments'
import { STATUS_META, uiStatus } from '@/claims/status'

const rank: Record<Verdict, number> = { WRONG: 0, STALE: 1, UNVERIFIABLE: 2, FRESH: 3, NOT_CHECKED: 4 }

// eslint-disable-next-line react-refresh/only-export-components -- the report page sorts claims the same way
export function byRisk(claims: Claim[]): Claim[] {
  return [...claims].sort((a, b) => rank[a.verdict] - rank[b.verdict] || b.p_changed - a.p_changed)
}

// ClaimInspector lists a message's claims, riskiest first (DESIGN.md section 7.1).
export function ClaimInspector({ report, onSelect }: { report: Report; onSelect(claimId: string): void }) {
  const unplaced = new Set(segmentAnswer(report.answer_text, report.claims).unplaced.map((c) => c.id))
  if (report.claims.length === 0) return <p className="text-sm text-text-muted">This answer has no claims to list.</p>
  return (
    <ul className="space-y-2">
      {byRisk(report.claims).map((c) => {
        const meta = STATUS_META[uiStatus(c, report.status)]
        return (
          <li key={c.id}>
            <button
              type="button"
              onClick={() => onSelect(c.id)}
              disabled={unplaced.has(c.id)}
              className="flex w-full items-start gap-3 rounded-md border border-border bg-surface-2 p-2 text-left transition-colors duration-[var(--dur-hover)] hover:border-border-strong disabled:cursor-default"
            >
              <ScoreRing score={c.score} size="sm" />
              <span className="min-w-0 space-y-0.5 text-sm">
                <span className="block [overflow-wrap:anywhere]">{c.quote}</span>
                <span className="block text-xs text-text-muted">{meta.label}</span>
                {unplaced.has(c.id) && (
                  <span className="block text-xs text-text-subtle">This claim could not be located in the answer text.</span>
                )}
              </span>
            </button>
          </li>
        )
      })}
    </ul>
  )
}
