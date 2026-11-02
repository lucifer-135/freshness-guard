import { describe, expect, it } from 'vitest'
import { createSSEParser, readSSE, type SSEMessage } from './sse'
import { streamBody } from '@/test/fakeFetch'

function parse(chunks: string[]): SSEMessage[] {
  const out: SSEMessage[] = []
  const parser = createSSEParser((m) => out.push(m))
  for (const chunk of chunks) parser.feed(chunk)
  return out
}

describe('createSSEParser', () => {
  const event = 'id: 1\nevent: report.started\ndata: {"a":1}\n\n'

  it('parses one event', () => {
    expect(parse([event])).toEqual([{ id: '1', event: 'report.started', data: '{"a":1}' }])
  })

  it('parses the same event fed one character at a time', () => {
    expect(parse(event.split(''))).toEqual([{ id: '1', event: 'report.started', data: '{"a":1}' }])
  })

  it('ignores comments', () => {
    expect(parse([': connected\n\n', ': ping\n\n'])).toEqual([])
  })

  it('joins data lines and accepts CRLF line endings', () => {
    expect(parse(['data: a\ndata: b\n\n'])).toEqual([{ id: null, event: 'message', data: 'a\nb' }])
    expect(parse(['data: a\r\n', '\r\n'])).toEqual([{ id: null, event: 'message', data: 'a' }])
    expect(parse(['data: a\r', '\n\r\n'])).toEqual([{ id: null, event: 'message', data: 'a' }])
  })

  it('parses an OpenAI-style stream', () => {
    expect(parse(['data: {"x":1}\n\ndata: [DONE]\n\n'])).toEqual([
      { id: null, event: 'message', data: '{"x":1}' },
      { id: null, event: 'message', data: '[DONE]' },
    ])
  })
})

describe('readSSE', () => {
  it('reads every message from a byte stream', async () => {
    const out: SSEMessage[] = []
    await readSSE(streamBody(['data: o', 'ne\n\ndata: two\n\n']), (m) => out.push(m))
    expect(out.map((m) => m.data)).toEqual(['one', 'two'])
  })
})
