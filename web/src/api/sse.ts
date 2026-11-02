// A server-sent events reader for streams read with fetch: EventSource cannot
// send the bearer token the plugin API requires.

export interface SSEMessage {
  id: string | null
  event: string
  data: string
}

export function createSSEParser(onMessage: (m: SSEMessage) => void): { feed(chunk: string): void } {
  let buffer = ''
  let id: string | null = null
  let event = ''
  let data: string[] = []

  function dispatch() {
    if (data.length > 0) onMessage({ id, event: event || 'message', data: data.join('\n') })
    id = null
    event = ''
    data = []
  }

  function line(text: string) {
    if (text === '') return dispatch()
    if (text.startsWith(':')) return
    const colon = text.indexOf(':')
    const field = colon === -1 ? text : text.slice(0, colon)
    let value = colon === -1 ? '' : text.slice(colon + 1)
    if (value.startsWith(' ')) value = value.slice(1)
    if (field === 'data') data.push(value)
    else if (field === 'event') event = value
    else if (field === 'id') id = value
  }

  return {
    feed(chunk: string) {
      buffer += chunk
      for (;;) {
        const end = buffer.search(/[\r\n]/)
        if (end === -1) return
        // A trailing "\r" may be the first half of "\r\n"; wait for the next chunk.
        if (buffer[end] === '\r' && end === buffer.length - 1) return
        const skip = buffer[end] === '\r' && buffer[end + 1] === '\n' ? 2 : 1
        line(buffer.slice(0, end))
        buffer = buffer.slice(end + skip)
      }
    },
  }
}

export async function readSSE(body: ReadableStream<Uint8Array>, onMessage: (m: SSEMessage) => void): Promise<void> {
  const parser = createSSEParser(onMessage)
  const decoder = new TextDecoder()
  const reader = body.getReader()
  for (;;) {
    const { done, value } = await reader.read()
    if (done) break
    parser.feed(decoder.decode(value, { stream: true }))
  }
  parser.feed(decoder.decode())
}
