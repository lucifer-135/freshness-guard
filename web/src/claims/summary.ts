import { uiStatus, type UiStatus } from './status'
import type { Claim, ReportStatus } from '@/api/types'

export interface SummaryChip {
  status: UiStatus
  count: number
  label: string
}

// Riskiest first, so the first chip is the one that needs attention.
const order: UiStatus[] = ['incorrect', 'outdated', 'unverified', 'current', 'unchecked', 'pending']

const words: Record<UiStatus, string> = {
  incorrect: 'incorrect',
  outdated: 'outdated',
  unverified: 'could not be verified',
  current: 'current',
  unchecked: 'not checked',
  pending: 'being checked',
}

// summaryChips counts a message's claims by status for the message header
// (DESIGN.md section 7.1). Statuses with no claim are left out.
export function summaryChips(claims: Claim[], reportStatus: ReportStatus): SummaryChip[] {
  const counts = new Map<UiStatus, number>()
  for (const claim of claims) {
    const status = uiStatus(claim, reportStatus)
    counts.set(status, (counts.get(status) ?? 0) + 1)
  }
  return order.flatMap((status) => {
    const count = counts.get(status)
    return count ? [{ status, count, label: `${count} ${words[status]}` }] : []
  })
}
