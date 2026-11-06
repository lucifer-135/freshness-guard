import { act, fireEvent, render, renderHook } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { SignalField } from './SignalField'
import { useFineMotion } from './useFineMotion'

type Listener = () => void
const original = window.matchMedia

// Replaces matchMedia; `matching` lists the queries that match. Returns a
// function that changes the matching queries and notifies listeners.
function mockMedia(matching: string[]) {
  let current = matching
  const listeners = new Set<Listener>()
  window.matchMedia = (query: string) =>
    ({
      matches: current.some((m) => query.includes(m)),
      media: query,
      onchange: null,
      addEventListener: (_: string, l: Listener) => listeners.add(l),
      removeEventListener: (_: string, l: Listener) => listeners.delete(l),
      addListener: () => {},
      removeListener: () => {},
      dispatchEvent: () => false,
    }) as unknown as MediaQueryList
  return (next: string[]) => {
    current = next
    listeners.forEach((l) => l())
  }
}

afterEach(() => {
  window.matchMedia = original
  vi.unstubAllGlobals()
})

describe('useFineMotion', () => {
  it('is on for a fine pointer without reduced motion', () => {
    mockMedia([])
    expect(renderHook(() => useFineMotion()).result.current).toBe(true)
  })

  it('is off with reduced motion or a coarse pointer, and follows changes', () => {
    const change = mockMedia(['reduce'])
    const { result } = renderHook(() => useFineMotion())
    expect(result.current).toBe(false)
    act(() => change(['coarse']))
    expect(result.current).toBe(false)
    act(() => change([]))
    expect(result.current).toBe(true)
  })
})

describe('SignalField', () => {
  it('is not drawn with reduced motion', () => {
    mockMedia(['reduce'])
    const { container } = render(<SignalField />)
    expect(container.querySelector('canvas')).toBeNull()
  })

  it('stops drawing while the tab is hidden', () => {
    mockMedia([])
    const cancel = vi.fn()
    vi.stubGlobal('requestAnimationFrame', () => 7)
    vi.stubGlobal('cancelAnimationFrame', cancel)
    render(<SignalField />)
    fireEvent.pointerMove(window, { clientX: 5, clientY: 5 })
    Object.defineProperty(document, 'visibilityState', { value: 'hidden', configurable: true })
    fireEvent(document, new Event('visibilitychange'))
    expect(cancel).toHaveBeenCalledWith(7)
    Object.defineProperty(document, 'visibilityState', { value: 'visible', configurable: true })
  })
})
