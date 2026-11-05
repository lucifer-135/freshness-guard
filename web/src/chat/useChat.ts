import { useCallback, useEffect, useRef, useState } from 'react'
import { sendChat, type ChatTurn, type FreshnessHeaders } from '@/api/chat'
import type { Mode, Policy } from '@/api/types'

export interface UserMessage {
  id: string
  role: 'user'
  text: string
}

export interface AssistantMessage {
  id: string
  role: 'assistant'
  model: string
  mode: Mode
  policy: Policy | null
  text: string
  phase: 'streaming' | 'done' | 'blocked' | 'failed'
  headers: FreshnessHeaders | null
  reportId: string | null
  elapsedMs: number | null
  error: string | null
}

export type Message = UserMessage | AssistantMessage

let nextId = 0
const newId = () => `m${++nextId}`

function history(messages: Message[]): ChatTurn[] {
  return messages.flatMap((m): ChatTurn[] => {
    if (m.role === 'user') return [{ role: 'user', content: m.text }]
    return m.phase === 'done' ? [{ role: 'assistant', content: m.text }] : []
  })
}

export interface ChatSession {
  messages: Message[]
  busy: boolean
  send(text: string, options: { model: string; mode: Mode; policy: Policy | null }): void
}

// useChatState holds one conversation in memory and sends each message, with
// the conversation so far, through the plugin's chat proxy. ChatProvider owns
// it above the routes, so the conversation and a running request survive tab
// switches and page changes; the request stops only when the app closes.
export function useChatState(): ChatSession {
  const [messages, setMessages] = useState<Message[]>([])
  const [busy, setBusy] = useState(false)
  const latest = useRef<Message[]>([])
  const controller = useRef<AbortController | null>(null)

  useEffect(() => () => controller.current?.abort(), [])

  const update = useCallback((fn: (all: Message[]) => Message[]) => {
    latest.current = fn(latest.current)
    setMessages(latest.current)
  }, [])

  const patch = useCallback(
    (id: string, fn: (m: AssistantMessage) => AssistantMessage) =>
      update((all) => all.map((m) => (m.id === id && m.role === 'assistant' ? fn(m) : m))),
    [update],
  )

  const send = useCallback(
    (text: string, options: { model: string; mode: Mode; policy: Policy | null }) => {
      const policy = options.mode === 'sync' ? options.policy : null
      const turns: ChatTurn[] = [...history(latest.current), { role: 'user', content: text }]
      const id = newId()
      update((all) => [
        ...all,
        { id: newId(), role: 'user', text },
        { id, role: 'assistant', model: options.model, mode: options.mode, policy, text: '', phase: 'streaming', headers: null, reportId: null, elapsedMs: null, error: null },
      ])
      const started = Date.now()
      const ac = new AbortController()
      controller.current = ac
      setBusy(true)
      sendChat({ model: options.model, messages: turns, mode: options.mode, policy }, (delta) => patch(id, (m) => ({ ...m, text: m.text + delta })), ac.signal)
        .then((result) => {
          const elapsedMs = Date.now() - started
          if (result.kind === 'answer') {
            patch(id, (m) => ({ ...m, text: result.text, phase: 'done', headers: result.headers, reportId: result.headers.reportId, elapsedMs }))
          } else if (result.kind === 'blocked') {
            patch(id, (m) => ({ ...m, phase: 'blocked', reportId: result.reportId, error: result.message, elapsedMs }))
          } else {
            patch(id, (m) => ({ ...m, phase: 'failed', error: result.message, elapsedMs }))
          }
        })
        .catch((err: unknown) => {
          if (ac.signal.aborted) return
          patch(id, (m) => ({ ...m, phase: 'failed', error: err instanceof Error ? err.message : 'The request failed.' }))
        })
        .finally(() => {
          if (!ac.signal.aborted) setBusy(false)
        })
    },
    [patch, update],
  )

  return { messages, busy, send }
}
