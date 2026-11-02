import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, render, screen } from '@testing-library/react'
import type { ReactNode } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { EventStreamProvider, useConnection } from './EventStream'
import { reportKey } from './reportCache'
import { useReport } from './useReport'
import { normalizeReport, setToken } from '@/api/client'
import type { Report } from '@/api/types'
import { fakeFetch, json, streamBody, type FakeFetch } from '@/test/fakeFetch'
import { completeReport, event, sse, staleClaim } from '@/test/fixtures'

function Connection() {
  return <p>{useConnection()}</p>
}

function Observed({ id }: { id: string }) {
  const { data } = useReport(id)
  return <p>report {data?.status ?? 'none'}</p>
}

function renderStream(queryClient: QueryClient, children?: ReactNode) {
  return render(
    <QueryClientProvider client={queryClient}>
      <EventStreamProvider>
        <Connection />
        {children}
      </EventStreamProvider>
    </QueryClientProvider>,
  )
}

// A stream that stays open, like a live connection without new events.
function openStream(): ReadableStream<Uint8Array> {
  return new ReadableStream({ start() {} })
}

const flush = (ms = 0) => act(() => vi.advanceTimersByTimeAsync(ms))

describe('EventStreamProvider', () => {
  let fake: FakeFetch
  beforeEach(() => {
    vi.useFakeTimers()
    setToken('t1')
  })
  afterEach(() => {
    fake?.restore()
    vi.useRealTimers()
  })

  it('keeps events for a report the page has not seen', async () => {
    const queryClient = new QueryClient()
    fake = fakeFetch({
      'GET /freshness/events': () =>
        new Response(
          streamBody([
            sse(event('report.started', 'r9', { model: 'm', provider: 'p', mode: 'sync', policy: 'heal' })),
            sse(event('report.claims', 'r9', { claims: [staleClaim()] })),
          ]),
          { headers: { 'Content-Type': 'text/event-stream' } },
        ),
    })
    renderStream(queryClient)
    await flush()
    const cached = queryClient.getQueryData<Report>(reportKey('r9'))
    expect(cached?.status).toBe('pending')
    expect(cached?.claims).toHaveLength(1)
  })

  it('refetches an observed report when it completes', async () => {
    const queryClient = new QueryClient()
    fake = fakeFetch({
      'GET /freshness/events': () =>
        new Response(
          streamBody([
            sse(
              event('report.complete', 'r9', {
                status: 'complete',
                score: 72,
                stale_count: 1,
                checked_count: 2,
                total_claims: 2,
                healed_count: 0,
                blocked: false,
                latency_ms: 6300,
                credits_used: 2,
              }),
            ),
          ]),
          { headers: { 'Content-Type': 'text/event-stream' } },
        ),
      'GET /freshness/reports/r9': () => json(200, completeReport({ id: 'r9' })),
    })
    renderStream(queryClient, <Observed id="r9" />)
    await flush()
    await flush()
    expect(fake.requests.filter((r) => r.path === '/freshness/reports/r9').length).toBeGreaterThanOrEqual(2)
    expect(screen.getByText('report complete')).toBeInTheDocument()
  })

  it('401 from the event stream does not reconnect', async () => {
    fake = fakeFetch({ 'GET /freshness/events': () => json(401, { error: 'A valid bearer token is required.' }) })
    renderStream(new QueryClient())
    await flush(30000)
    expect(fake.requests.filter((r) => r.path === '/freshness/events')).toHaveLength(1)
  })

  it('reconnecting invalidates pending reports', async () => {
    const queryClient = new QueryClient()
    queryClient.setQueryData(reportKey('p'), normalizeReport({ id: 'p', status: 'pending' }))
    queryClient.setQueryData(reportKey('c'), completeReport({ id: 'c' }))
    let calls = 0
    fake = fakeFetch({
      'GET /freshness/events': () => {
        calls++
        const body = calls === 1 ? streamBody([]) : openStream()
        return new Response(body, { headers: { 'Content-Type': 'text/event-stream' } })
      },
    })
    renderStream(queryClient)
    await flush()
    expect(screen.getByText('reconnecting')).toBeInTheDocument()
    expect(queryClient.getQueryState(reportKey('p'))?.isInvalidated).toBe(false)

    await flush(1000)
    expect(calls).toBe(2)
    expect(screen.getByText('live')).toBeInTheDocument()
    expect(queryClient.getQueryState(reportKey('p'))?.isInvalidated).toBe(true)
    expect(queryClient.getQueryState(reportKey('c'))?.isInvalidated).toBe(false)
  })
})
