import { ListChecks } from 'lucide-react'
import { useCallback, useState } from 'react'
import { ClaimInspector } from './ClaimInspector'
import { Composer } from './Composer'
import { MessageView } from './MessageView'
import { useChat } from './ChatProvider'
import type { AssistantMessage } from './useChat'
import { CheckPanel } from '@/check/CheckPanel'
import { Button } from '@/components/ui/button'
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetTrigger } from '@/components/ui/sheet'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useReport } from '@/live/useReport'
import { useStatus } from '@/shell/TopBar'

export function ChatPage() {
  return (
    <section className="space-y-4">
      <h1 className="text-2xl font-semibold">Freshness-checked chat</h1>
      <Tabs defaultValue="chat">
        <TabsList>
          <TabsTrigger value="chat">Chat</TabsTrigger>
          <TabsTrigger value="check">Check any answer</TabsTrigger>
        </TabsList>
        <TabsContent value="chat">
          <Conversation />
        </TabsContent>
        <TabsContent value="check">
          <CheckPanel />
        </TabsContent>
      </Tabs>
    </section>
  )
}

function Conversation() {
  const { messages, busy, send } = useChat()
  const syncTimeoutMs = useStatus().data?.settings.sync_timeout_ms ?? 8000
  const [openClaim, setOpenClaim] = useState<string | null>(null)
  const [announcement, setAnnouncement] = useState('')
  const onStatus = useCallback((sentence: string) => setAnnouncement(sentence), [])

  const latest = [...messages].reverse().find((m): m is AssistantMessage => m.role === 'assistant' && m.reportId !== null)
  const report = useReport(latest?.reportId ?? null, { enabled: latest?.phase === 'done' && latest.mode !== 'off' })

  function select(claimId: string) {
    setOpenClaim(claimId)
    document.querySelector(`[data-claim-id="${CSS.escape(claimId)}"]`)?.scrollIntoView?.({ block: 'center' })
  }

  const inspector = report.data && <ClaimInspector report={report.data} onSelect={select} />

  return (
    <div className="flex gap-6">
      <div className="min-w-0 flex-1 space-y-4 xl:max-w-[760px]">
        <div aria-live="polite" className="sr-only">
          {announcement}
        </div>
        {messages.length === 0 ? (
          <p className="text-text-muted">Ask a question to see which parts of the answer are still current.</p>
        ) : (
          messages.map((m) => (
            <MessageView
              key={m.id}
              message={m}
              syncTimeoutMs={syncTimeoutMs}
              openClaimId={m === latest ? openClaim : undefined}
              onOpenChange={m === latest ? setOpenClaim : undefined}
              onStatus={m === latest ? onStatus : undefined}
            />
          ))
        )}
        {inspector && (
          <div className="xl:hidden">
            <Sheet>
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
                {inspector}
              </SheetContent>
            </Sheet>
          </div>
        )}
        <Composer busy={busy} onSend={send} />
      </div>
      {inspector && (
        <aside aria-label="Claims in this answer" className="hidden w-[360px] shrink-0 space-y-3 xl:block">
          <h2 className="eyebrow">Claims in this answer</h2>
          {inspector}
        </aside>
      )}
    </div>
  )
}
