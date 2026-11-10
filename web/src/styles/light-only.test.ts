// @vitest-environment node
import { readdirSync, readFileSync } from 'node:fs'
import path from 'node:path'
import { expect, test } from 'vitest'

// The developer chose on 10 November 2026 to ship one light theme with no dark
// theme and no theme switch. Tailwind's dark: variant follows the operating
// system's colour scheme, so any dark: class would still change the app for
// readers whose system is set to dark.
const webRoot = path.resolve(import.meta.dirname, '../..')
const read = (file: string) => readFileSync(path.resolve(webRoot, file), 'utf8')

test('the page tells the browser it is light', () => {
  expect(read('src/styles/index.css')).toMatch(/color-scheme:\s*light;/)
  expect(read('index.html')).toContain('<meta name="color-scheme" content="light" />')
})

test('nothing switches to a dark palette', () => {
  const css = read('src/styles/index.css')
  expect(css).not.toMatch(/prefers-color-scheme/)
  expect(css).not.toMatch(/color-scheme:\s*dark/)
  const src = path.resolve(webRoot, 'src')
  const files = readdirSync(src, { recursive: true, encoding: 'utf8' }).filter((f) => /\.(ts|tsx)$/.test(f) && !/\.test\.tsx?$/.test(f))
  expect(files.length).toBeGreaterThan(0)
  const offenders = files.filter((f) => /(^|[\s"'`])dark:/.test(readFileSync(path.join(src, f), 'utf8')))
  expect(offenders).toEqual([])
})

test('the browser tests render the light scheme', () => {
  expect(read('playwright.config.ts')).not.toMatch(/colorScheme:\s*'dark'/)
})

test('the favicon and the placeholder page use the light palette', () => {
  const favicon = read('public/favicon.svg')
  expect(favicon).toContain('fill="#FFFFFF"')
  expect(favicon).not.toMatch(/#07090D/i)
  const placeholder = read('../internal/webui/static/index.html')
  expect(placeholder).toMatch(/background: #ffffff/i)
  expect(placeholder).not.toMatch(/#07090d/i)
})

test('floating layers sit on the white page colour with a soft shadow', () => {
  const css = read('src/styles/index.css')
  expect(css).toContain('--popover: var(--bg);')
  expect(css).not.toContain('--highlight-top')
  expect(read('src/claims/ClaimText.tsx')).not.toContain('bg-surface-3')
})
