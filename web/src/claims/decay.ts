import { formatDate } from './format'
import type { Claim, Report } from '@/api/types'

export const DAY_MS = 24 * 60 * 60 * 1000

// A decay curve: the probability that a claim has changed, as a function of
// time since origin. offsetDays is the age the claim already had at origin.
export interface DecayModel {
  origin: Date
  offsetDays: number
  factor: 1 | 2
  halfLifeDays: number
}

// decayModel follows triage.PChanged in the plugin. A verified current value
// starts fresh at the report time. An unverified claim is as old as the
// model's knowledge, doubled when the claim has a temporal marker. Outdated
// claims and facts without a half-life have no curve.
export function decayModel(claim: Claim, report: Report): DecayModel | null {
  const origin = new Date(report.created_at)
  if (claim.half_life_days <= 0 || Number.isNaN(origin.getTime())) return null
  switch (claim.verdict) {
    case 'STALE':
    case 'WRONG':
      return null
    case 'FRESH':
      return { origin, offsetDays: 0, factor: 1, halfLifeDays: claim.half_life_days }
    default:
      return {
        origin,
        offsetDays: report.model_age_days,
        factor: claim.temporal_marker ? 2 : 1,
        halfLifeDays: claim.half_life_days,
      }
  }
}

export function probabilityAt(model: DecayModel, at: Date): number {
  const age = Math.max(0, model.offsetDays + (at.getTime() - model.origin.getTime()) / DAY_MS)
  return 1 - 0.5 ** ((model.factor * age) / model.halfLifeDays)
}

export function scrubSentence(at: Date, p: number): string {
  return `On ${formatDate(at)}, this claim has a ${Math.round(p * 100)}% probability of being outdated.`
}
