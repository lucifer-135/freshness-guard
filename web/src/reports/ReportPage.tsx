import { useParams } from 'react-router'
import { byRisk } from '@/chat/ClaimInspector'
import { splitModel } from '@/chat/models'
import { modeLabel, policyLabel } from '@/chat/modes'
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
  // Only the model ID is set in mono; the other facts are words.
  const facts: [string, string, boolean][] = [
    ['Model', r.model ? `${provider ? `${provider} / ` : ''}${name}` : 'Pasted text', r.model !== ''],
    ['Mode', modeLabel(r.mode), false],
    ['When a claim is outdated', policyLabel(r.policy), false],
    ['Time', formatDateTime(r.created_at), false],
  ]

  return (
    // One centred measure for the whole report, so its rules and the claim list share both edges.
    <section className="mx-auto max-w-[880px] space-y-6">
      <header className="flex flex-wrap items-center gap-4">
        <ScoreRing score={r.status === 'complete' ? r.score : null} />
        <div>
          <h1 className="text-2xl font-semibold">Freshness report</h1>
          <p className="text-text-muted">{progressSentence(r, checkedClaims)}</p>
          {/* The sentence above is plain words; operators also get the pipeline's own reason. */}
          {r.status === 'error' && r.reason && <p className="mt-1 font-mono text-xs text-text-subtle [overflow-wrap:anywhere]">{r.reason}</p>}
        </div>
      </header>

      <dl className="flex flex-wrap gap-x-8 gap-y-2 text-sm">
        {facts.map(([k, v, mono]) => (
          <div key={k}>
            <dt className="text-xs text-text-subtle">{k}</dt>
            <dd className={mono ? 'font-mono text-xs' : ''}>{v}</dd>
          </div>
        ))}
        <div>
          <dt className="text-xs text-text-subtle">Cost</dt>
          <dd className="tabular-nums">
            <span>Checked in {(r.latency_ms / 1000).toFixed(1)} s</span>
            {' · '}
            <span>{`${r.credits_used} Tavily credit${r.credits_used === 1 ? '' : 's'} used`}</span>
          </dd>
        </div>
      </dl>

      {r.blocked && (
        <p className="rounded-md border border-status-outdated p-3 text-sm">This answer was blocked before it reached the user.</p>
      )}

      <div className="grid gap-6 border-y border-border py-6 lg:grid-cols-2 lg:gap-0 lg:divide-x lg:divide-border">
        <div className="space-y-2 lg:pr-8">
          <h2 className="text-sm font-semibold">Original answer</h2>
          <ClaimText text={r.answer_text} claims={r.claims} reportStatus={r.status} />
        </div>
        <div className="space-y-2 lg:pl-8">
          <h2 className="text-sm font-semibold">Corrected answer</h2>
          {hasChanges(changes) ? (
            <HealDiff report={r} segments={changes} showCorrected />
          ) : (
            <p className="text-text-muted">No claims needed correction.</p>
          )}
        </div>
      </div>

      {r.claims.length > 0 && (
        <section aria-labelledby="claims-title" className="space-y-2">
          <h2 id="claims-title" className="text-lg font-semibold">
            Claims, riskiest first
          </h2>
          <ul className="divide-y divide-border">
            {byRisk(r.claims).map((c) => (
              <li key={c.id} className="py-6">
                <ExplainCard claim={c} report={r} />
              </li>
            ))}
          </ul>
        </section>
      )}
    </section>
  )
}
