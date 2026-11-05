import { motion, useMotionValue, useSpring } from 'motion/react'
import { useEffect, useRef } from 'react'
import { useFineMotion } from './useFineMotion'

// DESIGN.md section 3.6: critically damped, so the aura never overshoots.
const spring = { stiffness: 500, damping: 50, mass: 0.5 }

function auraFor(target: Element | null): { mode: string; color: string } {
  const claim = target?.closest<HTMLElement>('[data-claim-status]')
  if (claim) {
    const status = claim.dataset.claimStatus === 'pending' ? 'unchecked' : claim.dataset.claimStatus
    return { mode: 'lens', color: `var(--status-${status})` }
  }
  if (target?.closest('[data-chart]')) return { mode: 'crosshair', color: 'var(--accent)' }
  if (target?.closest('input, textarea, select, table')) return { mode: 'hidden', color: 'var(--accent)' }
  return { mode: 'ring', color: 'var(--accent)' }
}

// CursorAura is a soft ring that follows the pointer and becomes a lens over a
// claim (DESIGN.md section 4.1). Pointer positions go to motion values and
// data attributes inside an animation frame, never to React state.
export function CursorAura() {
  return useFineMotion() ? <Aura /> : null
}

function Aura() {
  const el = useRef<HTMLDivElement>(null)
  const x = useMotionValue(-100)
  const y = useMotionValue(-100)
  const sx = useSpring(x, spring)
  const sy = useSpring(y, spring)

  useEffect(() => {
    let frame = 0
    let last: PointerEvent | null = null
    function onMove(e: PointerEvent) {
      last = e
      if (frame) return
      frame = requestAnimationFrame(() => {
        frame = 0
        if (!last || !el.current) return
        x.set(last.clientX)
        y.set(last.clientY)
        const { mode, color } = auraFor(last.target instanceof Element ? last.target : null)
        el.current.dataset.mode = mode
        el.current.style.setProperty('--aura-color', color)
      })
    }
    window.addEventListener('pointermove', onMove, { passive: true })
    return () => {
      window.removeEventListener('pointermove', onMove)
      cancelAnimationFrame(frame)
    }
  }, [x, y])

  return (
    <motion.div ref={el} data-aura aria-hidden="true" data-mode="ring" className="aura" style={{ x: sx, y: sy }}>
      <span className="aura-ring" />
    </motion.div>
  )
}
