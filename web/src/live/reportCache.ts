import { normalizeClaim, normalizeReport } from '@/api/client'
import type { LiveEvent, Report } from '@/api/types'

// Reports live in the TanStack Query cache under this key, written by both the
// event stream and API fetches.
export const reportKey = (id: string) => ['report', id] as const

function finished(r: Report): boolean {
  return r.status === 'complete' || r.status === 'error'
}

export function applyEvent(prev: Report | undefined, e: LiveEvent): Report {
  const base = prev ?? normalizeReport({ id: e.report_id, created_at: e.at })
  switch (e.type) {
    case 'report.started':
      if (prev) return prev
      return { ...base, status: 'pending', ...e.data }
    case 'report.claims':
      if (finished(base)) return base
      return { ...base, claims: e.data.claims.map(normalizeClaim) }
    case 'claim.verdict': {
      const claim = normalizeClaim(e.data.claim)
      const exists = base.claims.some((c) => c.id === claim.id)
      return { ...base, claims: exists ? base.claims.map((c) => (c.id === claim.id ? claim : c)) : [...base.claims, claim] }
    }
    case 'report.complete':
      return { ...base, ...e.data }
    case 'report.error':
      return { ...base, status: 'error', reason: e.data.reason }
  }
}

// mergeReport combines a fetched report with what events already delivered.
// The pipeline stores a report only when it starts and when it ends, so a
// fetched pending report has no claims yet, and a slow poll can return
// pending after the complete event has arrived.
export function mergeReport(cached: Report | undefined, fetched: Report): Report {
  if (fetched.status !== 'pending' || !cached) return fetched
  if (finished(cached)) return cached
  if (fetched.claims.length === 0 && cached.claims.length > 0) return { ...fetched, claims: cached.claims }
  return fetched
}
