import { useId } from 'react'
import { segmentAnswer } from './segments'
import { STATUS_META, uiStatus } from './status'
import type { Claim, ReportStatus } from '@/api/types'

// ClaimText renders an answer with each claim underlined by its status. Claims
// are focusable buttons whose description gives the status in words, so the
// meaning never depends on colour or underline style alone.
export function ClaimText({ text, claims, reportStatus }: { text: string; claims: Claim[]; reportStatus: ReportStatus }) {
  const idPrefix = useId()
  const { segments } = segmentAnswer(text, claims)
  const placed = segments.flatMap((s) => (s.kind === 'claim' ? [s.claim] : []))

  return (
    <div>
      <p data-testid="answer-text" className="text-base leading-relaxed whitespace-pre-wrap">
        {segments.map((s, i) => {
          if (s.kind === 'text') return <span key={i}>{s.text}</span>
          const status = uiStatus(s.claim, reportStatus)
          return (
            <span
              key={i}
              role="button"
              tabIndex={0}
              data-claim-id={s.claim.id}
              data-claim-status={status}
              className={`${STATUS_META[status].className} cursor-pointer rounded-sm`}
              aria-describedby={`${idPrefix}-${s.claim.id}`}
            >
              {s.text}
            </span>
          )
        })}
      </p>
      <div hidden>
        {placed.map((claim) => {
          const meta = STATUS_META[uiStatus(claim, reportStatus)]
          return (
            <span key={claim.id} id={`${idPrefix}-${claim.id}`}>
              {meta.label}. {meta.description}
            </span>
          )
        })}
      </div>
    </div>
  )
}
