import { formatDate } from './format'
import { STATUS_META, uiStatus } from './status'
import type { Claim, Report } from '@/api/types'
import { Button } from '@/components/ui/button'

// ClaimPreview is the short card that opens when a claim in a chat answer is
// hovered (DESIGN.md section 7.1). The full explanation opens in the inspector.
export function ClaimPreview({ claim, report, onShowDetails }: { claim: Claim; report: Report; onShowDetails(): void }) {
  const status = uiStatus(claim, report.status)
  const meta = STATUS_META[status]
  const Icon = meta.icon
  const outdated = claim.verdict === 'STALE' || claim.verdict === 'WRONG'
  const asOf = claim.as_of ? formatDate(claim.as_of) : ''
  return (
    <div className="max-w-80 space-y-2 text-sm">
      <p className="flex items-center gap-2 font-medium">
        <Icon aria-hidden="true" className="size-4" style={{ color: `var(--status-${status === 'pending' ? 'unchecked' : status})` }} />
        <span>{meta.label}</span>
      </p>
      <p className="[overflow-wrap:anywhere]">“{claim.quote}”</p>
      {outdated && claim.current_value && (
        <p className="text-text-muted">{`was ${claim.value}, now ${claim.current_value}${asOf ? ` as of ${asOf}` : ''}`}</p>
      )}
      <Button type="button" variant="ghost" size="sm" onClick={onShowDetails}>
        Show details
      </Button>
    </div>
  )
}
