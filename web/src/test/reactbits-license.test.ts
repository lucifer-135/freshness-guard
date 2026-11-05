// @vitest-environment node
import { readdirSync, readFileSync } from 'node:fs'
import path from 'node:path'
import { expect, test } from 'vitest'

// React Bits is under MIT + Commons Clause, not Apache-2.0 like the rest of the
// project, so every copied file must say so and the full licence must ship.
const webRoot = path.resolve(import.meta.dirname, '../..')
const dir = path.join(webRoot, 'src/components/reactbits')

test('every React Bits file carries its licence header', () => {
  const files = readdirSync(dir).filter((f) => f.endsWith('.tsx'))
  expect(files.length).toBeGreaterThan(0)
  for (const file of files) {
    const head = readFileSync(path.join(dir, file), 'utf8').slice(0, 600)
    expect(head, file).toContain('MIT + Commons Clause')
    expect(head, file).toContain('Copyright (c) 2026 David Haz')
    expect(head, file).toContain('react-bits-license.md')
  }
})

test('the full React Bits licence ships with the built app', () => {
  expect(readFileSync(path.join(webRoot, 'public/react-bits-license.md'), 'utf8')).toContain('Commons Clause Restriction')
})
