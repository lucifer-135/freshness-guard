import { CircleX, Info, LoaderCircle } from 'lucide-react'
import { motion } from 'motion/react'
import { useEffect, useState } from 'react'
import { Link } from 'react-router'
import { splitModel } from './models'
import type { AssistantMessage, Message } from './useChat'
import { ClaimPreview } from '@/claims/ClaimPreview'
import { ClaimText } from '@/claims/ClaimText'
import { progressSentence } from '@/claims/progress'
import { ScoreRing } from '@/claims/ScoreRing'
import { STATUS_META } from '@/claims/status'
import { summaryChips } from '@/claims/summary'
import { useStaggeredClaims } from '@/claims/useStaggeredClaims'
import { Segmented } from '@/components/Segmented'
import { correctedView, hasChanges } from '@/heal/corrections'
import { HealDiff } from '@/heal/HealDiff'
import { useReport } from '@/live/useReport'
import { useReducedMotion } from '@/motion/useReducedMotion'

const versionOptions = [
  { value: 'original', label: 'Original' },
  { value: 'corrected', label: 'Corrected' },
]

function statusColour(status: string): string {
  return `var(--status-${status === 'pending' ? 'unchecked' : status})`
}

interface MessageViewProps {
  message: Message
  syncTimeoutMs: number
  openClaimId?: string | null
  onOpenChange?: (id: string | null) => void
  // Opens a claim's full details in the claim inspector.
  onShowDetails?: (id: string) => void
  onStatus?: (sentence: string) => void
}

export function MessageView({ message, ...rest }: MessageViewProps) {
  if (message.role === 'user') {
    return <p className="ml-auto max-w-[85%] rounded-lg bg-surface-2 px-4 py-2 text-base whitespace-pre-wrap">{message.text}</p>
  }
  return <AnswerView message={message} {...rest} />
}

function Note({ children }: { children: string }) {
  return (
    <p className="flex items-start gap-2 text-sm text-text-muted">
      <Info aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
      {children}
    </p>
  )
}

function AnswerView({
  message,
  syncTimeoutMs,
  openClaimId,
  onOpenChange,
  onShowDetails,
  onStatus,
}: MessageViewProps & { message: AssistantMessage }) {
  const checked = message.mode !== 'off' && message.phase === 'done' && message.reportId !== null
  const report = useReport(message.reportId, { enabled: checked })
  const staggered = useStaggeredClaims(report.data?.claims ?? [], report.data?.status ?? 'pending')
  const reduce = useReducedMotion()
  const changes = report.data ? correctedView(report.data) : []
  const [showCorrected, setShowCorrected] = useState<boolean | null>(null)
  // The answer fades between versions only after the reader switches; a message that is first shown does not animate.
  const [switched, setSwitched] = useState(false)
  const corrected = (showCorrected ?? (message.headers?.healed ?? 0) > 0) && hasChanges(changes)

  const revealed = staggered.claims.filter((c) => c.checked && c.verdict !== 'NOT_CHECKED').length
  const sentence = report.data
    ? progressSentence({ ...report.data, claims: staggered.claims, status: staggered.reportStatus }, revealed)
    : null
  useEffect(() => {
    if (sentence) onStatus?.(sentence)
  }, [sentence, onStatus])

  const { provider, name } = splitModel(message.model)
  const notFound = report.error?.status === 404
  // A failed check has no verdicts to count, so it shows no chips.
  const chips = report.data && staggered.reportStatus !== 'error' ? summaryChips(staggered.claims, staggered.reportStatus) : []

  return (
    // The answer is the artifact, so it sits on the page itself; turns are separated by space, not boxes.
    <article aria-busy={staggered.reportStatus === 'pending' && report.data !== undefined} className="space-y-3 py-2">
      <header className="flex flex-wrap items-center gap-3">
        {report.data && <ScoreRing score={staggered.reportStatus === 'complete' ? report.data.score : null} />}
        <div className="min-w-48 flex-1">
          {report.data && staggered.reportStatus === 'complete' && <p className="font-medium">Freshness {report.data.score} of 100</p>}
          <p className="font-mono text-xs text-text-subtle">
            {provider && `${provider} / `}
            {name}
            {message.elapsedMs !== null && ` · Answered in ${(message.elapsedMs / 1000).toFixed(1)} s`}
          </p>
          {sentence && (
            // The page's live region announces the sentence, so the spinner beside it is hidden from screen readers.
            <p className="flex items-center gap-2 text-sm text-text-muted">
              {staggered.reportStatus === 'pending' && (
                <LoaderCircle data-spinner aria-hidden="true" className="size-3.5 shrink-0 animate-spin motion-reduce:animate-none" />
              )}
              {sentence}
            </p>
          )}
          {staggered.reportStatus === 'error' && (
            <p className="mt-1 flex items-center gap-1.5 text-sm text-text">
              <CircleX aria-hidden="true" className="size-4 text-status-outdated" />
              The claims could not be checked.
            </p>
          )}
        </div>
        {hasChanges(changes) && (
          <Segmented
            label="Answer version"
            value={corrected ? 'corrected' : 'original'}
            options={versionOptions}
            onChange={(version) => {
              setSwitched(true)
              setShowCorrected(version === 'corrected')
            }}
          />
        )}
      </header>
      {chips.length > 0 && (
        <ul aria-label="Verdict summary" className="flex flex-wrap gap-x-4 gap-y-1">
          {chips.map(({ status, label }) => {
            const Icon = STATUS_META[status].icon
            return (
              <li key={status} className="inline-flex items-center gap-1.5 text-sm text-text-muted">
                <Icon aria-hidden="true" className="size-3.5" style={{ color: statusColour(status) }} />
                {label}
              </li>
            )
          })}
        </ul>
      )}

      {message.phase === 'streaming' &&
        (message.text ? (
          <p className="text-base leading-relaxed whitespace-pre-wrap">{message.text}</p>
        ) : (
          <p className="text-sm text-text-muted">
            {message.mode === 'sync' ? 'Checking the answer before it is returned.' : 'Waiting for the answer.'}
          </p>
        ))}

      {message.phase === 'blocked' && (
        <div className="space-y-2">
          <Note>
            {message.error?.includes('could not be verified')
              ? message.error
              : 'The answer was blocked because it contains outdated or incorrect claims.'}
          </Note>
          {message.reportId && (
            <Link to={`/reports/${message.reportId}`} className="text-link underline underline-offset-4 hover:text-link-strong">
              Open the full report
            </Link>
          )}
        </div>
      )}

      {message.phase === 'failed' && <Note>{`The answer could not be loaded. ${message.error ?? ''}`.trim()}</Note>}

      {message.phase === 'done' && (
        <>
          <motion.div
            key={corrected ? 'corrected' : 'original'}
            initial={switched ? (reduce ? { opacity: 0 } : { opacity: 0, filter: 'blur(2px)' }) : false}
            animate={reduce ? { opacity: 1 } : { opacity: 1, filter: 'blur(0px)' }}
            transition={{ duration: reduce ? 0.08 : 0.2, ease: [0.23, 1, 0.32, 1] }}
          >
            {report.data?.answer_text ? (
              corrected ? (
                <HealDiff report={report.data} segments={changes} showCorrected />
              ) : (
                <ClaimText
                  text={report.data.answer_text}
                  claims={staggered.claims}
                  reportStatus={staggered.reportStatus}
                  report={report.data}
                  openClaimId={openClaimId}
                  onOpenChange={onOpenChange}
                  renderCard={(claim, claimReport, close) => (
                    <ClaimPreview
                      claim={claim}
                      report={claimReport}
                      onShowDetails={() => {
                        close()
                        onShowDetails?.(claim.id)
                      }}
                    />
                  )}
                />
              )
            ) : (
              <p className="text-base leading-relaxed whitespace-pre-wrap">{message.text}</p>
            )}
          </motion.div>
          {message.mode === 'off' && <Note>Freshness checking was off for this answer.</Note>}
          {message.mode === 'sync' && message.headers?.status === 'pending' && (
            <Note>{`Verification is taking longer than ${Math.round(syncTimeoutMs / 1000)} seconds. The answer is shown now, and the results will appear when they are ready.`}</Note>
          )}
          {message.headers?.status === 'skipped' && (
            <Note>This answer was not checked, because the verification queue was full or the request was not selected for sampling.</Note>
          )}
          {notFound && (
            <Note>No freshness report was found for this answer. It may have been skipped because the verification queue was full.</Note>
          )}
        </>
      )}
    </article>
  )
}
