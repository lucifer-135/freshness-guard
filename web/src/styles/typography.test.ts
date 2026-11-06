// @vitest-environment node
import { readFileSync } from 'node:fs'
import { expect, it } from 'vitest'

// DESIGN.md section 15: numbers in tables and times do not shift sideways as
// they change, and headings sit tight and wrap evenly.
const css = readFileSync(new URL('./index.css', import.meta.url), 'utf8')

it('uses tabular digits in tables and times', () => {
  expect(css).toMatch(/table,\s*time\s*\{[^}]*font-variant-numeric:\s*tabular-nums/)
})

it('tightens and balances the main headings', () => {
  expect(css).toMatch(/h1,\s*h2\s*\{[^}]*letter-spacing:\s*-0\.01em[^}]*text-wrap:\s*balance/)
})
