import { useEffect, useState } from 'react'
import type { Claim, ReportStatus } from '@/api/types'
import { useReducedMotion } from '@/motion/useReducedMotion'

function judged(claims: Claim[]): string[] {
  return claims.filter((c) => c.checked && c.verdict !== 'NOT_CHECKED').map((c) => c.id)
}

// The judge returns every verdict at once, so they are revealed one at a time
// and the progress sentence counts up (DESIGN.md moment 1). Until the last one
// is shown, the report counts as pending, so hidden verdicts read as
// "Not checked yet" rather than "Not checked (low risk)".
export function useStaggeredClaims(
  claims: Claim[],
  reportStatus: ReportStatus,
  stepMs = 250,
): { claims: Claim[]; reportStatus: ReportStatus } {
  const reduce = useReducedMotion()
  const ids = judged(claims)
  // A report that was already finished when it was first shown is not animated.
  const [shown, setShown] = useState(() => (reportStatus === 'pending' ? 0 : ids.length))
  const target = ids.length
  const visible = reduce ? target : Math.min(shown, target)

  useEffect(() => {
    if (reduce || shown >= target) return
    const timer = setTimeout(() => setShown((n) => n + 1), stepMs)
    return () => clearTimeout(timer)
  }, [reduce, shown, target, stepMs])

  const hidden = new Set(ids.slice(visible))
  return {
    claims: claims.map((c) => (hidden.has(c.id) ? { ...c, verdict: 'NOT_CHECKED' } : c)),
    reportStatus: visible < target ? 'pending' : reportStatus,
  }
}
