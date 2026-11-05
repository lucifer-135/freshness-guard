import { describe, expect, it } from 'vitest'
import { byProvider, categoriesIn, cellSentence, heatmapSummary, isLowSample, rate } from './heatmap'
import type { Leaderboard, LeaderboardRow } from '@/api/types'

const row = (model: string, provider: string, cells: LeaderboardRow['cells']): LeaderboardRow => {
  const total = { checked: 0, outdated: 0, incorrect: 0 }
  for (const c of Object.values(cells)) {
    total.checked += c.checked
    total.outdated += c.outdated
    total.incorrect += c.incorrect
  }
  return { model, provider, total, cells }
}

const board = (rows: LeaderboardRow[]): Leaderboard => ({ window_days: 7, since: '2026-10-29T00:00:00Z', min_sample: 10, rows })

describe('heatmap helpers', () => {
  it('computes the rate of a metric and treats no checked claims as zero', () => {
    expect(rate({ checked: 40, outdated: 10, incorrect: 2 }, 'outdated')).toBe(0.25)
    expect(rate({ checked: 40, outdated: 10, incorrect: 2 }, 'incorrect')).toBe(0.05)
    expect(rate({ checked: 0, outdated: 0, incorrect: 0 }, 'outdated')).toBe(0)
  })

  it('marks values below the minimum sample as low sample', () => {
    expect(isLowSample({ checked: 9, outdated: 0, incorrect: 0 }, 10)).toBe(true)
    expect(isLowSample({ checked: 10, outdated: 0, incorrect: 0 }, 10)).toBe(false)
  })

  it('describes a cell with its percentage and sample size', () => {
    expect(cellSentence('outdated', 'pricing', { checked: 118, outdated: 40, incorrect: 0 }, 10)).toBe(
      'Outdated claims on pricing: 34% of 118 checked claims.',
    )
    expect(cellSentence('outdated', 'law', { checked: 1, outdated: 1, incorrect: 0 }, 1)).toBe('Outdated claims on law: 100% of 1 checked claim.')
  })

  it('says that a low-sample value has low confidence', () => {
    expect(cellSentence('incorrect', 'law', { checked: 4, outdated: 0, incorrect: 1 }, 10)).toBe(
      'Incorrect claims on law: 25% of 4 checked claims. This value is based on fewer than 10 claims and has low confidence.',
    )
  })

  it('lists the categories that have data in the standard order', () => {
    const lb = board([
      row('a', 'p', { version: { checked: 5, outdated: 1, incorrect: 0 }, pricing: { checked: 5, outdated: 1, incorrect: 0 } }),
      row('b', 'p', { law: { checked: 5, outdated: 0, incorrect: 0 } }),
    ])
    expect(categoriesIn(lb)).toEqual(['pricing', 'version', 'law'])
  })

  it('summarizes the highest rate among values with enough claims', () => {
    const lb = board([
      row('groq/openai/gpt-oss-20b', 'groq', {
        pricing: { checked: 118, outdated: 40, incorrect: 0 },
        law: { checked: 3, outdated: 3, incorrect: 0 },
      }),
      row('gemini/flash', 'gemini', { pricing: { checked: 50, outdated: 5, incorrect: 0 } }),
    ])
    expect(heatmapSummary(lb, 'outdated')).toBe(
      'The highest outdated-claim rate is groq/openai/gpt-oss-20b on pricing: 34% of 118 checked claims.',
    )
  })

  it('summarizes empty, low-sample-only and all-zero data', () => {
    expect(heatmapSummary(board([]), 'outdated')).toBe('No claims have been checked yet. Send a message in the chat to see results here.')
    const low = board([row('a', 'p', { pricing: { checked: 3, outdated: 2, incorrect: 0 } })])
    expect(heatmapSummary(low, 'outdated')).toBe('Every value so far is based on fewer than 10 checked claims.')
    const none = board([row('a', 'p', { pricing: { checked: 20, outdated: 0, incorrect: 0 } })])
    expect(heatmapSummary(none, 'outdated')).toBe('No outdated claims were found among 20 checked claims.')
  })

  it('groups rows by provider and sums their totals', () => {
    const rows = [
      row('groq/a', 'groq', { pricing: { checked: 10, outdated: 2, incorrect: 1 } }),
      row('groq/b', 'groq', { pricing: { checked: 30, outdated: 3, incorrect: 0 } }),
      row('gemini/c', 'gemini', { pricing: { checked: 5, outdated: 0, incorrect: 0 } }),
    ]
    const groups = byProvider(rows)
    expect(groups.map((g) => g.provider)).toEqual(['groq', 'gemini'])
    expect(groups[0].total).toEqual({ checked: 40, outdated: 5, incorrect: 1 })
    expect(groups[0].rows.map((r) => r.model)).toEqual(['groq/a', 'groq/b'])
  })
})
