import { useState } from 'react'
import { Link } from 'react-router'
import { decayModel } from './decay'
import { DecayCurve } from './DecayCurve'
import { formatDate, percent } from './format'
import { safeHref } from './links'
import { plainSnippet } from './snippet'
import { STATUS_META, uiStatus } from './status'
import type { Category, Claim, Report, Stance } from '@/api/types'

const categoryLabel: Record<Category, string> = {
  pricing: 'Pricing',
  version: 'Software version',
  leadership: 'Leadership',
  law: 'Law or regulation',
  statistic: 'Statistic',
  event: 'Event',
  feature: 'Product feature',
  static: 'Unchanging fact',
}

const stanceLabel: Record<Stance, string> = {
  supports: 'Supports the claim',
  contradicts: 'Contradicts the claim',
  neutral: 'Does not decide the claim',
}

// Excerpt shows a search-result excerpt as plain text, clamped to three lines
// when it is long. An excerpt that is only Markdown syntax is left out.
function Excerpt({ text }: { text: string }) {
  const [expanded, setExpanded] = useState(false)
  const plain = plainSnippet(text)
  if (!plain) return null
  const long = plain.length > 240
  return (
    <>
      <p data-excerpt className={`mt-1 text-text-muted [overflow-wrap:anywhere] ${long && !expanded ? 'line-clamp-3' : ''}`}>
        {plain}
      </p>
      {long && (
        <button
          type="button"
          aria-expanded={expanded}
          onClick={() => setExpanded(!expanded)}
          className="text-xs text-link underline underline-offset-4 hover:text-link-strong"
        >
          {expanded ? 'Show less of the excerpt' : 'Show the full excerpt'}
        </button>
      )}
    </>
  )
}

function Eyebrow({ children }: { children: string }) {
  return <h3 className="eyebrow mt-4 mb-2">{children}</h3>
}

// ExplainCard is the content of a claim's explain card (DESIGN.md section 7.1).
// Sources and the judge's reasoning are kept in separate, labelled sections,
// so evidence is never mistaken for inference.
export function ExplainCard({ claim, report }: { claim: Claim; report: Report }) {
  const status = uiStatus(claim, report.status)
  const meta = STATUS_META[status]
  const Icon = meta.icon
  const model = decayModel(claim, report)
  const outdated = claim.verdict === 'STALE' || claim.verdict === 'WRONG'
  const description = claim.verdict === 'UNVERIFIABLE' && claim.rationale && !claim.checked ? claim.rationale : meta.description

  return (
    <div className="space-y-2 text-sm">
      <p className="flex items-center gap-2 font-semibold">
        <Icon aria-hidden="true" className="size-4" style={{ color: `var(--status-${status === 'pending' ? 'unchecked' : status})` }} />
        <span>{meta.label}</span>
      </p>
      <p className="text-text">“{claim.quote}”</p>
      <p className="text-text-muted">{description}</p>

      {outdated && claim.current_value && (
        <p>
          <span className="font-semibold">Current value: {claim.current_value}</span>
          {claim.as_of && formatDate(claim.as_of) && <span className="text-text-muted"> as of {formatDate(claim.as_of)}</span>}
        </p>
      )}

      {claim.verdict === 'FRESH' && claim.score_parts && (
        <ul className="space-y-0.5 font-mono text-xs tabular-nums text-text-muted">
          <li>Evidence recency {percent(claim.score_parts.recency)}</li>
          <li>Source authority {percent(claim.score_parts.authority)}</li>
          <li>Agreement between sources {percent(claim.score_parts.agreement)}</li>
        </ul>
      )}

      <Eyebrow>Why this claim was checked</Eyebrow>
      <ul className="space-y-0.5 text-text-muted">
        <li>Category: {categoryLabel[claim.category]}</li>
        {claim.half_life_days > 0 && (
          <li>Half-life: {claim.half_life_days} days. Half of the facts in this category change within this time.</li>
        )}
        <li>The model's knowledge was about {report.model_age_days} days old when it answered.</li>
        <li>Estimated chance of change before checking: {percent(claim.p_changed)}.</li>
      </ul>

      {claim.checked && (
        <>
          <Eyebrow>Evidence from live sources</Eyebrow>
          {claim.evidence.length === 0 ? (
            <p className="text-text-muted">No sources were returned for this claim.</p>
          ) : (
            <ul className="space-y-3">
              {claim.evidence.map((e, i) => {
                const href = safeHref(e.url)
                return (
                  <li key={e.ref || i} className="rounded-md border border-border bg-surface-2 p-2">
                    <p className="font-medium [overflow-wrap:anywhere]">
                      {href ? (
                        <a href={href} target="_blank" rel="noopener noreferrer" className="text-link underline underline-offset-4 hover:text-link-strong">
                          {e.title || e.domain}
                        </a>
                      ) : (
                        e.title || e.domain
                      )}
                    </p>
                    <p className="font-mono text-xs text-text-subtle">
                      {e.domain}
                      {e.published_date && formatDate(e.published_date) && ` · Published ${formatDate(e.published_date)}`}
                      {e.authority === 1 && ' · Authoritative source'}
                    </p>
                    <Excerpt text={e.snippet} />
                    <p className="mt-1 text-xs text-text">{stanceLabel[e.stance]}</p>
                  </li>
                )
              })}
            </ul>
          )}
          {claim.rationale && (
            <>
              <Eyebrow>Inference by the judge model</Eyebrow>
              <p className="text-text">{claim.rationale}</p>
              <p className="font-mono text-xs tabular-nums text-text-muted">Judge confidence: {percent(claim.confidence)}</p>
              <p className="text-xs text-text-subtle">
                This is the judge model's reasoning about the sources above. It is an inference, not a source.
              </p>
            </>
          )}
        </>
      )}

      {model ? (
        <DecayCurve model={model} today={new Date()} />
      ) : outdated ? (
        <p className="text-text-muted">This claim is already outdated, so no decay curve is shown.</p>
      ) : (
        <p className="text-text-muted">Facts of this kind rarely change, so no decay curve is shown.</p>
      )}

      <Link to={`/reports/${report.id}`} className="inline-block pt-2 text-link underline underline-offset-4 hover:text-link-strong">
        Open full report
      </Link>
    </div>
  )
}
