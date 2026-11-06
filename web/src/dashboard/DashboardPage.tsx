import { LiveFeed } from './LiveFeed'
import { StalenessHeatmap } from './StalenessHeatmap'
import { SummaryRow } from './SummaryRow'

// DashboardPage is the operator dashboard: a summary row, the heatmap and the
// live feed (DESIGN.md section 7.2). Charts for the knowledge cutoff and the
// half-life table arrive with the stretch features.
export function DashboardPage() {
  return (
    <section className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">Dashboard</h1>
        <p className="mt-1 text-text-muted">Freshness checks for every answer that passes through this gateway.</p>
      </div>
      <SummaryRow />
      <div className="grid gap-6 lg:grid-cols-12">
        <div className="lg:col-span-8">
          <StalenessHeatmap />
        </div>
        <div className="lg:col-span-4">
          <LiveFeed />
        </div>
      </div>
    </section>
  )
}
