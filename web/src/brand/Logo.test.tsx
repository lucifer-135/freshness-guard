// @vitest-environment node
import { readdirSync, readFileSync } from 'node:fs'
import path from 'node:path'
import { renderToStaticMarkup } from 'react-dom/server'
import { expect, it } from 'vitest'
import { Logo } from './Logo'

const src = path.resolve(import.meta.dirname, '..')

it('draws the mark as a drained score ring ending in a dot for today, in the text colour', () => {
  const html = renderToStaticMarkup(<Logo />)
  expect(html).toContain('data-logo')
  expect(html).toContain('aria-hidden="true"')
  expect(html).toContain('Freshness Guard')
  expect(html).toMatch(/stroke="currentColor"/)
  // Colour means verdict, so the mark carries no saturated colour of its own.
  expect(html).not.toMatch(/#[0-9a-f]{3,8}|var\(--status/i)
})

it('replaces the stock shield icon everywhere the name is shown', () => {
  const files = readdirSync(src, { recursive: true, encoding: 'utf8' }).filter((f) => f.endsWith('.tsx') && !f.endsWith('.test.tsx'))
  const users = files.filter((f) => /ShieldCheck/.test(readFileSync(path.join(src, f), 'utf8')))
  expect(users).toEqual([])
})
