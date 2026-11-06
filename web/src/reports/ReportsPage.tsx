import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router'
import { listReports, type ApiError } from '@/api/client'
import type { Report, ReportStatus } from '@/api/types'
import { formatDateTime } from '@/claims/format'
import { ScoreFigure } from '@/claims/ScoreRing'
import { modeLabel } from '@/chat/modes'

const statusLabel: Record<ReportStatus, string> = { pending: 'Checking', complete: 'Complete', error: 'Failed' }

// Mode and status are details; on a phone the time, model and figures come first.
const narrowHidden = new Set(['Mode', 'Status'])

export function ReportsPage() {
  const reports = useQuery<Report[], ApiError>({ queryKey: ['reports'], queryFn: () => listReports({ limit: 50 }) })

  return (
    <section className="space-y-4">
      <div>
        <h1 className="text-2xl font-semibold">Reports</h1>
        <p className="mt-1 text-text-muted">Every answer Freshness Guard has checked, newest first.</p>
      </div>
      {reports.isPending && <p className="text-text-muted">Loading the reports.</p>}
      {reports.isError && <p className="text-status-outdated">{reports.error.message}</p>}
      {reports.data?.length === 0 && (
        <p className="text-text-muted">No answers have been checked yet. Send a message in the chat to see results here.</p>
      )}
      {reports.data && reports.data.length > 0 && (
        <div className="overflow-x-auto border-y border-border">
          <table className="w-full text-left text-sm">
            <thead className="text-text-muted">
              <tr>
                {['Time', 'Model', 'Mode', 'Status', 'Score', 'Outdated', 'Checked'].map((h) => (
                  <th key={h} scope="col" className={`px-3 py-2 font-medium ${narrowHidden.has(h) ? 'hidden sm:table-cell' : ''}`}>
                    {h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {reports.data.map((r) => (
                // The whole row opens the report: the date link is stretched over the row.
                <tr key={r.id} className="relative border-t border-border hover:bg-surface-1">
                  <td className="px-3 py-2">
                    <Link to={`/reports/${r.id}`} className="font-medium text-text after:absolute after:inset-0 hover:text-link-strong">
                      {formatDateTime(r.created_at) || r.id}
                    </Link>
                  </td>
                  <td className="px-3 py-2 font-mono text-xs">{r.model || 'Pasted text'}</td>
                  <td className="hidden px-3 py-2 sm:table-cell">{modeLabel(r.mode)}</td>
                  <td className="hidden px-3 py-2 sm:table-cell">{statusLabel[r.status]}</td>
                  <td className="px-3 py-2">
                    <ScoreFigure score={r.score} />
                  </td>
                  <td className="px-3 py-2 font-mono tabular-nums">{r.stale_count}</td>
                  <td className="px-3 py-2 font-mono tabular-nums">
                    {r.checked_count} of {r.total_claims}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  )
}
