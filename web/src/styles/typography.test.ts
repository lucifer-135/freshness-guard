// @vitest-environment node
import { readdirSync, readFileSync } from 'node:fs'
import { expect, it } from 'vitest'

// DESIGN.md section 15: numbers in tables and times do not shift sideways as
// they change, and headings sit tight and wrap evenly.
const css = readFileSync(new URL('./index.css', import.meta.url), 'utf8')

it('uses tabular digits in tables and times', () => {
  expect(css).toMatch(/table,\s*time\s*\{[^}]*font-variant-numeric:\s*tabular-nums/)
})

// Small tracked uppercase labels above headings read as a template; headings carry their own weight.
it('has no eyebrow labels in the styles or the components', () => {
  expect(css).not.toMatch(/\.eyebrow/)
  expect(css).not.toMatch(/text-transform:\s*uppercase/)
  const src = new URL('../', import.meta.url)
  const files = readdirSync(src, { recursive: true, encoding: 'utf8' }).filter((f) => f.endsWith('.tsx') && !f.endsWith('.test.tsx'))
  const offenders = files.filter((f) => /\beyebrow\b/.test(readFileSync(new URL(f.replaceAll('\\', '/'), src), 'utf8')))
  expect(offenders).toEqual([])
})

it('themes text selection, the caret and scrollbars from the palette', () => {
  expect(css).toMatch(/::selection\s*\{[^}]*background(-color)?:\s*var\(--accent-soft-strong\)/)
  expect(css).toMatch(/caret-color:\s*var\(--accent\)/)
  expect(css).toMatch(/scrollbar-color:\s*var\(--border-strong\)\s+transparent/)
})

it('keeps the pointer field out of the 1200px content frame', () => {
  expect(css).toMatch(/\.pointer-field\s*\{[^}]*mask-image:\s*linear-gradient\(90deg,[^;]*calc\(50% - 600px\)[^;]*calc\(50% \+ 600px\)/)
})

it('tightens and balances the main headings', () => {
  expect(css).toMatch(/h1,\s*h2\s*\{[^}]*letter-spacing:\s*-0\.01em[^}]*text-wrap:\s*balance/)
})
