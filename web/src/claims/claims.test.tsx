import { act, render, renderHook, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ClaimText } from './ClaimText'
import { progressSentence } from './progress'
import { ScoreRing } from './ScoreRing'
import { segmentAnswer } from './segments'
import { uiStatus } from './status'
import { useStaggeredClaims } from './useStaggeredClaims'
import { normalizeReport } from '@/api/client'
import type { Claim, Report } from '@/api/types'
import { answer, completeReport, freshClaim, staleClaim } from '@/test/fixtures'

function claimAt(text: string, quote: string, extra: Partial<Claim> = {}): Claim {
  const start = text.indexOf(quote)
  return staleClaim({ id: quote, quote, span_start: start, span_end: start + quote.length, ...extra })
}

function joined(text: string, claims: Claim[]): string {
  return segmentAnswer(text, claims)
    .segments.map((s) => s.text)
    .join('')
}

describe('uiStatus', () => {
  it('maps verdicts to interface statuses', () => {
    expect(uiStatus(freshClaim(), 'complete')).toBe('current')
    expect(uiStatus(staleClaim(), 'complete')).toBe('outdated')
    expect(uiStatus(staleClaim({ verdict: 'WRONG' }), 'complete')).toBe('incorrect')
    expect(uiStatus(staleClaim({ verdict: 'UNVERIFIABLE' }), 'complete')).toBe('unverified')
  })

  it('tells a claim being checked from one that is not checked', () => {
    const waiting = staleClaim({ verdict: 'NOT_CHECKED', checked: true })
    expect(uiStatus(waiting, 'pending')).toBe('pending')
    expect(uiStatus(waiting, 'complete')).toBe('unchecked')
    expect(uiStatus(staleClaim({ verdict: 'NOT_CHECKED', checked: false }), 'pending')).toBe('unchecked')
  })
})

describe('segmentAnswer', () => {
  it('splits the answer around valid spans', () => {
    const { segments, unplaced } = segmentAnswer(answer, [staleClaim(), freshClaim()])
    expect(segments.map((s) => s.kind)).toEqual(['claim', 'text', 'claim', 'text'])
    expect(unplaced).toEqual([])
    expect(joined(answer, [staleClaim(), freshClaim()])).toBe(answer)
  })

  it('lists claims that cannot be placed', () => {
    const missing = staleClaim({ span_start: -1, span_end: -1 })
    const beyond = staleClaim({ id: 'x', span_start: 10, span_end: answer.length + 5 })
    const result = segmentAnswer(answer, [missing, beyond])
    expect(result.unplaced.map((c) => c.id)).toEqual(['c1', 'x'])
    expect(result.segments).toEqual([{ kind: 'text', text: answer }])
  })

  it('keeps the first of two overlapping claims', () => {
    const first = staleClaim({ id: 'a', span_start: 0, span_end: 20 })
    const second = staleClaim({ id: 'b', span_start: 10, span_end: 30 })
    const result = segmentAnswer(answer, [second, first])
    expect(result.segments.filter((s) => s.kind === 'claim').map((s) => s.kind === 'claim' && s.claim.id)).toEqual(['a'])
    expect(result.unplaced.map((c) => c.id)).toEqual(['b'])
  })

  it('never splits an emoji', () => {
    const text = 'Price 😀 now'
    const claim = staleClaim({ span_start: 0, span_end: 7 })
    const claimSegment = segmentAnswer(text, [claim]).segments[0]
    expect(claimSegment).toMatchObject({ kind: 'claim', text: 'Price 😀' })
    expect(joined(text, [claim])).toBe(text)
  })
})

describe('progressSentence', () => {
  const report = (status: Report['status'], claims: Claim[], extra: Partial<Report> = {}) =>
    normalizeReport({ id: 'r', status, claims, ...extra })
  const pendingClaim = (id: string) => staleClaim({ id, verdict: 'NOT_CHECKED', checked: true })
  const text = 'abcdefghij'

  it('describes a pending check', () => {
    expect(progressSentence(report('pending', []), 0)).toBe('Finding the claims in this answer.')
    expect(progressSentence(report('pending', [pendingClaim('a'), pendingClaim('b')]), 1)).toBe(
      'Checking claims against live sources. 1 of 2 claims checked.',
    )
  })

  it('summarizes a finished check', () => {
    const many = [
      ...Array.from({ length: 6 }, (_, i) => freshClaim({ id: `f${i}` })),
      staleClaim({ id: 's1' }),
      staleClaim({ id: 's2' }),
      staleClaim({ id: 'u1', verdict: 'UNVERIFIABLE' }),
    ]
    expect(progressSentence(report('complete', many), 9)).toBe(
      '9 claims checked. 2 claims are outdated and 1 claim could not be verified.',
    )
    expect(progressSentence(report('complete', [staleClaim(), freshClaim()]), 2)).toBe('2 claims checked. 1 claim is outdated.')
    expect(
      progressSentence(
        report('complete', [staleClaim(), staleClaim({ id: 'w', verdict: 'WRONG' }), staleClaim({ id: 'u', verdict: 'UNVERIFIABLE' })]),
        3,
      ),
    ).toBe('3 claims checked. 1 claim is outdated, 1 claim is incorrect and 1 claim could not be verified.')
    expect(
      progressSentence(
        report('complete', [freshClaim(), freshClaim({ id: 'f2' }), claimAt(text, 'abc', { verdict: 'NOT_CHECKED', checked: false })]),
        2,
      ),
    ).toBe('2 claims checked. All of them are current. 1 claim was not checked because it is unlikely to change.')
  })

  it('covers answers without checked claims and failed checks', () => {
    expect(progressSentence(report('complete', []), 0)).toBe('No time-sensitive claims were found in this answer.')
    expect(progressSentence(report('complete', [claimAt(text, 'abc', { verdict: 'NOT_CHECKED', checked: false })]), 0)).toBe(
      'No claims were checked because none of them is likely to change over time.',
    )
    expect(progressSentence(report('error', [], { reason: 'Claim extraction failed: timeout' }), 0)).toBe(
      'The freshness check could not finish. Finding the claims failed because a model did not answer in time.',
    )
  })

  // The pipeline's reasons are written for logs; the reader gets the stage and the cause in plain words.
  it('explains a failed check in plain words instead of the raw error', () => {
    const failed = (reason: string) => progressSentence(report('error', [], { reason }), 0)
    expect(
      failed('Verification failed: judge: call groq/openai/gpt-oss-120b: Post "http://127.0.0.1:8080/v1/chat/completions": context deadline exceeded'),
    ).toBe('The freshness check could not finish. Checking the claims failed because a model did not answer in time.')
    expect(failed('Claim extraction failed: model groq/openai/gpt-oss-20b returned HTTP 400: {"error":{"code":"json_validate_failed"}}')).toBe(
      'The freshness check could not finish. Finding the claims failed because a model returned a reply that could not be read.',
    )
    expect(failed('Verification failed: judge: model groq/a returned HTTP 429: rate limit')).toBe(
      'The freshness check could not finish. Checking the claims failed because a model provider is limiting requests. Try again in a minute.',
    )
    expect(failed('Claim extraction failed: call groq/a: Post "http://127.0.0.1:8080": dial tcp 127.0.0.1:8080: connect: connection refused')).toBe(
      'The freshness check could not finish. Finding the claims failed because Bifrost could not be reached.',
    )
    // A reason that is already a plain sentence is kept as it is.
    expect(failed('The judge model did not respond.')).toBe('The freshness check could not finish. The judge model did not respond.')
  })
})

describe('useStaggeredClaims', () => {
  afterEach(() => vi.useRealTimers())

  it('reveals verdicts that arrive together one at a time', () => {
    vi.useFakeTimers()
    const pending = [staleClaim({ verdict: 'NOT_CHECKED' }), freshClaim({ verdict: 'NOT_CHECKED' })]
    const { result, rerender } = renderHook(({ claims, status }) => useStaggeredClaims(claims, status), {
      initialProps: { claims: pending, status: 'pending' as Report['status'] },
    })
    rerender({ claims: [staleClaim(), freshClaim()], status: 'complete' })
    const judged = () => result.current.claims.filter((c) => c.verdict !== 'NOT_CHECKED').length
    expect(judged()).toBe(0)
    expect(result.current.reportStatus).toBe('pending')
    act(() => vi.advanceTimersByTime(250))
    expect(judged()).toBe(1)
    act(() => vi.advanceTimersByTime(250))
    expect(judged()).toBe(2)
    expect(result.current.reportStatus).toBe('complete')
  })

  it('shows a report that was already finished at once', () => {
    const { result } = renderHook(() => useStaggeredClaims([staleClaim(), freshClaim()], 'complete'))
    expect(result.current.claims.filter((c) => c.verdict !== 'NOT_CHECKED')).toHaveLength(2)
  })

  it('shows every verdict at once with reduced motion', () => {
    const original = window.matchMedia
    window.matchMedia = (query: string) => ({ ...original(query), matches: query.includes('reduce') })
    const { result, rerender } = renderHook(({ claims, status }) => useStaggeredClaims(claims, status), {
      initialProps: { claims: [staleClaim({ verdict: 'NOT_CHECKED' })], status: 'pending' as Report['status'] },
    })
    rerender({ claims: [staleClaim()], status: 'complete' })
    expect(result.current.claims[0].verdict).toBe('STALE')
    window.matchMedia = original
  })
})

describe('ScoreRing', () => {
  it('labels the score for screen readers', () => {
    render(<ScoreRing score={72} />)
    expect(screen.getByRole('img', { name: 'Freshness score 72 out of 100' })).toHaveTextContent('72')
  })

  it('shows a dash without a score', () => {
    render(<ScoreRing score={null} />)
    expect(screen.getByRole('img', { name: 'No freshness score' })).toHaveTextContent('–')
  })
})

describe('ClaimText', () => {
  it('underlines each claim with its status and keeps the text intact', () => {
    const report = completeReport()
    render(<ClaimText text={answer} claims={report.claims} reportStatus="complete" />)
    expect(screen.getByTestId('answer-text').textContent).toBe(answer)
    const claim = screen.getByRole('button', { name: 'Example Cloud Pro costs $20 per month' })
    expect(claim).toHaveAttribute('tabindex', '0')
    expect(claim).toHaveAttribute('data-claim-status', 'outdated')
    expect(claim).toHaveClass('claim-outdated')
    expect(claim).toHaveAccessibleDescription(
      'Outdated. This statement was correct in the past. Current sources show a different value.',
    )
  })
})
