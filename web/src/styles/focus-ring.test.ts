// @vitest-environment node
import { readdirSync, readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { expect, it } from 'vitest'

// DESIGN.md section 9: the 2px accent focus ring is never removed. Utilities
// such as outline-none would override the base :focus-visible rule.
it('no shadcn component removes the focus outline', () => {
  for (const folder of ['../components/ui']) {
    const dir = fileURLToPath(new URL(folder, import.meta.url))
    const offenders = readdirSync(dir).filter((f) => /\boutline-(none|hidden)\b/.test(readFileSync(`${dir}/${f}`, 'utf8')))
    expect(offenders, folder).toEqual([])
  }
})
