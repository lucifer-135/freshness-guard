import { act, fireEvent, render, renderHook, screen } from '@testing-library/react'
import { Profiler } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { CursorAura } from './CursorAura'
import { MagneticButton } from './MagneticButton'
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

describe('CursorAura', () => {
  it('renders nothing with reduced motion', () => {
    mockMedia(['reduce'])
    const { container } = render(<CursorAura />)
    expect(container).toBeEmptyDOMElement()
  })

  it('becomes a lens over a claim without re-rendering', () => {
    mockMedia([])
    vi.stubGlobal('requestAnimationFrame', (cb: FrameRequestCallback) => {
      cb(0)
      return 1
    })
    const onRender = vi.fn()
    render(
      <>
        <span data-claim-status="outdated">claim</span>
        <Profiler id="aura" onRender={onRender}>
          <CursorAura />
        </Profiler>
      </>,
    )
    const aura = document.querySelector('[data-aura]') as HTMLElement
    expect(aura).toHaveAttribute('aria-hidden', 'true')
    const renders = onRender.mock.calls.length
    fireEvent.pointerMove(screen.getByText('claim'), { clientX: 10, clientY: 10 })
    expect(aura.dataset.mode).toBe('lens')
    expect(aura.style.getPropertyValue('--aura-color')).toBe('var(--status-outdated)')
    expect(onRender.mock.calls.length).toBe(renders)
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

describe('MagneticButton', () => {
  it('does not move with reduced motion', () => {
    mockMedia(['reduce'])
    render(<MagneticButton>Send</MagneticButton>)
    const button = screen.getByRole('button', { name: 'Send' })
    fireEvent.pointerMove(button, { clientX: 50, clientY: 50 })
    expect(button.closest('[style*="transform"]')).toBeNull()
  })
})
