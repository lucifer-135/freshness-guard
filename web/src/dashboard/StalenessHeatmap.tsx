import { useQuery } from '@tanstack/react-query'
import { useId, useState } from 'react'
import { byProvider, categoriesIn, cellSentence, heatmapSummary, isLowSample, rate } from './heatmap'
import { getLeaderboard, type ApiError } from '@/api/client'
import type { Category, Counts, Leaderboard, LeaderboardRow, Metric, StatsWindow } from '@/api/types'
import { percent } from '@/claims/format'
import { Segmented } from '@/components/Segmented'
import { Button } from '@/components/ui/button'
import { statsKey } from '@/live/keys'

const EMPTY = 'No claims have been checked yet. Send a message in the chat to see results here.'
const HINT = 'Hover over or focus a cell to see its exact value and sample size.'

const metricOptions: { value: Metric; label: string }[] = [
  { value: 'outdated', label: 'Outdated claims' },
  { value: 'incorrect', label: 'Incorrect claims' },
]
const windowOptions: { value: StatsWindow; label: string }[] = [
  { value: '7d', label: '7 days' },
  { value: '30d', label: '30 days' },
]

// The most a cell is coloured: the text on it stays readable at 4.5:1.
const MAX_MIX = 60

interface Active {
  rowKey: string
  category: Category
  sentence: string
}

function sum(rows: LeaderboardRow[], category: Category): Counts | undefined {
  const present = rows.map((r) => r.cells[category]).filter((c): c is Counts => c !== undefined)
  if (present.length === 0) return undefined
  return present.reduce((a, c) => ({ checked: a.checked + c.checked, outdated: a.outdated + c.outdated, incorrect: a.incorrect + c.incorrect }))
}

interface GridProps {
  lb: Leaderboard
  metric: Metric
  categories: Category[]
  active: Active | null
  onActive: (a: Active | null) => void
  describedBy: string
}

function HeatmapGrid({ lb, metric, categories, active, onActive, describedBy }: GridProps) {
  const color = metric === 'outdated' ? 'var(--status-outdated)' : 'var(--status-incorrect)'

  function cells(rowKey: string, rows: LeaderboardRow[]) {
    return categories.map((category) => {
      const counts = sum(rows, category)
      const crosshair = active !== null && (active.rowKey === rowKey || active.category === category)
      if (!counts) {
        return (
          <td key={category} data-crosshair={crosshair} className="px-3 py-2 text-text-subtle data-[crosshair=true]:bg-surface-3">
            No data
          </td>
        )
      }
      const sentence = cellSentence(metric, category, counts, lb.min_sample)
      const low = isLowSample(counts, lb.min_sample)
      return (
        <td
          key={category}
          tabIndex={0}
          aria-label={sentence}
          data-low-sample={low}
          data-active={active?.rowKey === rowKey && active.category === category}
          data-crosshair={crosshair}
          // oklab keeps the mix between rose and slate; oklch swung the hue through violet.
          // Low-sample cells are hatched, so their uncertainty is visible as well as written.
          style={
            low
              ? {
                  backgroundColor: 'var(--surface-2)',
                  backgroundImage: 'repeating-linear-gradient(135deg, var(--surface-2) 0 6px, var(--surface-3) 6px 12px)',
                }
              : { backgroundColor: `color-mix(in oklab, ${color} ${Math.round(rate(counts, metric) * MAX_MIX)}%, var(--surface-3))` }
          }
          className="px-3 py-2 text-center font-mono tabular-nums data-[active=true]:ring-2 data-[active=true]:ring-[var(--text)] data-[active=true]:ring-inset data-[crosshair=true]:brightness-125"
          onPointerEnter={() => onActive({ rowKey, category, sentence })}
          onPointerLeave={() => onActive(null)}
          onFocus={() => onActive({ rowKey, category, sentence })}
          onBlur={() => onActive(null)}
        >
          {percent(rate(counts, metric))}
          {low && <span className="block font-sans text-xs text-text-muted">Low sample</span>}
        </td>
      )
    })
  }

  return (
    <div className="overflow-x-auto rounded-lg border border-border">
      <table aria-label="Claim rates by model and category" aria-describedby={describedBy} className="w-full border-collapse text-sm">
        <thead className="bg-surface-1 text-text-muted">
          <tr>
            <th scope="col" className="px-3 py-2 text-left font-medium">
              Model
            </th>
            {categories.map((category) => (
              <th
                key={category}
                scope="col"
                data-crosshair={active?.category === category}
                className="px-3 py-2 font-medium capitalize data-[crosshair=true]:text-text"
              >
                {category}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {byProvider(lb.rows).map((group) => {
            const groupKey = `provider:${group.provider}`
            return [
              <tr key={groupKey} className="border-t border-border-strong bg-surface-1 font-semibold">
                <th scope="row" data-crosshair={active?.rowKey === groupKey} className="px-3 py-2 text-left data-[crosshair=true]:text-text">
                  {`All ${group.provider} models`}
                </th>
                {cells(groupKey, group.rows)}
              </tr>,
              ...group.rows.map((row) => (
                <tr key={row.model} className="border-t border-border">
                  <th
                    scope="row"
                    data-crosshair={active?.rowKey === row.model}
                    className="px-3 py-2 text-left font-mono text-xs font-normal data-[crosshair=true]:text-text"
                  >
                    {row.model}
                  </th>
                  {cells(row.model, [row])}
                </tr>
              )),
            ]
          })}
        </tbody>
      </table>
    </div>
  )
}

function CountsTable({ lb, categories }: { lb: Leaderboard; categories: Category[] }) {
  return (
    <div className="overflow-x-auto rounded-lg border border-border">
      <table aria-label="Claim counts by model and category" className="w-full text-left text-sm">
        <thead className="bg-surface-1 text-text-muted">
          <tr>
            {['Model', 'Category', 'Checked claims', 'Outdated', 'Incorrect'].map((h) => (
              <th key={h} scope="col" className="px-3 py-2 font-medium">
                {h}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {lb.rows.flatMap((row) =>
            categories.flatMap((category) => {
              const c = row.cells[category]
              if (!c) return []
              return [
                <tr key={`${row.model}:${category}`} className="border-t border-border">
                  <td className="px-3 py-2 font-mono text-xs">{row.model}</td>
                  <td className="px-3 py-2">{category}</td>
                  <td className="px-3 py-2 font-mono tabular-nums">{c.checked}</td>
                  <td className="px-3 py-2 font-mono tabular-nums">{c.outdated}</td>
                  <td className="px-3 py-2 font-mono tabular-nums">{c.incorrect}</td>
                </tr>,
              ]
            }),
          )}
        </tbody>
      </table>
    </div>
  )
}

// StalenessHeatmap shows the rate of outdated or incorrect claims by model and
// category (DESIGN.md sections 4.6 and 7.2). Every cell prints its value, so
// the colour is never the only signal.
export function StalenessHeatmap() {
  const [metric, setMetric] = useState<Metric>('outdated')
  const [period, setPeriod] = useState<StatsWindow>('7d')
  const [asTable, setAsTable] = useState(false)
  const [active, setActive] = useState<Active | null>(null)
  const summaryId = useId()
  const leaderboard = useQuery<Leaderboard, ApiError>({
    queryKey: [...statsKey, 'leaderboard', period],
    queryFn: () => getLeaderboard(period),
  })

  const lb = leaderboard.data
  return (
    <section aria-labelledby={`${summaryId}-title`} className="h-full space-y-4 rounded-xl border border-border bg-surface-2 p-4">
      <h2 id={`${summaryId}-title`} className="text-lg font-semibold">
        Claim rates by model and topic
      </h2>
      <div className="flex flex-wrap items-center gap-x-4 gap-y-3">
        <Segmented label="Show the rate of" value={metric} options={metricOptions} onChange={setMetric} />
        <Segmented label="Time window" value={period} options={windowOptions} onChange={setPeriod} />
        <Button variant="secondary" onClick={() => setAsTable((v) => !v)}>
          {asTable ? 'View as heatmap' : 'View as table'}
        </Button>
      </div>

      {leaderboard.isPending && (
        <div aria-busy="true" className="h-48 animate-pulse rounded-lg bg-surface-3">
          <p className="sr-only">Loading the leaderboard.</p>
        </div>
      )}
      {leaderboard.isError && (
        <div className="flex flex-wrap items-center gap-3">
          <p className="text-status-outdated">The leaderboard could not be loaded.</p>
          <Button variant="secondary" onClick={() => void leaderboard.refetch()}>
            Try again
          </Button>
        </div>
      )}
      {lb && lb.rows.length === 0 && <p className="text-text-muted">{EMPTY}</p>}
      {lb && lb.rows.length > 0 && (
        <>
          <p id={summaryId} className="text-sm text-text-muted">
            {heatmapSummary(lb, metric)} Cells turn redder as the rate rises, and each cell also shows its value as text.
          </p>
          {asTable ? (
            <CountsTable lb={lb} categories={categoriesIn(lb)} />
          ) : (
            <>
              <HeatmapGrid lb={lb} metric={metric} categories={categoriesIn(lb)} active={active} onActive={setActive} describedBy={summaryId} />
              <p className="min-h-6 text-sm">{active ? active.sentence : HINT}</p>
            </>
          )}
        </>
      )}
    </section>
  )
}
