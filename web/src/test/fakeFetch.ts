import { vi } from 'vitest'

// A request as the fake fetch saw it. Requests are recorded field by field
// because jsdom's AbortSignal cannot be passed to Node's Request constructor.
export interface RecordedRequest {
  method: string
  path: string
  url: URL
  headers: Headers
  body: string | null
  signal: AbortSignal | null
}

export type Route = (req: RecordedRequest) => Response | Promise<Response>

export interface FakeFetch {
  requests: RecordedRequest[]
  restore(): void
}

// fakeFetch replaces fetch. Routes are keyed "METHOD /path"; the query string
// is not part of the key. A request without a route answers HTTP 404.
export function fakeFetch(routes: Record<string, Route>): FakeFetch {
  const requests: RecordedRequest[] = []
  const original = globalThis.fetch
  globalThis.fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = new URL(typeof input === 'string' ? input : input instanceof URL ? input.href : input.url, 'http://localhost')
    const req: RecordedRequest = {
      method: (init?.method ?? 'GET').toUpperCase(),
      path: url.pathname,
      url,
      headers: new Headers(init?.headers),
      body: typeof init?.body === 'string' ? init.body : null,
      signal: init?.signal ?? null,
    }
    requests.push(req)
    const route = routes[`${req.method} ${req.path}`]
    if (!route) return json(404, { error: `No fake route for ${req.method} ${req.path}.` })
    return route(req)
  }) as typeof fetch
  return {
    requests,
    restore: () => {
      globalThis.fetch = original
    },
  }
}

export function json(status: number, body: unknown, headers: Record<string, string> = {}): Response {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json', ...headers } })
}

export function streamBody(chunks: string[]): ReadableStream<Uint8Array> {
  const encoder = new TextEncoder()
  return new ReadableStream({
    start(controller) {
      for (const chunk of chunks) controller.enqueue(encoder.encode(chunk))
      controller.close()
    },
  })
}
