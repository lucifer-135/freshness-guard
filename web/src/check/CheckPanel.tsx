import { useQueryClient } from '@tanstack/react-query'
import { SearchCheck } from 'lucide-react'
import { useId, useState, type FormEvent } from 'react'
import { checkText } from '@/api/client'
import { ClaimInspector } from '@/chat/ClaimInspector'
import { ClaimText } from '@/claims/ClaimText'
import { progressSentence } from '@/claims/progress'
import { ScoreRing } from '@/claims/ScoreRing'
import { useStaggeredClaims } from '@/claims/useStaggeredClaims'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import type { Report } from '@/api/types'
import { mergeReport, reportKey } from '@/live/reportCache'
import { useReport } from '@/live/useReport'
import { Button } from '@/components/ui/button'

// The plugin's POST /freshness/check limit, counted in characters (runes).
const MAX_CHARACTERS = 20000

// CheckPanel checks text from any source with POST /freshness/check. A check
// that is still running is completed by the event stream or by polling.
export function CheckPanel() {
  const queryClient = useQueryClient()
  const [text, setText] = useState('')
  const [reportId, setReportId] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [openClaim, setOpenClaim] = useState<string | null>(null)
  const fieldId = useId()
  const limitId = useId()

  const report = useReport(reportId)
  const staggered = useStaggeredClaims(report.data?.claims ?? [], report.data?.status ?? 'pending')

  const length = Array.from(text).length
  const tooLong = length > MAX_CHARACTERS
  const canCheck = !busy && !tooLong && text.trim() !== ''

  async function submit(e: FormEvent) {
    e.preventDefault()
    if (!canCheck) return
    setBusy(true)
    setError(null)
    try {
      const checked = await checkText(text)
      // The request can wait up to 30 s; events may already have delivered claims for it.
      queryClient.setQueryData<Report>(reportKey(checked.id), (prev) => mergeReport(prev, checked))
      setOpenClaim(null)
      setReportId(checked.id)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'The text could not be checked.')
    } finally {
      setBusy(false)
    }
  }

  const data = report.data
  const revealed = staggered.claims.filter((c) => c.checked && c.verdict !== 'NOT_CHECKED').length
  const sentence = data ? progressSentence({ ...data, claims: staggered.claims, status: staggered.reportStatus }, revealed) : null

  return (
    <div className="flex gap-6">
      <div className="min-w-0 flex-1 space-y-4 xl:max-w-[760px]">
        <form onSubmit={submit} className="space-y-3 rounded-xl border border-border bg-surface-1 p-4">
          <Label htmlFor={fieldId}>Answer to check</Label>
          <Textarea
            id={fieldId}
            value={text}
            onChange={(e) => setText(e.target.value)}
            placeholder="Paste an answer from any model or source."
            rows={8}
            aria-invalid={tooLong}
            aria-describedby={limitId}
          />
          <p id={limitId} className={`text-xs tabular-nums ${tooLong ? 'text-status-outdated' : 'text-text-subtle'}`}>
            {tooLong
              ? 'The text must not be longer than 20,000 characters.'
              : `${length.toLocaleString('en-GB')} of ${MAX_CHARACTERS.toLocaleString('en-GB')} characters`}
          </p>
          {busy && !data && <p className="text-sm text-text-muted">Checking the text.</p>}
          {error && (
            <p role="alert" className="text-sm text-status-outdated">
              {error}
            </p>
          )}
          <Button type="submit" className="h-10" disabled={!canCheck}>
            <SearchCheck aria-hidden="true" />
            Check this answer
          </Button>
        </form>

        {data && (
          <article className="space-y-3 rounded-xl border border-border bg-surface-1 p-4">
            <header className="flex items-center gap-3">
              <ScoreRing score={staggered.reportStatus === 'complete' ? data.score : null} />
              <p aria-live="polite" className="text-sm text-text-muted">
                {sentence}
              </p>
            </header>
            {data.answer_text ? (
              <ClaimText
                text={data.answer_text}
                claims={staggered.claims}
                reportStatus={staggered.reportStatus}
                report={data}
                openClaimId={openClaim}
                onOpenChange={setOpenClaim}
              />
            ) : (
              <p className="text-base leading-relaxed whitespace-pre-wrap">{text}</p>
            )}
          </article>
        )}
      </div>
      {data && data.claims.length > 0 && (
        <aside aria-label="Claims in the checked text" className="hidden w-[360px] shrink-0 space-y-3 xl:block">
          <h2 className="eyebrow">Claims in the checked text</h2>
          <ClaimInspector report={data} selectedId={null} onSelect={setOpenClaim} />
        </aside>
      )}
    </div>
  )
}
