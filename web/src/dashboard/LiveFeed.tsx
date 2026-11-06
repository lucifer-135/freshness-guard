import { useQuery } from '@tanstack/react-query'
import { Pause, Play } from 'lucide-react'
import { AnimatePresence, motion } from 'motion/react'
import { useState } from 'react'
import { Link } from 'react-router'
import { listReports, type ApiError } from '@/api/client'
import type { Report, ReportStatus } from '@/api/types'
import { formatDateTime } from '@/claims/format'
import { ScoreFigure } from '@/claims/ScoreRing'
import { Button } from '@/components/ui/button'
import { feedKey } from '@/live/keys'
import { useReducedMotion } from '@/motion/useReducedMotion'

const statusLabel: Record<ReportStatus, string> = { pending: 'Checking', complete: 'Complete', error: 'Failed' }

// The feed shows the most recent 50 reports; that cap replaces virtualisation
// (DESIGN.md section 10 asks for it only above 50 items).
const FEED_LIMIT = 50
// The feed shows whole rows only; the full list is one link away.
const FEED_ROWS = 8

// LiveFeed lists the newest reports. While it is paused it keeps the list it
// had, so nothing moves under the reader, and it counts what is waiting.
export function LiveFeed() {
  const reports = useQuery<Report[], ApiError>({ queryKey: feedKey, queryFn: () => listReports({ limit: FEED_LIMIT }) })
  const reduced = useReducedMotion()
  const [frozen, setFrozen] = useState<Report[] | null>(null)

  const latest = reports.data ?? []
  const shown = frozen ?? latest
  const waiting = frozen ? latest.filter((r) => !frozen.some((f) => f.id === r.id)).length : 0

  return (
    <section aria-labelledby="live-feed-title" className="@container flex h-full flex-col gap-3 lg:absolute lg:inset-0">
      <div className="flex items-center justify-between gap-3">
        <h2 id="live-feed-title" className="text-lg font-semibold">
          Live verification feed
        </h2>
        {/* The label names the next action, so a paused feed is visible at a glance
            (a toggle with aria-pressed and a fixed label looked unchanged). */}
        <Button variant="secondary" onClick={() => setFrozen(frozen ? null : latest)}>
          {frozen ? <Play aria-hidden="true" /> : <Pause aria-hidden="true" />}
          {frozen ? 'Resume live feed' : 'Pause live feed'}
        </Button>
      </div>
      {frozen && (
        // The line exists from the moment of pausing, so a new report changes its
        // text without moving the list, and the live region announces it.
        <p role="status" className="min-h-5 text-sm text-text-muted">
          {waiting === 0 ? 'The feed is paused. New reports will wait here.' : waiting === 1 ? '1 new report is waiting.' : `${waiting} new reports are waiting.`}
        </p>
      )}
      {reports.isPending && <p className="text-text-muted">Loading the reports.</p>}
      {reports.isError && <p className="text-status-outdated">The reports could not be loaded.</p>}
      {reports.data && shown.length === 0 && (
        <p className="text-text-muted">No answers have been checked yet. Send a message in the chat to see results here.</p>
      )}
      <ol aria-label="Recent reports" className="min-h-0 shrink-0 divide-y divide-border overflow-y-auto rounded-lg border border-border bg-surface-1 empty:hidden">
        <AnimatePresence initial={false}>
          {shown.slice(0, FEED_ROWS).map((r) => (
            <motion.li
              key={r.id}
              initial={reduced ? { opacity: 0 } : { opacity: 0, y: -8 }}
              animate={reduced ? { opacity: 1 } : { opacity: 1, y: 0 }}
              transition={{ duration: reduced ? 0.08 : 0.22 }}
            >
              <Link to={`/reports/${r.id}`} className="flex flex-wrap items-center gap-x-3 gap-y-0.5 px-3 py-2 text-sm hover:bg-surface-3 focus-visible:outline-offset-[-2px] @lg:h-10 @lg:flex-nowrap @lg:py-0">
                <span className="w-7 shrink-0 text-right">
                  <ScoreFigure score={r.score} />
                </span>
                <span className="min-w-0 flex-1 truncate font-mono text-xs" title={r.model || undefined}>
                  {r.model || 'Pasted text'}
                </span>
                <span className="shrink-0 text-text-muted">
                  {r.status === 'complete' ? `${r.stale_count} outdated` : statusLabel[r.status]}
                </span>
                <time dateTime={r.created_at} className="w-full shrink-0 pl-10 text-xs text-text-muted tabular-nums @lg:w-auto @lg:pl-0">
                  {formatDateTime(r.created_at)}
                </time>
              </Link>
            </motion.li>
          ))}
        </AnimatePresence>
      </ol>
      {shown.length > FEED_ROWS && (
        <Link to="/reports" className="self-start text-sm text-link underline underline-offset-4 hover:text-link-strong">
          Show all reports
        </Link>
      )}
    </section>
  )
}
