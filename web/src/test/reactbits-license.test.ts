// @vitest-environment node
import { existsSync, readdirSync, readFileSync } from 'node:fs'
import path from 'node:path'
import { expect, test } from 'vitest'

// The developer chose on 7 November 2026 not to use React Bits in the new design.
// Its MIT + Commons Clause licence also kept the web app from being purely Apache-2.0.
const webRoot = path.resolve(import.meta.dirname, '../..')
const repoRoot = path.resolve(webRoot, '..')

test('no React Bits component is copied into the app', () => {
  expect(existsSync(path.join(webRoot, 'src/components/reactbits'))).toBe(false)
  const src = path.join(webRoot, 'src')
  const files = readdirSync(src, { recursive: true, encoding: 'utf8' }).filter((f) => /\.(ts|tsx)$/.test(f) && !f.endsWith('reactbits-license.test.ts'))
  const importers = files.filter((f) => /reactbits/i.test(readFileSync(path.join(src, f), 'utf8')))
  expect(importers).toEqual([])
})

test('no React Bits licence is shipped or listed', () => {
  expect(existsSync(path.join(webRoot, 'public/react-bits-license.md'))).toBe(false)
  expect(readFileSync(path.join(repoRoot, 'NOTICE'), 'utf8')).not.toMatch(/React Bits/)
  expect(readFileSync(path.join(repoRoot, 'README.md'), 'utf8')).not.toMatch(/React Bits/)
})
