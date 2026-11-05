import { motion } from 'motion/react'
import { citationFor, type CorrectionSegment } from './corrections'
import type { Report } from '@/api/types'
import { useReducedMotion } from '@/motion/useReducedMotion'

const enter = { duration: 0.36, ease: [0.16, 1, 0.3, 1] as const }

// HealDiff is the corrected-answer view (DESIGN.md moment 2): each outdated
// value is struck through, the current value follows it, and a citation chip
// names the source. With the view off, the original answer is shown.
export function HealDiff({ segments, showCorrected }: { report: Report; segments: CorrectionSegment[]; showCorrected: boolean }) {
  const reduce = useReducedMotion()
  if (!showCorrected) {
    return (
      <p className="text-base leading-relaxed whitespace-pre-wrap">
        {segments.map((s) => (s.kind === 'text' ? s.text : s.before)).join('')}
      </p>
    )
  }
  return (
    <p className="text-base leading-relaxed whitespace-pre-wrap">
      {segments.map((s, i) => {
        if (s.kind === 'text') return <span key={i}>{s.text}</span>
        const citation = s.claim ? citationFor(s.claim) : null
        return (
          <span key={i}>
            {s.before && <del className="text-status-outdated decoration-2">{s.before}</del>}
            {s.before && s.after && ' '}
            <motion.ins
              className="text-status-current no-underline"
              initial={reduce ? { opacity: 0 } : { opacity: 0, y: 4 }}
              animate={{ opacity: 1, y: 0 }}
              transition={reduce ? { duration: 0.08 } : enter}
            >
              {s.after}
            </motion.ins>
            {citation && (
              <span className="ml-1 inline-flex rounded-sm border border-border bg-surface-2 px-1 align-middle font-mono text-[11px] text-text-muted">
                {citation.href ? (
                  <a href={citation.href} target="_blank" rel="noopener noreferrer">
                    {citation.domain}
                  </a>
                ) : (
                  citation.domain
                )}
              </span>
            )}
          </span>
        )
      })}
    </p>
  )
}
