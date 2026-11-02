// @vitest-environment node
import { readdirSync, readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { expect, it } from 'vitest'

// DESIGN.md: components use colour tokens only, never raw hex values.
it('has no raw hex colours in components', () => {
  const src = fileURLToPath(new URL('..', import.meta.url))
  const files = readdirSync(src, { recursive: true, encoding: 'utf8' }).filter((f) => f.endsWith('.tsx'))
  expect(files.length).toBeGreaterThan(0)
  const offenders = files.filter((f) => /#[0-9a-fA-F]{3,8}\b/.test(readFileSync(`${src}/${f}`, 'utf8')))
  expect(offenders).toEqual([])
})
