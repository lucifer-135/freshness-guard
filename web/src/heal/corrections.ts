import { diffWords } from 'diff'
import type { Claim, Report } from '@/api/types'
import { safeHref } from '@/claims/links'
import { segmentAnswer } from '@/claims/segments'

export type CorrectionSegment =
  | { kind: 'text'; text: string }
  | { kind: 'change'; before: string; after: string; claim: Claim | null }

export function citationFor(claim: Claim): { domain: string; href: string | null } | null {
  const source = claim.evidence.find((e) => e.stance === 'contradicts') ?? claim.evidence[0]
  if (!source) return null
  return { domain: source.domain, href: safeHref(source.url) }
}

// The plugin's inline annotation format (internal/heal/annotate.go).
function annotation(claim: Claim): string {
  const label = claim.verdict === 'WRONG' ? 'incorrect' : 'outdated'
  const asOf = claim.as_of ? ` as of ${claim.as_of}` : ''
  const domain = citationFor(claim)?.domain
  return ` [${label} — now ${claim.current_value}${asOf}${domain ? ` (${domain})` : ''}]`
}

function compact(segments: CorrectionSegment[]): CorrectionSegment[] {
  const out: CorrectionSegment[] = []
  for (const s of segments) {
    if (s.kind === 'text' && s.text === '') continue
    const last = out[out.length - 1]
    if (s.kind === 'text' && last?.kind === 'text') last.text += s.text
    else out.push(s.kind === 'text' ? { ...s } : s)
  }
  return out
}

// correctionSegments builds the corrected view of an async answer from each
// outdated claim's current value: the old value inside the quote is replaced,
// or, when the quote does not contain it, an annotation follows the quote.
export function correctionSegments(answer: string, claims: Claim[]): CorrectionSegment[] {
  const out: CorrectionSegment[] = []
  for (const s of segmentAnswer(answer, claims).segments) {
    if (s.kind === 'text') {
      out.push(s)
      continue
    }
    const c = s.claim
    if ((c.verdict !== 'STALE' && c.verdict !== 'WRONG') || !c.current_value) {
      out.push({ kind: 'text', text: s.text })
      continue
    }
    let at = c.value ? s.text.indexOf(c.value) : -1
    if (at === -1 && c.value) at = s.text.toLowerCase().indexOf(c.value.toLowerCase())
    if (at === -1) {
      out.push({ kind: 'text', text: s.text }, { kind: 'change', before: '', after: annotation(c), claim: c })
      continue
    }
    const end = at + c.value.length
    out.push(
      { kind: 'text', text: s.text.slice(0, at) },
      { kind: 'change', before: s.text.slice(at, end), after: c.current_value, claim: c },
      { kind: 'text', text: s.text.slice(end) },
    )
  }
  return compact(out)
}

// diffSegments shows a sync answer that the plugin corrected as a word diff.
// A change inside a word ("$" + "20" -> "25") is widened to the whole word.
export function diffSegments(original: string, healed: string, claims: Claim[]): CorrectionSegment[] {
  const claimAt = (pos: number) => claims.find((c) => c.span_start >= 0 && c.span_start <= pos && pos <= c.span_end) ?? null
  const out: CorrectionSegment[] = []
  let pos = 0
  const parts = diffWords(original, healed)
  for (let i = 0; i < parts.length; i++) {
    const part = parts[i]
    if (!part.added && !part.removed) {
      out.push({ kind: 'text', text: part.value })
      pos += part.value.length
      continue
    }
    const next = parts[i + 1]
    const before = part.removed ? part.value : ''
    const after = part.added ? part.value : next?.added ? next.value : ''
    if (part.removed && next?.added) i++
    out.push({ kind: 'change', before, after, claim: claimAt(pos) })
    pos += before.length
  }

  for (let i = 0; i < out.length; i++) {
    const s = out[i]
    if (s.kind !== 'change' || !s.before || !s.after) continue
    const prev = out[i - 1]
    const next = out[i + 1]
    const head = prev?.kind === 'text' && /\S$/.test(s.before[0] ?? '') ? (/\S+$/.exec(prev.text)?.[0] ?? '') : ''
    const tail = next?.kind === 'text' && /\S/.test(s.before[s.before.length - 1] ?? '') ? (/^\S+/.exec(next.text)?.[0] ?? '') : ''
    if (head && prev?.kind === 'text') prev.text = prev.text.slice(0, -head.length)
    if (tail && next?.kind === 'text') next.text = next.text.slice(tail.length)
    out[i] = { ...s, before: head + s.before + tail, after: head + s.after + tail }
  }
  return compact(out)
}

export function correctedView(report: Report): CorrectionSegment[] {
  return report.healed_text
    ? diffSegments(report.answer_text, report.healed_text, report.claims)
    : correctionSegments(report.answer_text, report.claims)
}

export function hasChanges(segments: CorrectionSegment[]): boolean {
  return segments.some((s) => s.kind === 'change')
}
