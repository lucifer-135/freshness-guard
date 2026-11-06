import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import LatticeLoader from '@/components/reactbits/LatticeLoader'

describe('LatticeLoader', () => {
  it('draws the elapsed time without reducing its opacity', () => {
    render(<LatticeLoader label="Checking claims" color="var(--text-muted)" />)
    const timer = screen.getByText('0.0s')
    // axe measures contrast with opacity applied, even on aria-hidden text, so a faded timer
    // failed the accessibility scan whenever the loader was still on screen.
    expect(timer.className).not.toMatch(/opacity/)
  })
})
