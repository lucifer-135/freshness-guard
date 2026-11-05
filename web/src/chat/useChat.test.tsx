import { act, renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { ChatProvider, useChat } from './ChatProvider'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { groupModels, splitModel } from './models'
import { setToken } from '@/api/client'
import { fakeFetch, json, type FakeFetch } from '@/test/fakeFetch'

const wrapper = ({ children }: { children: ReactNode }) => <ChatProvider>{children}</ChatProvider>

const options = { model: 'openai/fake-model', mode: 'sync' as const, policy: 'heal' as const }

describe('useChat', () => {
  let fake: FakeFetch
  beforeEach(() => setToken('t1'))
  afterEach(() => fake?.restore())

  it('sends the conversation so far with each message', async () => {
    let n = 0
    fake = fakeFetch({ 'POST /freshness/chat': () => json(200, { choices: [{ message: { content: `answer ${++n}` } }] }) })
    const { result } = renderHook(() => useChat(), { wrapper })
    act(() => result.current.send('first', options))
    await waitFor(() => expect(result.current.busy).toBe(false))
    act(() => result.current.send('second', options))
    await waitFor(() => expect(result.current.busy).toBe(false))
    const body = JSON.parse(fake.requests[1].body ?? '{}') as { messages: unknown[] }
    expect(body.messages).toEqual([
      { role: 'user', content: 'first' },
      { role: 'assistant', content: 'answer 1' },
      { role: 'user', content: 'second' },
    ])
  })

  it('stops the request when the app closes', async () => {
    fake = fakeFetch({
      'POST /freshness/chat': () =>
        new Response(new ReadableStream({ start() {} }), { headers: { 'Content-Type': 'text/event-stream' } }),
    })
    const { result, unmount } = renderHook(() => useChat(), { wrapper })
    act(() => result.current.send('hello', { ...options, mode: 'async' }))
    await waitFor(() => expect(fake.requests).toHaveLength(1))
    unmount()
    expect(fake.requests[0].signal?.aborted).toBe(true)
  })
})

describe('models', () => {
  it('splits the provider from the model name', () => {
    expect(splitModel('groq/openai/gpt-oss-20b')).toEqual({ provider: 'groq', name: 'openai/gpt-oss-20b' })
    expect(splitModel('plain')).toEqual({ provider: '', name: 'plain' })
  })

  it('groups models by provider', () => {
    expect(
      groupModels([
        { id: 'groq/a', provider: 'groq' },
        { id: 'gemini/b', provider: 'gemini' },
        { id: 'groq/c', provider: 'groq' },
      ]),
    ).toEqual([
      { provider: 'groq', models: [{ id: 'groq/a', provider: 'groq' }, { id: 'groq/c', provider: 'groq' }] },
      { provider: 'gemini', models: [{ id: 'gemini/b', provider: 'gemini' }] },
    ])
  })
})
