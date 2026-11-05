import type { Claim } from '@/api/types'

export type Segment = { kind: 'text'; text: string } | { kind: 'claim'; text: string; claim: Claim }

function isLowSurrogate(text: string, i: number): boolean {
  const code = text.charCodeAt(i)
  return code >= 0xdc00 && code <= 0xdfff
}

// segmentAnswer cuts the answer into plain text and claim segments. Spans are
// UTF-16 offsets, which are JavaScript string indices. A claim whose span is
// missing, out of range, empty or overlaps an earlier claim cannot be shown
// inline and is returned in unplaced. The segments always join to the text.
export function segmentAnswer(text: string, claims: Claim[]): { segments: Segment[]; unplaced: Claim[] } {
  const unplaced: Claim[] = []
  const placed: { start: number; end: number; claim: Claim }[] = []
  for (const claim of claims) {
    let { span_start: start, span_end: end } = claim
    if (start < 0 || end > text.length || start >= end) {
      unplaced.push(claim)
      continue
    }
    // A boundary between the two halves of a surrogate pair widens to the whole character.
    if (start > 0 && isLowSurrogate(text, start)) start--
    if (end < text.length && isLowSurrogate(text, end)) end++
    placed.push({ start, end, claim })
  }
  placed.sort((a, b) => a.start - b.start || b.end - a.end)

  const segments: Segment[] = []
  let cursor = 0
  for (const { start, end, claim } of placed) {
    if (start < cursor) {
      unplaced.push(claim)
      continue
    }
    if (start > cursor) segments.push({ kind: 'text', text: text.slice(cursor, start) })
    segments.push({ kind: 'claim', text: text.slice(start, end), claim })
    cursor = end
  }
  if (cursor < text.length || segments.length === 0) segments.push({ kind: 'text', text: text.slice(cursor) })
  return { segments, unplaced }
}
