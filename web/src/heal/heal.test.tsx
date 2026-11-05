import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { citationFor, correctedView, correctionSegments, diffSegments, hasChanges } from './corrections'
import { HealDiff } from './HealDiff'
import { answer, completeReport, freshClaim, staleClaim } from '@/test/fixtures'

describe('correctionSegments', () => {
  it('replaces the outdated value inside its claim', () => {
    expect(correctionSegments(answer, [staleClaim(), freshClaim()])).toEqual([
      { kind: 'text', text: 'Example Cloud Pro costs ' },
      { kind: 'change', before: '$20 per month', after: '$25 per month', claim: staleClaim() },
      { kind: 'text', text: ', and its current version is 4.2.' },
    ])
  })

  it('appends an annotation when the value is not in the quote', () => {
    const claim = staleClaim({ value: 'twenty dollars', as_of: '2026-09-01' })
    const segments = correctionSegments(answer, [claim])
    expect(segments[1]).toEqual({
      kind: 'change',
      before: '',
      after: ' [outdated — now $25 per month as of 2026-09-01 (example.com)]',
      claim,
    })
    expect(segments[0]).toEqual({ kind: 'text', text: 'Example Cloud Pro costs $20 per month' })
  })

  it('says incorrect for a wrong value and leaves current claims alone', () => {
    const wrong = staleClaim({ verdict: 'WRONG', value: 'nothing like it' })
    expect(correctionSegments(answer, [wrong])[1]).toMatchObject({ after: expect.stringContaining('[incorrect — now') })
    expect(hasChanges(correctionSegments(answer, [freshClaim()]))).toBe(false)
  })
})

describe('diffSegments', () => {
  it('turns a corrected value into one change linked to its claim', () => {
    const healed = answer.replace('$20', '$25')
    const changes = diffSegments(answer, healed, [staleClaim(), freshClaim()]).filter((s) => s.kind === 'change')
    expect(changes).toHaveLength(1)
    expect(changes[0]).toMatchObject({ before: '$20', after: '$25' })
    expect(changes[0].kind === 'change' && changes[0].claim?.id).toBe('c1')
  })

  it('shows an appended annotation as an insertion', () => {
    const healed = answer.replace('$20 per month', '$20 per month [outdated — now $25 per month]')
    const changes = diffSegments(answer, healed, [staleClaim()]).filter((s) => s.kind === 'change')
    expect(changes).toHaveLength(1)
    expect(changes[0]).toMatchObject({ before: '' })
  })
})

describe('correctedView and citationFor', () => {
  it('uses the healed text when there is one', () => {
    const report = completeReport({ healed_text: answer.replace('$20', '$25') })
    expect(correctedView(report).find((s) => s.kind === 'change')).toMatchObject({ before: '$20', after: '$25' })
  })

  it('cites the contradicting source first', () => {
    expect(citationFor(staleClaim())).toEqual({ domain: 'example.com', href: 'https://example.com/pricing' })
    expect(citationFor(staleClaim({ evidence: [] }))).toBeNull()
  })
})

describe('HealDiff', () => {
  const report = completeReport()
  const segments = correctionSegments(answer, report.claims)

  it('shows the original answer when the corrected view is off', () => {
    const { container } = render(<HealDiff report={report} segments={segments} showCorrected={false} />)
    expect(container.textContent).toBe(answer)
  })

  it('strikes the old value and cites the new one', () => {
    const { container } = render(<HealDiff report={report} segments={segments} showCorrected />)
    expect(container.querySelector('del')).toHaveTextContent('$20 per month')
    expect(container.querySelector('ins')).toHaveTextContent('$25 per month')
    expect(screen.getByRole('link', { name: 'example.com' })).toHaveAttribute('href', 'https://example.com/pricing')
  })
})
