import { CircleCheck, CircleDashed, CircleHelp, CircleX, History, type LucideIcon } from 'lucide-react'
import type { Claim, ReportStatus } from '@/api/types'

// DESIGN.md section 3.3; "pending" is a claim that will be checked but has no verdict yet.
export type UiStatus = 'current' | 'unverified' | 'outdated' | 'incorrect' | 'unchecked' | 'pending'

export function uiStatus(claim: Claim, reportStatus: ReportStatus): UiStatus {
  switch (claim.verdict) {
    case 'FRESH':
      return 'current'
    case 'UNVERIFIABLE':
      return 'unverified'
    case 'STALE':
      return 'outdated'
    case 'WRONG':
      return 'incorrect'
    case 'NOT_CHECKED':
      return claim.checked && reportStatus === 'pending' ? 'pending' : 'unchecked'
  }
}

export const STATUS_META: Record<UiStatus, { label: string; icon: LucideIcon; className: string; description: string }> = {
  current: {
    label: 'Current',
    icon: CircleCheck,
    className: 'claim-current',
    description: 'Current sources confirm this statement.',
  },
  unverified: {
    label: 'Could not verify',
    icon: CircleHelp,
    className: 'claim-unverified',
    description: 'No recent sources were found that confirm or contradict this statement.',
  },
  outdated: {
    label: 'Outdated',
    icon: History,
    className: 'claim-outdated',
    description: 'This statement was correct in the past. Current sources show a different value.',
  },
  incorrect: {
    label: 'Incorrect',
    icon: CircleX,
    className: 'claim-incorrect',
    description: 'Current sources show that this value was never correct.',
  },
  unchecked: {
    label: 'Not checked (low risk)',
    icon: CircleDashed,
    className: 'claim-unchecked',
    description: 'This statement was not checked because it is unlikely to change over time.',
  },
  pending: {
    label: 'Not checked yet',
    icon: CircleDashed,
    className: 'claim-unchecked',
    description: 'This statement is being checked against live sources.',
  },
}
