// @vitest-environment node
import { readFileSync } from 'node:fs'
import { expect, it } from 'vitest'

// WCAG 2.4.11: the sticky top bar and the docked composer must not hide the
// element that has keyboard focus, so scrolling keeps clear of both.
it('keeps focused elements clear of the sticky top bar and the docked composer', () => {
  const css = readFileSync(new URL('./index.css', import.meta.url), 'utf8')
  expect(css).toMatch(/scroll-padding-bottom:\s*var\(--composer-inset,\s*0px\)/)
  expect(css).toMatch(/scroll-padding-top:/)
})
