import { useEffect, useId, useRef, useState, type CSSProperties, type KeyboardEvent, type ReactNode } from 'react'
import { ExplainCard } from './ExplainCard'
import { segmentAnswer } from './segments'
import { STATUS_META, uiStatus, type UiStatus } from './status'
import type { Claim, Report, ReportStatus } from '@/api/types'
import { Popover, PopoverAnchor, PopoverContent } from '@/components/ui/popover'

// DESIGN.md section 4.3: the card opens when the pointer rests 120 ms, and the
// message is restored 150 ms after the pointer leaves.
const OPEN_DELAY_MS = 120
const CLOSE_DELAY_MS = 150

function statusVar(status: UiStatus): string {
  return `var(--status-${status === 'pending' ? 'unchecked' : status})`
}

// CardSlot calls the caller's renderCard with `close` as a prop, so that the
// function that reads refs is never invoked while ClaimText itself renders.
function CardSlot({
  render,
  claim,
  report,
  close,
}: {
  render: (claim: Claim, report: Report, close: () => void) => ReactNode
  claim: Claim
  report: Report
  close: () => void
}) {
  return <>{render(claim, report, close)}</>
}

interface ClaimTextProps {
  text: string
  claims: Claim[]
  reportStatus: ReportStatus
  // Without a report, claims are underlined but have no explain card.
  report?: Report
  // When set, the parent controls which card is pinned open (the claim inspector does).
  openClaimId?: string | null
  onOpenChange?: (id: string | null) => void
  // The card shown beside a claim; `close` closes it and returns focus to the claim.
  // The default is the full explain card.
  renderCard?: (claim: Claim, report: Report, close: () => void) => ReactNode
}

// ClaimText renders an answer with each claim underlined by its status. Claims
// are focusable buttons whose description gives the status in words, so the
// meaning never depends on colour or underline style alone.
export function ClaimText({ text, claims, reportStatus, report, openClaimId, onOpenChange, renderCard }: ClaimTextProps) {
  const idPrefix = useId()
  const { segments } = segmentAnswer(text, claims)
  const placed = segments.flatMap((s) => (s.kind === 'claim' ? [s.claim] : []))

  const [ownPinned, setOwnPinned] = useState<string | null>(null)
  const pinned = openClaimId !== undefined ? openClaimId : ownPinned
  const [hovered, setHovered] = useState<string | null>(null)
  const openId = report ? (pinned ?? hovered) : null
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  const anchors = useRef(new Map<string, HTMLElement>())

  useEffect(() => () => clearTimeout(timer.current), [])

  function setPinned(id: string | null) {
    if (openClaimId === undefined) setOwnPinned(id)
    onOpenChange?.(id)
  }

  function hoverSoon(id: string | null, delay: number) {
    clearTimeout(timer.current)
    timer.current = setTimeout(() => setHovered(id), delay)
  }

  function close(id: string) {
    clearTimeout(timer.current)
    setHovered(null)
    setPinned(null)
    anchors.current.get(id)?.focus()
  }

  function onKeyDown(e: KeyboardEvent, id: string) {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault()
      setPinned(pinned === id ? null : id)
    }
  }

  return (
    <div>
      <p data-testid="answer-text" className="text-base leading-relaxed whitespace-pre-wrap">
        {segments.map((s, i) => {
          const dimmed = openId !== null && (s.kind === 'text' || s.claim.id !== openId)
          if (s.kind === 'text') {
            return (
              <span key={i} data-dimmed={dimmed || undefined} className="transition-opacity duration-[var(--dur-hover)] data-[dimmed]:opacity-55">
                {s.text}
              </span>
            )
          }
          const id = s.claim.id
          const status = uiStatus(s.claim, reportStatus)
          const open = openId === id
          const style: CSSProperties | undefined = open
            ? { backgroundColor: `color-mix(in srgb, ${statusVar(status)} 12%, transparent)` }
            : undefined
          const span = (
            <span
              ref={(el) => {
                if (el) anchors.current.set(id, el)
                else anchors.current.delete(id)
              }}
              role="button"
              tabIndex={0}
              data-claim-id={id}
              data-claim-status={status}
              data-dimmed={dimmed || undefined}
              className={`${STATUS_META[status].className} cursor-pointer rounded-sm transition-opacity duration-[var(--dur-hover)] data-[dimmed]:opacity-55`}
              style={style}
              aria-describedby={`${idPrefix}-${id}`}
              aria-expanded={report ? open : undefined}
              onPointerEnter={() => hoverSoon(id, OPEN_DELAY_MS)}
              onPointerLeave={() => hoverSoon(null, CLOSE_DELAY_MS)}
              onClick={() => setPinned(pinned === id ? null : id)}
              onKeyDown={(e) => onKeyDown(e, id)}
            >
              {s.text}
            </span>
          )
          if (!report) return <span key={i}>{span}</span>
          return (
            <Popover key={i} open={open} onOpenChange={(next) => !next && close(id)}>
              <PopoverAnchor asChild>{span}</PopoverAnchor>
              <PopoverContent
                className={`max-h-[70vh] max-w-[calc(100vw-2rem)] overflow-y-auto bg-surface-3 shadow-[var(--shadow-float)] ${renderCard ? 'w-auto' : 'w-96'}`}
                // A card opened by hovering must not take focus from the reader.
                onOpenAutoFocus={(e) => pinned !== id && e.preventDefault()}
                onCloseAutoFocus={(e) => e.preventDefault()}
                onInteractOutside={(e) => e.target instanceof Node && anchors.current.get(id)?.contains(e.target) && e.preventDefault()}
                onPointerEnter={() => clearTimeout(timer.current)}
                onPointerLeave={() => hoverSoon(null, CLOSE_DELAY_MS)}
              >
                {renderCard ? (
                  <CardSlot render={renderCard} claim={s.claim} report={{ ...report, status: reportStatus }} close={() => close(id)} />
                ) : (
                  <ExplainCard claim={s.claim} report={{ ...report, status: reportStatus }} />
                )}
              </PopoverContent>
            </Popover>
          )
        })}
      </p>
      <div hidden>
        {placed.map((claim) => {
          const meta = STATUS_META[uiStatus(claim, reportStatus)]
          return (
            <span key={claim.id} id={`${idPrefix}-${claim.id}`}>
              {meta.label}. {meta.description}
            </span>
          )
        })}
      </div>
    </div>
  )
}
