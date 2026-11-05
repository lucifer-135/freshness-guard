import { useEffect, useRef, type ReactNode } from 'react'
import { useFineMotion } from './useFineMotion'
import { cn } from '@/lib/utils'

// SpotlightCard is a dashboard card with a faint light that follows the cursor
// (DESIGN.md section 4.2). The pointer position goes to CSS variables inside
// requestAnimationFrame, never into React state, and only a fine pointer
// without reduced motion gets the effect.
export function SpotlightCard({ children, className }: { children: ReactNode; className?: string }) {
  const fine = useFineMotion()
  const frame = useRef<number | null>(null)
  useEffect(
    () => () => {
      if (frame.current !== null) cancelAnimationFrame(frame.current)
    },
    [],
  )

  const handlers = fine
    ? {
        onPointerEnter: (e: React.PointerEvent<HTMLDivElement>) => e.currentTarget.style.setProperty('--spot', '1'),
        onPointerLeave: (e: React.PointerEvent<HTMLDivElement>) => e.currentTarget.style.setProperty('--spot', '0'),
        onPointerMove: (e: React.PointerEvent<HTMLDivElement>) => {
          const el = e.currentTarget
          const { clientX, clientY } = e
          if (frame.current !== null) cancelAnimationFrame(frame.current)
          frame.current = requestAnimationFrame(() => {
            const box = el.getBoundingClientRect()
            el.style.setProperty('--mx', `${clientX - box.left}px`)
            el.style.setProperty('--my', `${clientY - box.top}px`)
          })
        },
      }
    : {}

  return (
    <div className={cn('spotlight-card rounded-xl border border-border bg-surface-2 p-4', className)} {...handlers}>
      {children}
    </div>
  )
}
