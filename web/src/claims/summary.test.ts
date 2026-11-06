import { describe, expect, it } from 'vitest'
import { summaryChips } from './summary'
import { freshClaim, staleClaim } from '@/test/fixtures'

describe('summaryChips', () => {
  it('counts the verdicts of a finished report, riskiest first', () => {
    expect(summaryChips([freshClaim(), staleClaim()], 'complete')).toEqual([
      { status: 'outdated', count: 1, label: '1 outdated' },
      { status: 'current', count: 1, label: '1 current' },
    ])
  })

  it('has no chips for an answer without claims', () => {
    expect(summaryChips([], 'complete')).toEqual([])
  })

  it('groups claims of the same status', () => {
    const chips = summaryChips(
      [staleClaim({ id: 'a', verdict: 'UNVERIFIABLE' }), staleClaim({ id: 'b', verdict: 'UNVERIFIABLE' })],
      'complete',
    )
    expect(chips).toEqual([{ status: 'unverified', count: 2, label: '2 could not be verified' }])
  })

  it('counts claims that are still being checked while the report is pending', () => {
    const chips = summaryChips([staleClaim({ verdict: 'NOT_CHECKED', checked: true })], 'pending')
    expect(chips).toEqual([{ status: 'pending', count: 1, label: '1 being checked' }])
  })
})
