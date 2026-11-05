import { useEffect, useId, useState } from 'react'
import { Link } from 'react-router'
import { splitModel } from './models'
import type { AssistantMessage, Message } from './useChat'
import { ClaimText } from '@/claims/ClaimText'
import { progressSentence } from '@/claims/progress'
import { ScoreRing } from '@/claims/ScoreRing'
import { useStaggeredClaims } from '@/claims/useStaggeredClaims'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { correctedView, hasChanges } from '@/heal/corrections'
import { HealDiff } from '@/heal/HealDiff'
import { useReport } from '@/live/useReport'

interface MessageViewProps {
  message: Message
  syncTimeoutMs: number
  openClaimId?: string | null
  onOpenChange?: (id: string | null) => void
  onStatus?: (sentence: string) => void
}

export function MessageView({ message, ...rest }: MessageViewProps) {
  if (message.role === 'user') {
    return <p className="ml-auto max-w-[85%] rounded-lg bg-surface-2 px-4 py-2 text-base whitespace-pre-wrap">{message.text}</p>
  }
  return <AnswerView message={message} {...rest} />
}

function Note({ children }: { children: string }) {
  return <p className="rounded-md border border-border bg-surface-2 p-3 text-sm text-text-muted">{children}</p>
}

function AnswerView({ message, syncTimeoutMs, openClaimId, onOpenChange, onStatus }: MessageViewProps & { message: AssistantMessage }) {
  const checked = message.mode !== 'off' && message.phase === 'done' && message.reportId !== null
  const report = useReport(message.reportId, { enabled: checked })
  const staggered = useStaggeredClaims(report.data?.claims ?? [], report.data?.status ?? 'pending')
  const switchId = useId()
  const changes = report.data ? correctedView(report.data) : []
  const [showCorrected, setShowCorrected] = useState<boolean | null>(null)
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

  return (
    <article className="space-y-3 rounded-xl border border-border bg-surface-1 p-4 shadow-[var(--highlight-top)]">
      <header className="flex flex-wrap items-center gap-3">
        {report.data && <ScoreRing score={staggered.reportStatus === 'complete' ? report.data.score : null} />}
        <div className="min-w-0">
          <p className="font-mono text-xs text-text-subtle">
            {provider && `${provider} / `}
            {name}
            {message.elapsedMs !== null && ` · Answered in ${(message.elapsedMs / 1000).toFixed(1)} s`}
          </p>
          {sentence && <p className="text-sm text-text-muted">{sentence}</p>}
        </div>
      </header>

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
            <Link to={`/reports/${message.reportId}`} className="text-primary underline underline-offset-4">
              Open the full report
            </Link>
          )}
        </div>
      )}

      {message.phase === 'failed' && <Note>{`The answer could not be loaded. ${message.error ?? ''}`.trim()}</Note>}

      {message.phase === 'done' && (
        <>
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
              />
            )
          ) : (
            <p className="text-base leading-relaxed whitespace-pre-wrap">{message.text}</p>
          )}
          {hasChanges(changes) && (
            <div className="flex items-center gap-2">
              <Switch id={switchId} checked={corrected} onCheckedChange={setShowCorrected} />
              <Label htmlFor={switchId}>Show corrected answer</Label>
            </div>
          )}
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
