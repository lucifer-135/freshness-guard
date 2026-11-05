import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterEach } from 'vitest'

afterEach(() => cleanup())

class NoopResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}

// jsdom implements neither matchMedia nor ResizeObserver, and Radix and the
// motion hooks use both; it also lacks scrollIntoView. Tests that need a matching query replace matchMedia.
function installBrowserStubs() {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    configurable: true,
    value: (query: string): MediaQueryList => ({
      matches: false,
      media: query,
      onchange: null,
      addEventListener: () => {},
      removeEventListener: () => {},
      addListener: () => {},
      removeListener: () => {},
      dispatchEvent: () => false,
    }),
  })
  window.ResizeObserver = NoopResizeObserver
  // cmdk scrolls the highlighted item into view, which jsdom does not implement.
  Element.prototype.scrollIntoView = () => {}
}

// Tests that read files run in the Node environment, which has no window.
if (typeof window !== 'undefined') installBrowserStubs()
