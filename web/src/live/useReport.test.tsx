import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook } from '@testing-library/react'
import type { ReactNode } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useReport } from './useReport'
import { setToken } from '@/api/client'
import { fakeFetch, json, type FakeFetch } from '@/test/fakeFetch'
import { completeReport } from '@/test/fixtures'

function wrapper(queryClient: QueryClient) {
  return ({ children }: { children: ReactNode }) => <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
}

const flush = (ms = 0) => act(() => vi.advanceTimersByTimeAsync(ms))

describe('useReport', () => {
  let fake: FakeFetch
  beforeEach(() => {
    vi.useFakeTimers()
    setToken('t1')
  })
  afterEach(() => {
    fake?.restore()
    vi.useRealTimers()
  })

  it('polls a pending report until it completes', async () => {
    let calls = 0
    fake = fakeFetch({
      'GET /freshness/reports/r1': () => {
        calls++
        return calls === 1 ? json(200, { id: 'r1', status: 'pending', claims: [] }) : json(200, completeReport())
      },
    })
    const { result } = renderHook(() => useReport('r1'), { wrapper: wrapper(new QueryClient()) })
    await flush()
    expect(result.current.data?.status).toBe('pending')
    // A little past the interval: the refetched result is published by a 0 ms timer.
    await flush(3100)
    expect(result.current.data?.status).toBe('complete')
    await flush(10000)
    expect(calls).toBe(2)
  })

  it('gives up on a report that is not found', async () => {
    fake = fakeFetch({ 'GET /freshness/reports/gone': () => json(404, { error: 'The report was not found.' }) })
    const { result } = renderHook(() => useReport('gone'), { wrapper: wrapper(new QueryClient()) })
    await flush(10000)
    expect(fake.requests).toHaveLength(5)
    expect(result.current.isError).toBe(true)
    expect(result.current.error?.status).toBe(404)
  })

  it('does nothing without an ID', async () => {
    fake = fakeFetch({})
    renderHook(() => useReport(null), { wrapper: wrapper(new QueryClient()) })
    await flush(5000)
    expect(fake.requests).toHaveLength(0)
  })
})
