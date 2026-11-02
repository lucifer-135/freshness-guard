import { describe, expect, it } from 'vitest'
import { applyEvent, mergeReport } from './reportCache'
import { normalizeReport } from '@/api/client'
import { completeReport, event, freshClaim, staleClaim } from '@/test/fixtures'

const started = event('report.started', 'r1', { model: 'm', provider: 'p', mode: 'sync', policy: 'heal' })

describe('applyEvent', () => {
  it('creates a pending report from report.started', () => {
    expect(applyEvent(undefined, started)).toMatchObject({
      id: 'r1',
      status: 'pending',
      model: 'm',
      provider: 'p',
      mode: 'sync',
      policy: 'heal',
      claims: [],
    })
  })

  it('sets the claims and replaces a claim on its verdict', () => {
    const pendingStale = staleClaim({ verdict: 'NOT_CHECKED' })
    const pendingFresh = freshClaim({ verdict: 'NOT_CHECKED' })
    let r = applyEvent(applyEvent(undefined, started), event('report.claims', 'r1', { claims: [pendingStale, pendingFresh] }))
    r = applyEvent(r, event('claim.verdict', 'r1', { claim: freshClaim() }))
    expect(r.claims.map((c) => c.id)).toEqual(['c1', 'c2'])
    expect(r.claims[0].verdict).toBe('NOT_CHECKED')
    expect(r.claims[1].verdict).toBe('FRESH')
  })

  it('completes the report and keeps its claims', () => {
    let r = applyEvent(applyEvent(undefined, started), event('report.claims', 'r1', { claims: [staleClaim()] }))
    r = applyEvent(
      r,
      event('report.complete', 'r1', {
        status: 'complete',
        score: 72,
        stale_count: 1,
        checked_count: 2,
        total_claims: 2,
        healed_count: 0,
        blocked: false,
        latency_ms: 6300,
        credits_used: 2,
      }),
    )
    expect(r).toMatchObject({ status: 'complete', score: 72, stale_count: 1, checked_count: 2, blocked: false })
    expect(r.claims).toHaveLength(1)
  })

  it('marks the report as failed on report.error', () => {
    const r = applyEvent(applyEvent(undefined, started), event('report.error', 'r1', { reason: 'Claim extraction failed.' }))
    expect(r).toMatchObject({ status: 'error', reason: 'Claim extraction failed.' })
  })

  it('ignores late start and claims events for a finished report', () => {
    const done = completeReport()
    expect(applyEvent(done, started)).toBe(done)
    expect(applyEvent(done, event('report.claims', 'r1', { claims: [] }))).toBe(done)
  })
})

describe('mergeReport', () => {
  it('a pending poll keeps claims from events', () => {
    const cached = normalizeReport({ id: 'r1', status: 'pending', claims: [staleClaim(), freshClaim()] })
    const fetched = normalizeReport({ id: 'r1', status: 'pending', answer_text: 'text', claims: [] })
    const merged = mergeReport(cached, fetched)
    expect(merged.claims).toHaveLength(2)
    expect(merged.answer_text).toBe('text')
  })

  it('a late pending poll does not undo a finished report', () => {
    const done = completeReport()
    expect(mergeReport(done, normalizeReport({ id: 'r1', status: 'pending' }))).toBe(done)
  })

  it('a finished report from the API wins', () => {
    const fetched = completeReport()
    expect(mergeReport(normalizeReport({ id: 'r1', status: 'pending' }), fetched)).toBe(fetched)
    expect(mergeReport(undefined, fetched)).toBe(fetched)
  })
})
