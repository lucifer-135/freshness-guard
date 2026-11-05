import type { Category, Counts, Leaderboard, LeaderboardRow, Metric } from '@/api/types'

// The categories in the order of the heatmap's columns.
export const CATEGORY_ORDER: Category[] = ['pricing', 'version', 'leadership', 'law', 'statistic', 'event', 'feature', 'static']

const EMPTY_SENTENCE = 'No claims have been checked yet. Send a message in the chat to see results here.'

export function categoriesIn(lb: Leaderboard): Category[] {
  return CATEGORY_ORDER.filter((c) => lb.rows.some((r) => r.cells[c] !== undefined))
}

export function rate(c: Counts, metric: Metric): number {
  return c.checked === 0 ? 0 : c[metric] / c.checked
}

export function isLowSample(c: Counts, minSample: number): boolean {
  return c.checked < minSample
}

const percent = (value: number) => `${Math.round(value * 100)}%`
const claimsOf = (n: number) => `${n} checked claim${n === 1 ? '' : 's'}`
const capitalized = (metric: Metric) => metric.charAt(0).toUpperCase() + metric.slice(1)

// cellSentence is the cell's tooltip and accessible name, for example
// "Outdated claims on pricing: 34% of 118 checked claims."
export function cellSentence(metric: Metric, category: Category, c: Counts, minSample: number): string {
  const sentence = `${capitalized(metric)} claims on ${category}: ${percent(rate(c, metric))} of ${claimsOf(c.checked)}.`
  return isLowSample(c, minSample) ? `${sentence} This value is based on fewer than ${minSample} claims and has low confidence.` : sentence
}

// heatmapSummary is the one-sentence summary of the heatmap for screen readers.
export function heatmapSummary(lb: Leaderboard, metric: Metric): string {
  if (lb.rows.length === 0) return EMPTY_SENTENCE
  let best: { model: string; category: Category; counts: Counts } | null = null
  let checked = 0
  for (const row of lb.rows) {
    for (const category of CATEGORY_ORDER) {
      const counts = row.cells[category]
      if (!counts || isLowSample(counts, lb.min_sample)) continue
      checked += counts.checked
      if (!best || rate(counts, metric) > rate(best.counts, metric)) best = { model: row.model, category, counts }
    }
  }
  if (!best) return `Every value so far is based on fewer than ${lb.min_sample} checked claims.`
  if (rate(best.counts, metric) === 0) return `No ${metric} claims were found among ${claimsOf(checked)}.`
  return `The highest ${metric}-claim rate is ${best.model} on ${best.category}: ${percent(rate(best.counts, metric))} of ${claimsOf(best.counts.checked)}.`
}

export interface ProviderGroup {
  provider: string
  total: Counts
  rows: LeaderboardRow[]
}

// byProvider groups the rows by provider, keeping the API's order.
export function byProvider(rows: LeaderboardRow[]): ProviderGroup[] {
  const groups: ProviderGroup[] = []
  for (const row of rows) {
    let group = groups.find((g) => g.provider === row.provider)
    if (!group) {
      group = { provider: row.provider, total: { checked: 0, outdated: 0, incorrect: 0 }, rows: [] }
      groups.push(group)
    }
    group.rows.push(row)
    group.total.checked += row.total.checked
    group.total.outdated += row.total.outdated
    group.total.incorrect += row.total.incorrect
  }
  return groups
}
