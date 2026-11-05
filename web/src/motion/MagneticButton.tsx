import { motion, useMotionValue, useSpring } from 'motion/react'
import type { ComponentProps, PointerEvent } from 'react'
import { useFineMotion } from './useFineMotion'
import { Button } from '@/components/ui/button'

// DESIGN.md section 3.6: a gentle pull that keeps the button inside its hit area.
const spring = { stiffness: 300, damping: 20 }
const STRENGTH = 0.3

// MagneticButton is the one primary action per screen that leans toward the
// pointer (DESIGN.md section 4.7).
export function MagneticButton(props: ComponentProps<typeof Button>) {
  const fine = useFineMotion()
  const x = useMotionValue(0)
  const y = useMotionValue(0)
  const sx = useSpring(x, spring)
  const sy = useSpring(y, spring)

  if (!fine) return <Button {...props} />

  function onPointerMove(e: PointerEvent<HTMLSpanElement>) {
    const rect = e.currentTarget.getBoundingClientRect()
    x.set((e.clientX - (rect.left + rect.width / 2)) * STRENGTH)
    y.set((e.clientY - (rect.top + rect.height / 2)) * STRENGTH)
  }

  return (
    <motion.span
      className="inline-block"
      style={{ x: sx, y: sy }}
      onPointerMove={onPointerMove}
      onPointerLeave={() => {
        x.set(0)
        y.set(0)
      }}
    >
      <Button {...props} />
    </motion.span>
  )
}
