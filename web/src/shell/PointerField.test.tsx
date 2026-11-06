import { act, fireEvent, render } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { PointerField } from './PointerField'

const frame = () => new Promise((resolve) => requestAnimationFrame(resolve))
const realMatchMedia = window.matchMedia

function reducedMotion(on: boolean) {
  window.matchMedia = ((query: string) => ({
    ...realMatchMedia(query),
    matches: on && query.includes('reduce'),
    addEventListener: () => {},
    removeEventListener: () => {},
  })) as typeof window.matchMedia
}

describe('PointerField', () => {
  afterEach(() => {
    window.matchMedia = realMatchMedia
  })

  it('is a decorative layer hidden from assistive technology and from the pointer', () => {
    const { container } = render(<PointerField />)
    const field = container.querySelector('[data-pointer-field]')!
    expect(field).toHaveAttribute('aria-hidden', 'true')
    expect(field).toHaveClass('pointer-events-none', 'fixed', 'inset-0')
  })

  it('follows a mouse once per frame, without re-rendering React', async () => {
    const { container } = render(<PointerField />)
    const field = container.querySelector<HTMLElement>('[data-pointer-field]')!
    fireEvent.pointerMove(window, { clientX: 120, clientY: 80, pointerType: 'mouse' })
    await act(frame)
    expect(field.style.getPropertyValue('--mx')).toBe('120px')
    expect(field.style.getPropertyValue('--my')).toBe('80px')
    expect(field).toHaveAttribute('data-active')
  })

  it('ignores touch, so a finger scrolling the page does not light the field', async () => {
    const { container } = render(<PointerField />)
    const field = container.querySelector<HTMLElement>('[data-pointer-field]')!
    fireEvent.pointerMove(window, { clientX: 120, clientY: 80, pointerType: 'touch' })
    await act(frame)
    expect(field).not.toHaveAttribute('data-active')
  })

  it('stays still when the reader asks for reduced motion', async () => {
    reducedMotion(true)
    const { container } = render(<PointerField />)
    const field = container.querySelector<HTMLElement>('[data-pointer-field]')!
    fireEvent.pointerMove(window, { clientX: 120, clientY: 80, pointerType: 'mouse' })
    await act(frame)
    expect(field).not.toHaveAttribute('data-active')
  })
})
