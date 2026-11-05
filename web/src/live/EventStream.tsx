import { useQueryClient, type QueryClient } from '@tanstack/react-query'
import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import { feedKey, statsKey } from './keys'
import { applyEvent, reportKey } from './reportCache'
import { ApiError, apiFetch } from '@/api/client'
import { readSSE } from '@/api/sse'
import type { LiveEvent, Report } from '@/api/types'

export type Connection = 'connecting' | 'live' | 'reconnecting'

const ConnectionContext = createContext<Connection>('connecting')

// Delays before reconnecting after a dropped stream; the last one repeats.
const RECONNECT_DELAYS_MS = [1000, 2000, 5000, 10000]

function apply(queryClient: QueryClient, e: LiveEvent) {
  // Removing cached answers changes the dashboard numbers, not any report.
  if (e.type === 'cache.purged') {
    void queryClient.invalidateQueries({ queryKey: statsKey })
    return
  }
  queryClient.setQueryData<Report>(reportKey(e.report_id), (prev) => applyEvent(prev, e))
  // Events carry no answer text or corrected text; load the stored report.
  if (e.type === 'report.complete' || e.type === 'report.error') {
    void queryClient.invalidateQueries({ queryKey: reportKey(e.report_id) })
    void queryClient.invalidateQueries({ queryKey: statsKey })
    void queryClient.invalidateQueries({ queryKey: feedKey })
  }
}

// Events may have been missed while the stream was down, so pending reports
// are loaded again.
function invalidatePending(queryClient: QueryClient) {
  void queryClient.invalidateQueries({
    predicate: (q) => q.queryKey[0] === 'report' && (q.state.data as Report | undefined)?.status === 'pending',
  })
}

// EventStreamProvider keeps one connection to GET /freshness/events and writes
// every event into the report cache.
export function EventStreamProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient()
  const [connection, setConnection] = useState<Connection>('connecting')

  useEffect(() => {
    const controller = new AbortController()
    let stopped = false
    let failures = 0
    let connectedBefore = false
    let timer: ReturnType<typeof setTimeout> | undefined

    async function connect() {
      try {
        const resp = await apiFetch('/freshness/events', { signal: controller.signal })
        if (!resp.ok || !resp.body) throw new Error(`The event stream answered with HTTP ${resp.status}.`)
        setConnection('live')
        if (connectedBefore) invalidatePending(queryClient)
        connectedBefore = true
        failures = 0
        await readSSE(resp.body, (m) => {
          try {
            apply(queryClient, JSON.parse(m.data) as LiveEvent)
          } catch (err) {
            console.error('An event from the plugin could not be read.', err)
          }
        })
      } catch (err) {
        // A rejected token signs the user out; reconnecting would only repeat it.
        if (stopped || (err instanceof ApiError && err.status === 401)) return
      }
      if (stopped) return
      setConnection('reconnecting')
      timer = setTimeout(connect, RECONNECT_DELAYS_MS[Math.min(failures, RECONNECT_DELAYS_MS.length - 1)])
      failures++
    }

    void connect()
    return () => {
      stopped = true
      clearTimeout(timer)
      controller.abort()
    }
  }, [queryClient])

  return <ConnectionContext.Provider value={connection}>{children}</ConnectionContext.Provider>
}

// eslint-disable-next-line react-refresh/only-export-components -- the hook belongs with its provider
export function useConnection(): Connection {
  return useContext(ConnectionContext)
}
