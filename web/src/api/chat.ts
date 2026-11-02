import { ApiError, apiFetch } from './client'
import { readSSE } from './sse'
import type { FreshnessStatus, Mode, Policy } from './types'

export interface FreshnessHeaders {
  reportId: string | null
  status: FreshnessStatus | null
  score: number | null
  stale: number | null
  checked: number | null
  claims: number | null
  healed: number | null
}

function count(value: string | null | undefined): number | null {
  if (value == null || value === '') return null
  const n = Number(value)
  return Number.isFinite(n) ? n : null
}

export function parseFreshnessHeaders(h: Headers): FreshnessHeaders {
  const [checked, claims] = (h.get('X-Freshness-Checked') ?? '').split('/')
  return {
    reportId: h.get('X-Freshness-Report-Id'),
    status: h.get('X-Freshness-Status') as FreshnessStatus | null,
    score: count(h.get('X-Freshness-Score')),
    stale: count(h.get('X-Freshness-Stale')),
    checked: count(checked),
    claims: count(claims),
    healed: count(h.get('X-Freshness-Healed')),
  }
}

export interface ChatTurn {
  role: 'user' | 'assistant'
  content: string
}

export interface ChatRequest {
  model: string
  messages: ChatTurn[]
  mode: Mode
  policy: Policy | null
}

export type ChatResult =
  | { kind: 'answer'; text: string; headers: FreshnessHeaders }
  | { kind: 'blocked'; reportId: string | null; message: string }
  | { kind: 'failed'; message: string }

const unreachable = 'The plugin API could not be reached.'

interface ErrorBody {
  error?: string | { type?: string; message?: string }
}

async function failure(resp: Response): Promise<ChatResult> {
  let body: ErrorBody = {}
  try {
    body = (await resp.json()) as ErrorBody
  } catch {
    // Not JSON; the generic message below is used.
  }
  const error = body.error
  const message =
    (typeof error === 'string' ? error : error?.message) || `The request failed with HTTP ${resp.status}.`
  if (resp.status === 422 && typeof error === 'object' && error?.type === 'freshness_blocked') {
    const reportId = resp.headers.get('X-Freshness-Report-Id') ?? /Report ID: ([0-9a-f-]+)\./.exec(message)?.[1] ?? null
    return { kind: 'blocked', reportId, message }
  }
  return { kind: 'failed', message }
}

interface Completion {
  choices?: { message?: { content?: string }; delta?: { content?: string } }[]
  error?: { message?: string }
}

// sendChat sends one chat request through the plugin's chat proxy. Async and
// off answers stream; sync answers arrive whole, after verification.
export async function sendChat(
  req: ChatRequest,
  onDelta: (delta: string) => void,
  signal: AbortSignal,
): Promise<ChatResult> {
  const stream = req.mode !== 'sync'
  let resp: Response
  try {
    resp = await apiFetch('/freshness/chat', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ model: req.model, messages: req.messages, mode: req.mode, policy: req.policy, stream }),
      signal,
    })
  } catch (err) {
    if (err instanceof ApiError) return { kind: 'failed', message: err.message }
    if (signal.aborted) throw err
    return { kind: 'failed', message: unreachable }
  }
  if (!resp.ok) return failure(resp)

  const headers = parseFreshnessHeaders(resp.headers)
  const isStream = resp.body && (resp.headers.get('Content-Type') ?? '').includes('text/event-stream')
  if (!isStream || !resp.body) {
    const completion = (await resp.json()) as Completion
    return { kind: 'answer', text: completion.choices?.[0]?.message?.content ?? '', headers }
  }

  let text = ''
  let streamError: string | null = null
  await readSSE(resp.body, (m) => {
    if (m.data === '[DONE]') return
    const chunk = JSON.parse(m.data) as Completion
    if (chunk.error) streamError = chunk.error.message ?? 'The answer stream reported an error.'
    const delta = chunk.choices?.[0]?.delta?.content
    if (delta) {
      text += delta
      onDelta(delta)
    }
  })
  if (streamError) return { kind: 'failed', message: streamError }
  return { kind: 'answer', text, headers }
}
