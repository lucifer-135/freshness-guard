import { useRef, useState, type KeyboardEvent, type PointerEvent } from 'react'
import { DAY_MS, probabilityAt, scrubSentence, type DecayModel } from './decay'

const WIDTH = 280
const HEIGHT = 96
const PAD = 4
const SAMPLES = 60

// DecayCurve plots the probability that a claim is outdated over time, with a
// time scrubber (DESIGN.md section 4.4). Pointer movement writes to the DOM in
// an animation frame; only keyboard steps go through React state.
export function DecayCurve({ model, today }: { model: DecayModel; today: Date }) {
  const start = model.origin.getTime() - model.offsetDays * DAY_MS
  const end = Math.max(today.getTime(), model.origin.getTime()) + 2 * model.halfLifeDays * DAY_MS
  const x = (t: number) => PAD + ((t - start) / (end - start)) * (WIDTH - 2 * PAD)
  const y = (p: number) => HEIGHT - PAD - p * (HEIGHT - 2 * PAD)

  const [scrub, setScrub] = useState(() => Math.min(Math.max(today.getTime(), start), end))
  const guide = useRef<SVGLineElement>(null)
  const label = useRef<HTMLParagraphElement>(null)
  const frame = useRef(0)

  const points = Array.from({ length: SAMPLES + 1 }, (_, i) => {
    const t = start + ((end - start) * i) / SAMPLES
    return `${x(t).toFixed(1)},${y(probabilityAt(model, new Date(t))).toFixed(1)}`
  })
  const sentence = scrubSentence(new Date(scrub), probabilityAt(model, new Date(scrub)))

  function onPointerMove(e: PointerEvent<SVGSVGElement>) {
    const rect = e.currentTarget.getBoundingClientRect()
    const share = Math.min(Math.max((e.clientX - rect.left) / rect.width, 0), 1)
    cancelAnimationFrame(frame.current)
    frame.current = requestAnimationFrame(() => {
      const t = start + share * (end - start)
      guide.current?.setAttribute('transform', `translate(${x(t) - x(scrub)} 0)`)
      if (label.current) label.current.textContent = scrubSentence(new Date(t), probabilityAt(model, new Date(t)))
    })
  }

  function onPointerLeave() {
    cancelAnimationFrame(frame.current)
    guide.current?.removeAttribute('transform')
    if (label.current) label.current.textContent = sentence
  }

  function onKeyDown(e: KeyboardEvent) {
    const step = (e.shiftKey ? 30 : 7) * DAY_MS
    if (e.key !== 'ArrowRight' && e.key !== 'ArrowLeft') return
    e.preventDefault()
    setScrub((t) => Math.min(Math.max(t + (e.key === 'ArrowRight' ? step : -step), start), end))
  }

  return (
    <div className="space-y-1">
      <svg
        data-chart
        width={WIDTH}
        height={HEIGHT}
        viewBox={`0 0 ${WIDTH} ${HEIGHT}`}
        className="max-w-full"
        aria-hidden="true"
        onPointerMove={onPointerMove}
        onPointerLeave={onPointerLeave}
      >
        <polyline points={points.join(' ')} fill="none" stroke="var(--accent)" strokeWidth="2" />
        <line x1={x(today.getTime())} x2={x(today.getTime())} y1={PAD} y2={HEIGHT - PAD} stroke="var(--border-strong)" strokeDasharray="3 3" />
        <text x={x(today.getTime()) + 4} y={PAD + 10} fill="var(--text-subtle)" fontSize="10">
          Today
        </text>
        <line ref={guide} x1={x(scrub)} x2={x(scrub)} y1={PAD} y2={HEIGHT - PAD} stroke="var(--text-muted)" />
      </svg>
      <div
        role="slider"
        tabIndex={0}
        aria-label="Date on the decay curve"
        aria-valuemin={0}
        aria-valuemax={Math.round((end - start) / DAY_MS)}
        aria-valuenow={Math.round((scrub - start) / DAY_MS)}
        aria-valuetext={sentence}
        onKeyDown={onKeyDown}
        className="rounded-sm"
      >
        <p ref={label} className="text-xs text-text-muted">
          {sentence}
        </p>
      </div>
    </div>
  )
}
