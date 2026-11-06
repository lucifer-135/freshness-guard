import { animate, motion, useMotionValue, useMotionValueEvent, useTransform, type AnimationPlaybackControls } from 'motion/react'
import { useEffect, useRef, type KeyboardEvent, type PointerEvent } from 'react'
import { useReducedMotion } from '@/motion/useReducedMotion'

// The example answer is built from three claims of the curated benchmark
// (bench/curated.json), with the verdicts and values recorded there.
const notes = [
  { tone: 'outdated', label: 'Outdated', value: 'Now $14.99, since 28 August 2026 · apple.com' },
  { tone: 'current', label: 'Current', value: 'Confirmed by live sources' },
  { tone: 'outdated', label: 'Outdated', value: 'Now $22.99, since 21 April 2026 · news.xbox.com' },
] as const

function Answer({ marked }: { marked: boolean }) {
  const claim = (tone: 'outdated' | 'current', text: string) => (marked ? <span className={`claim-${tone}`}>{text}</span> : text)
  return (
    <>
      <p className={`text-[clamp(1.75rem,3.4vw,2.75rem)] leading-[1.35] font-medium tracking-[-0.01em] ${marked ? 'text-text' : 'text-text-subtle'}`}>
        In the United States, {claim('outdated', 'Apple TV costs $12.99 per month')}, {claim('current', 'ChatGPT Plus costs $20 per month')}, and{' '}
        {claim('outdated', 'Xbox Game Pass Ultimate costs $29.99 per month')}.
      </p>
      <ol className={`mt-6 grid gap-4 sm:grid-cols-3 ${marked ? '' : 'invisible'}`}>
        {notes.map((n, i) => (
          <li key={i} className="text-sm">
            <span className={n.tone === 'outdated' ? 'font-medium text-status-outdated' : 'font-medium text-status-current'}>{n.label}</span>
            <span className="block font-mono text-xs text-text-muted">{n.value}</span>
          </li>
        ))}
      </ol>
    </>
  )
}

// ScanAnswer shows an answer as the model wrote it and a scan line that
// reveals the verdicts behind it. The line sweeps once, then the reader can
// drag it or move it with the arrow keys. Pointer movement never re-renders:
// the position lives in a motion value that drives a CSS variable.
export function ScanAnswer() {
  const reduced = useReducedMotion()
  const scan = useMotionValue(reduced ? 100 : 0)
  const scanVar = useTransform(scan, (v) => `${v}%`)
  const slider = useRef<HTMLDivElement>(null)
  const stage = useRef<HTMLDivElement>(null)
  const intro = useRef<AnimationPlaybackControls | null>(null)
  const dragging = useRef(false)

  useMotionValueEvent(scan, 'change', (v) => {
    slider.current?.setAttribute('aria-valuenow', String(Math.round(v)))
    slider.current?.setAttribute('aria-valuetext', v >= 100 ? 'The whole answer is checked' : `${Math.round(v)} percent of the answer checked`)
  })

  useEffect(() => {
    if (reduced) {
      scan.set(100)
      return
    }
    intro.current = animate(scan, 100, { duration: 1.6, delay: 0.4, ease: [0.16, 1, 0.3, 1] })
    return () => intro.current?.stop()
  }, [reduced, scan])

  function takeOver() {
    intro.current?.stop()
    intro.current = null
  }

  function moveTo(clientX: number) {
    const rect = stage.current?.getBoundingClientRect()
    if (!rect || rect.width === 0) return
    scan.set(Math.min(100, Math.max(0, ((clientX - rect.left) / rect.width) * 100)))
  }

  function onPointerDown(e: PointerEvent<HTMLDivElement>) {
    takeOver()
    dragging.current = true
    e.currentTarget.setPointerCapture(e.pointerId)
    moveTo(e.clientX)
  }

  function onKeyDown(e: KeyboardEvent<HTMLDivElement>) {
    const step = e.shiftKey ? 25 : 5
    const next = { ArrowRight: scan.get() + step, ArrowUp: scan.get() + step, ArrowLeft: scan.get() - step, ArrowDown: scan.get() - step, Home: 0, End: 100 }[e.key]
    if (next === undefined) return
    e.preventDefault()
    takeOver()
    scan.set(Math.min(100, Math.max(0, Math.round(next))))
  }

  return (
    <motion.div
      ref={stage}
      style={{ '--scan': scanVar } as unknown as Parameters<typeof motion.div>[0]['style']}
      className="relative cursor-ew-resize touch-pan-y select-none"
      onPointerDown={onPointerDown}
      onPointerMove={(e) => dragging.current && moveTo(e.clientX)}
      onPointerUp={() => (dragging.current = false)}
      onPointerCancel={() => (dragging.current = false)}
    >
      <div aria-hidden="true">
        <Answer marked={false} />
      </div>
      <div className="absolute inset-0" style={{ clipPath: 'inset(-1rem calc(100% - var(--scan)) -1rem 0)' }}>
        <Answer marked />
      </div>
      <div
        ref={slider}
        role="slider"
        tabIndex={0}
        aria-label="Scan position"
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={reduced ? 100 : 0}
        aria-orientation="horizontal"
        onKeyDown={onKeyDown}
        className="absolute -top-6 -bottom-4 w-px -translate-x-1/2 bg-text"
        style={{ left: 'var(--scan)' }}
      >
        <span className="absolute -top-1 left-1/2 -translate-x-1/2 rounded-sm bg-text px-1.5 py-0.5 font-mono text-[11px] leading-none whitespace-nowrap text-bg">
          scan
        </span>
      </div>
    </motion.div>
  )
}
