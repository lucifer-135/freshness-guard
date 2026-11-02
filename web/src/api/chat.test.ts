import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { parseFreshnessHeaders, sendChat, type ChatRequest } from './chat'
import { setToken } from './client'
import { fakeFetch, json, streamBody, type FakeFetch } from '@/test/fakeFetch'

const asyncRequest: ChatRequest = {
  model: 'openai/fake-model',
  messages: [{ role: 'user', content: 'What does it cost?' }],
  mode: 'async',
  policy: null,
}

function chunk(content: string): string {
  return `data: ${JSON.stringify({ choices: [{ delta: { content } }] })}\n\n`
}

describe('parseFreshnessHeaders', () => {
  it('reads every freshness header', () => {
    const h = new Headers({
      'X-Freshness-Report-Id': 'r1',
      'X-Freshness-Status': 'complete',
      'X-Freshness-Score': '72',
      'X-Freshness-Stale': '1',
      'X-Freshness-Checked': '2/3',
      'X-Freshness-Healed': '1',
    })
    expect(parseFreshnessHeaders(h)).toEqual({ reportId: 'r1', status: 'complete', score: 72, stale: 1, checked: 2, claims: 3, healed: 1 })
  })

  it('returns nulls without headers', () => {
    expect(parseFreshnessHeaders(new Headers())).toEqual({
      reportId: null,
      status: null,
      score: null,
      stale: null,
      checked: null,
      claims: null,
      healed: null,
    })
  })
})

describe('sendChat', () => {
  let fake: FakeFetch
  beforeEach(() => setToken('t1'))
  afterEach(() => fake?.restore())

  it('streams an async answer', async () => {
    fake = fakeFetch({
      'POST /freshness/chat': () =>
        new Response(streamBody([chunk('Example '), chunk('Cloud '), chunk('Pro.'), 'data: [DONE]\n\n']), {
          headers: { 'Content-Type': 'text/event-stream', 'X-Freshness-Report-Id': 'r1', 'X-Freshness-Status': 'pending' },
        }),
    })
    const onDelta = vi.fn()
    const result = await sendChat(asyncRequest, onDelta, new AbortController().signal)
    expect(onDelta).toHaveBeenCalledTimes(3)
    expect(result).toMatchObject({ kind: 'answer', text: 'Example Cloud Pro.' })
    expect(result.kind === 'answer' && result.headers.reportId).toBe('r1')
    expect(JSON.parse(fake.requests[0].body ?? '')).toMatchObject({ stream: true, mode: 'async', policy: null })
  })

  it('reads a sync answer from JSON', async () => {
    fake = fakeFetch({
      'POST /freshness/chat': () => json(200, { choices: [{ message: { content: 'Healed.' } }] }, { 'X-Freshness-Status': 'complete' }),
    })
    const result = await sendChat({ ...asyncRequest, mode: 'sync', policy: 'heal' }, vi.fn(), new AbortController().signal)
    expect(result).toMatchObject({ kind: 'answer', text: 'Healed.' })
    expect(JSON.parse(fake.requests[0].body ?? '')).toMatchObject({ stream: false, mode: 'sync', policy: 'heal' })
  })

  it('reports a blocked answer with the report ID from the message', async () => {
    fake = fakeFetch({
      'POST /freshness/chat': () =>
        json(422, {
          error: {
            type: 'freshness_blocked',
            message: 'The answer was blocked because it contains outdated or incorrect claims. Report ID: abc-123.',
          },
        }),
    })
    const result = await sendChat({ ...asyncRequest, mode: 'sync', policy: 'block' }, vi.fn(), new AbortController().signal)
    expect(result).toMatchObject({ kind: 'blocked', reportId: 'abc-123' })
  })

  it('reports a plugin API error', async () => {
    fake = fakeFetch({ 'POST /freshness/chat': () => json(502, { error: 'Bifrost could not be reached.' }) })
    expect(await sendChat(asyncRequest, vi.fn(), new AbortController().signal)).toEqual({
      kind: 'failed',
      message: 'Bifrost could not be reached.',
    })
  })

  it('reports an unreachable plugin API', async () => {
    fake = fakeFetch({
      'POST /freshness/chat': () => {
        throw new TypeError('fetch failed')
      },
    })
    expect(await sendChat(asyncRequest, vi.fn(), new AbortController().signal)).toEqual({
      kind: 'failed',
      message: 'The plugin API could not be reached.',
    })
  })
})
