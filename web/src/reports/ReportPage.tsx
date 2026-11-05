import { useParams } from 'react-router'
import { byRisk } from '@/chat/ClaimInspector'
import { splitModel } from '@/chat/models'
import { ClaimText } from '@/claims/ClaimText'
import { ExplainCard } from '@/claims/ExplainCard'
import { formatDateTime } from '@/claims/format'
import { progressSentence } from '@/claims/progress'
import { ScoreRing } from '@/claims/ScoreRing'
import { correctedView, hasChanges } from '@/heal/corrections'
import { HealDiff } from '@/heal/HealDiff'
import { useReport } from '@/live/useReport'

// ReportPage shows one report in full and can be shared by link (DESIGN.md section 7.3).
export function ReportPage() {
  const { id = '' } = useParams()
  // A shared link to a missing report should say so at once, not after retries.
  const report = useReport(id, { notFoundRetries: 0 })

  if (report.error?.status === 404) return <p className="text-text-muted">The report was not found.</p>
  if (report.isError) return <p className="text-status-outdated">{report.error.message}</p>
  if (!report.data) return <p className="text-text-muted">Loading the report.</p>

  const r = report.data
  const checkedClaims = r.claims.filter((c) => c.checked && c.verdict !== 'NOT_CHECKED').length
  const changes = correctedView(r)
  const { provider, name } = splitModel(r.model)
  const facts: [string, string][] = [
    ['Model', r.model ? `${provider ? `${provider} / ` : ''}${name}` : 'Pasted text'],
    ['Mode', r.mode],
    ['Policy', r.policy || 'None'],
    ['Time', formatDateTime(r.created_at)],
  ]

  return (
    <section className="space-y-6">
      <header className="flex flex-wrap items-center gap-4">
        <ScoreRing score={r.status === 'complete' ? r.score : null} />
        <div>
          <h1 className="text-2xl font-semibold">Freshness report</h1>
          <p className="text-text-muted">{progressSentence(r, checkedClaims)}</p>
        </div>
      </header>

      <dl className="flex flex-wrap gap-x-8 gap-y-2 text-sm">
        {facts.map(([k, v]) => (
          <div key={k}>
            <dt className="eyebrow">{k}</dt>
            <dd className="font-mono text-xs">{v}</dd>
          </div>
        ))}
        <div>
          <dt className="eyebrow">Cost</dt>
          <dd className="font-mono text-xs">
            <span>Checked in {(r.latency_ms / 1000).toFixed(1)} s</span>
            {' · '}
            <span>{`${r.credits_used} Tavily credit${r.credits_used === 1 ? '' : 's'} used`}</span>
          </dd>
        </div>
      </dl>

      {r.blocked && (
        <p className="rounded-md border border-status-outdated p-3 text-sm">This answer was blocked before it reached the user.</p>
      )}

      <div className="grid gap-6 lg:grid-cols-2">
        <div className="space-y-2 rounded-xl border border-border bg-surface-1 p-4">
          <h2 className="eyebrow">Original answer</h2>
          <ClaimText text={r.answer_text} claims={r.claims} reportStatus={r.status} />
        </div>
        <div className="space-y-2 rounded-xl border border-border bg-surface-1 p-4">
          <h2 className="eyebrow">Corrected answer</h2>
          {hasChanges(changes) ? (
            <HealDiff report={r} segments={changes} showCorrected />
          ) : (
            <p className="text-text-muted">No claims needed correction.</p>
          )}
        </div>
      </div>

      {r.claims.length > 0 && (
        <div className="space-y-3">
          <h2 className="text-lg font-semibold">Claims</h2>
          <ul className="grid gap-4 lg:grid-cols-2">
            {byRisk(r.claims).map((c) => (
              <li key={c.id} className="rounded-xl border border-border bg-surface-1 p-4">
                <ExplainCard claim={c} report={r} />
              </li>
            ))}
          </ul>
        </div>
      )}
    </section>
  )
}
