import { normalizeClaim, normalizeReport } from '@/api/client'
import type { Claim, LiveEvent, Report } from '@/api/types'

// The fake provider's answer and its two claims, as the e2e suite sees them.
export const answer = 'Example Cloud Pro costs $20 per month, and its current version is 4.2.'

export function staleClaim(overrides: Partial<Claim> = {}): Claim {
  const quote = 'Example Cloud Pro costs $20 per month'
  return normalizeClaim({
    id: 'c1',
    quote,
    span_start: answer.indexOf(quote),
    span_end: answer.indexOf(quote) + quote.length,
    entity: 'Example Cloud Pro',
    attribute: 'price',
    value: '$20 per month',
    category: 'pricing',
    p_changed: 0.72,
    checked: true,
    verdict: 'STALE',
    current_value: '$25 per month',
    as_of: '2026-10-01',
    score: 8,
    half_life_days: 60,
    confidence: 0.85,
    rationale: 'Current pricing pages list $25 per month.',
    evidence: [
      {
        ref: 'e1',
        url: 'https://example.com/pricing',
        title: 'Example Cloud pricing',
        domain: 'example.com',
        published_date: '2026-10-01T00:00:00Z',
        snippet: 'Pro costs $25 per month.',
        authority: 1,
        stance: 'contradicts',
      },
    ],
    ...overrides,
  })
}

export function freshClaim(overrides: Partial<Claim> = {}): Claim {
  const quote = 'its current version is 4.2'
  return normalizeClaim({
    id: 'c2',
    quote,
    span_start: answer.indexOf(quote),
    span_end: answer.indexOf(quote) + quote.length,
    entity: 'Example Cloud Pro',
    attribute: 'version',
    value: '4.2',
    category: 'version',
    p_changed: 0.4,
    checked: true,
    verdict: 'FRESH',
    current_value: '4.2',
    score: 82,
    score_parts: { recency: 0.9, authority: 1, agreement: 0.67 },
    half_life_days: 90,
    confidence: 0.9,
    rationale: 'The release notes list 4.2 as the latest version.',
    ...overrides,
  })
}

export function completeReport(overrides: Partial<Report> = {}): Report {
  return normalizeReport({
    id: 'r1',
    created_at: '2026-11-01T18:42:00Z',
    model: 'openai/fake-model',
    provider: 'openai',
    mode: 'async',
    status: 'complete',
    answer_text: answer,
    score: 45,
    stale_count: 1,
    checked_count: 2,
    total_claims: 2,
    latency_ms: 6300,
    credits_used: 2,
    model_age_days: 412,
    claims: [staleClaim(), freshClaim()],
    ...overrides,
  })
}

let nextEventId = 1

export function event<T extends LiveEvent['type']>(
  type: T,
  reportId: string,
  data: Extract<LiveEvent, { type: T }>['data'],
): LiveEvent {
  return { id: nextEventId++, type, report_id: reportId, at: '2026-11-01T18:42:00Z', data } as LiveEvent
}

export function sse(e: LiveEvent): string {
  return `id: ${e.id}\nevent: ${e.type}\ndata: ${JSON.stringify(e)}\n\n`
}
