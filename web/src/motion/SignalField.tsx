import { useEffect, useRef } from 'react'
import { useFineMotion } from './useFineMotion'

const SPACING = 24
const RADIUS = 140
const BASE_ALPHA = 0.06
const NEAR_ALPHA = 0.18

// SignalField draws a faint grid of dots behind the shell; dots near the
// pointer brighten (DESIGN.md section 4.8). It draws at most once per frame and
// not at all while the tab is hidden.
export function SignalField() {
  return useFineMotion() ? <Field /> : null
}

function Field() {
  const canvas = useRef<HTMLCanvasElement>(null)

  useEffect(() => {
    const el = canvas.current
    const ctx = el?.getContext('2d') ?? null
    let frame = 0
    let px = -1000
    let py = -1000

    function draw() {
      frame = 0
      if (!el || !ctx) return
      const ratio = window.devicePixelRatio || 1
      const w = window.innerWidth
      const h = window.innerHeight
      if (el.width !== w * ratio || el.height !== h * ratio) {
        el.width = w * ratio
        el.height = h * ratio
      }
      ctx.setTransform(ratio, 0, 0, ratio, 0, 0)
      ctx.clearRect(0, 0, w, h)
      ctx.fillStyle = getComputedStyle(document.documentElement).getPropertyValue('--text-subtle').trim()
      for (let gx = SPACING / 2; gx < w; gx += SPACING) {
        for (let gy = SPACING / 2; gy < h; gy += SPACING) {
          ctx.globalAlpha = Math.hypot(gx - px, gy - py) < RADIUS ? NEAR_ALPHA : BASE_ALPHA
          ctx.fillRect(gx, gy, 1.5, 1.5)
        }
      }
    }

    function schedule() {
      if (frame || document.visibilityState === 'hidden') return
      frame = requestAnimationFrame(draw)
    }

    function onMove(e: PointerEvent) {
      px = e.clientX
      py = e.clientY
      schedule()
    }

    function onVisibility() {
      if (document.visibilityState === 'hidden') {
        cancelAnimationFrame(frame)
        frame = 0
      } else {
        schedule()
      }
    }

    schedule()
    window.addEventListener('pointermove', onMove, { passive: true })
    window.addEventListener('resize', schedule)
    document.addEventListener('visibilitychange', onVisibility)
    return () => {
      cancelAnimationFrame(frame)
      window.removeEventListener('pointermove', onMove)
      window.removeEventListener('resize', schedule)
      document.removeEventListener('visibilitychange', onVisibility)
    }
  }, [])

  return <canvas ref={canvas} aria-hidden="true" className="pointer-events-none fixed inset-0 -z-10 size-full" />
}
