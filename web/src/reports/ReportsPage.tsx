import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router'
import { listReports, type ApiError } from '@/api/client'
import type { Report, ReportStatus } from '@/api/types'
import { formatDateTime } from '@/claims/format'
import { ScoreRing } from '@/claims/ScoreRing'

const statusLabel: Record<ReportStatus, string> = { pending: 'Checking', complete: 'Complete', error: 'Failed' }

export function ReportsPage() {
  const reports = useQuery<Report[], ApiError>({ queryKey: ['reports'], queryFn: () => listReports({ limit: 50 }) })

  return (
    <section className="space-y-4">
      <h1 className="text-2xl font-semibold">Reports</h1>
      {reports.isPending && <p className="text-text-muted">Loading the reports.</p>}
      {reports.isError && <p className="text-status-outdated">{reports.error.message}</p>}
      {reports.data?.length === 0 && (
        <p className="text-text-muted">No answers have been checked yet. Send a message in the chat to see results here.</p>
      )}
      {reports.data && reports.data.length > 0 && (
        <div className="overflow-x-auto rounded-xl border border-border">
          <table className="w-full text-left text-sm">
            <thead className="bg-surface-1 text-text-muted">
              <tr>
                {['Time', 'Model', 'Mode', 'Status', 'Score', 'Outdated', 'Checked'].map((h) => (
                  <th key={h} scope="col" className="px-3 py-2 font-medium">
                    {h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {reports.data.map((r) => (
                <tr key={r.id} className="border-t border-border hover:bg-surface-2">
                  <td className="px-3 py-2">
                    <Link to={`/reports/${r.id}`} className="text-primary underline underline-offset-4">
                      {formatDateTime(r.created_at) || r.id}
                    </Link>
                  </td>
                  <td className="px-3 py-2 font-mono text-xs">{r.model || 'Pasted text'}</td>
                  <td className="px-3 py-2">{r.mode}</td>
                  <td className="px-3 py-2">{statusLabel[r.status]}</td>
                  <td className="px-3 py-2">
                    <ScoreRing score={r.score} size="sm" />
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
