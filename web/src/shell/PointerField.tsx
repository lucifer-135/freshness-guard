import { useEffect, useRef } from 'react'
import { useReducedMotion } from '@/motion/useReducedMotion'

// PointerField is the page ground: a faint dot grid whose dots brighten in a
// soft circle around the mouse, like the scan revealing what is under it. The
// position is written to CSS variables once per frame, so moving the mouse never
// re-renders React. Touch input and reduced motion leave the grid still.
export function PointerField() {
  const field = useRef<HTMLDivElement>(null)
  const reduced = useReducedMotion()

  useEffect(() => {
    const el = field.current
    if (reduced || !el) return
    let frame = 0
    let x = 0
    let y = 0
    const onMove = (e: PointerEvent) => {
      if (e.pointerType !== 'mouse' && e.pointerType !== 'pen') return
      x = e.clientX
      y = e.clientY
      if (frame) return
      frame = requestAnimationFrame(() => {
        frame = 0
        el.style.setProperty('--mx', `${x}px`)
        el.style.setProperty('--my', `${y}px`)
        el.dataset.active = ''
      })
    }
    const onLeave = () => delete el.dataset.active
    window.addEventListener('pointermove', onMove, { passive: true })
    document.documentElement.addEventListener('pointerleave', onLeave)
    return () => {
      cancelAnimationFrame(frame)
      window.removeEventListener('pointermove', onMove)
      document.documentElement.removeEventListener('pointerleave', onLeave)
    }
  }, [reduced])

  return <div ref={field} data-pointer-field aria-hidden="true" className="pointer-field pointer-events-none fixed inset-0 -z-10" />
}
