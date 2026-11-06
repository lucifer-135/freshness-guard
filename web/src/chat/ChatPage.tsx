import { ListChecks } from 'lucide-react'
import { useCallback, useLayoutEffect, useRef, useState, type RefObject } from 'react'
import { ClaimInspector } from './ClaimInspector'
import { Composer } from './Composer'
import { MessageView } from './MessageView'
import { useChat } from './ChatProvider'
import type { AssistantMessage } from './useChat'
import { CheckPanel } from '@/check/CheckPanel'
import { Button } from '@/components/ui/button'
import { Segmented } from '@/components/Segmented'
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetTrigger } from '@/components/ui/sheet'
import { useMediaQuery } from '@/lib/useMediaQuery'
import { useReport } from '@/live/useReport'
import { useStatus } from '@/shell/TopBar'

type Section = 'chat' | 'check'

const sections: { value: Section; label: string }[] = [
  { value: 'chat', label: 'Chat' },
  { value: 'check', label: 'Check any answer' },
]

export function ChatPage() {
  const [section, setSection] = useState<Section>('chat')
  return (
    // The chat and the pasted-text check share one centred frame, so switching between them moves nothing.
    <section className="mx-auto max-w-[1144px] space-y-4">
      <h1 className="sr-only">Freshness-checked chat</h1>
      <Segmented label="Page section" value={section} options={sections} onChange={setSection} />
      {section === 'chat' ? <Conversation /> : <CheckPanel />}
    </section>
  )
}

// useComposerInset publishes the docked composer's height as --composer-inset
// on the root element, where index.css turns it into scroll-padding-bottom, so
// a focused element is never scrolled under the composer.
function useComposerInset(ref: RefObject<HTMLElement | null>) {
  useLayoutEffect(() => {
    const el = ref.current
    if (!el) return
    const root = document.documentElement
    const update = () => root.style.setProperty('--composer-inset', `${el.offsetHeight}px`)
    update()
    const observer = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(update)
    observer?.observe(el)
    return () => {
      observer?.disconnect()
      root.style.removeProperty('--composer-inset')
    }
  }, [ref])
}

function Conversation() {
  const { messages, busy, send } = useChat()
  const composer = useRef<HTMLDivElement>(null)
  useComposerInset(composer)
  const syncTimeoutMs = useStatus().data?.settings.sync_timeout_ms ?? 8000
  // From Tailwind's xl breakpoint the inspector sits beside the chat; below it, it is a sheet.
  // The query uses rem like the CSS, so both agree when the reader changes the browser's font size.
  const wide = useMediaQuery('(min-width: 80rem)')
  // Claim IDs repeat across reports (c1, c2, ...), so a selection belongs to one report.
  const [selected, setSelected] = useState<{ reportId: string; claimId: string } | null>(null)
  const [announcement, setAnnouncement] = useState('')
  const [sheetOpen, setSheetOpen] = useState(false)
  const onStatus = useCallback((sentence: string) => setAnnouncement(sentence), [])

  const latest = [...messages].reverse().find((m): m is AssistantMessage => m.role === 'assistant' && m.reportId !== null)
  const report = useReport(latest?.reportId ?? null, { enabled: latest?.phase === 'done' && latest.mode !== 'off' })

  const selectedClaim = selected && latest && selected.reportId === latest.reportId ? selected.claimId : null

  function open(claimId: string) {
    if (latest?.reportId) setSelected({ reportId: latest.reportId, claimId })
  }

  // A row in the inspector opens that claim's details there, or closes them when they are open,
  // and brings the claim into view in the answer.
  function select(claimId: string) {
    if (selectedClaim === claimId) {
      setSelected(null)
      return
    }
    open(claimId)
    // Look only inside the latest answer, because claim IDs repeat across reports.
    const scope = latest && document.querySelector(`[data-message-id="${CSS.escape(latest.id)}"]`)
    scope?.querySelector(`[data-claim-id="${CSS.escape(claimId)}"]`)?.scrollIntoView?.({ block: 'center' })
  }

  // "Show details" in a claim's preview, or a click on the claim, opens the details where the inspector is.
  function showDetails(claimId: string) {
    open(claimId)
    if (!wide) setSheetOpen(true)
  }

  const inspector = (selectedId: string | null) =>
    report.data && <ClaimInspector report={report.data} selectedId={selectedId} onSelect={select} />

  return (
    // DESIGN.md 7.1: the conversation and the inspector sit in the centre of wide screens.
    <div className="mx-auto flex max-w-[1144px] gap-6">
      <div className="min-w-0 flex-1 space-y-4 xl:max-w-[760px]">
        <div aria-live="polite" className="sr-only">
          {announcement}
        </div>
        {messages.length === 0 ? (
          <p className="text-text-muted">Ask a question to see which parts of the answer are still current.</p>
        ) : (
          messages.map((m) => (
            <div key={m.id} data-message-id={m.id}>
              <MessageView
                message={m}
                syncTimeoutMs={syncTimeoutMs}
                openClaimId={m === latest ? null : undefined}
                onOpenChange={m === latest ? (id) => id && showDetails(id) : undefined}
                onShowDetails={m === latest ? showDetails : undefined}
                onStatus={m === latest ? onStatus : undefined}
              />
            </div>
          ))
        )}
        {report.data && (
          <div className="xl:hidden">
            <Sheet open={sheetOpen} onOpenChange={setSheetOpen}>
              <SheetTrigger asChild>
                <Button variant="outline" size="sm">
                  <ListChecks aria-hidden="true" />
                  Claims
                </Button>
              </SheetTrigger>
              <SheetContent side="bottom" className="max-h-[70vh] overflow-y-auto p-4">
                <SheetHeader>
                  <SheetTitle>Claims in this answer</SheetTitle>
                </SheetHeader>
                {inspector(wide ? null : selectedClaim)}
              </SheetContent>
            </Sheet>
          </div>
        )}
        <div ref={composer} className="sticky bottom-0 z-10 bg-bg pt-2 pb-4">
          <Composer busy={busy} onSend={send} />
        </div>
      </div>
      {report.data && (
        <aside aria-label="Claims in this answer" className="hidden w-[360px] shrink-0 space-y-3 xl:block">
          <h2 className="text-sm font-semibold">Claims in this answer</h2>
          {inspector(wide ? selectedClaim : null)}
        </aside>
      )}
    </div>
  )
}
