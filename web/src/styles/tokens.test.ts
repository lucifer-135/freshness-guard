// @vitest-environment node
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

// The values of DESIGN.md sections 3.1 to 3.3 and 3.6 (revised 6 November 2026).
const tokens: [string, string][] = [
  ['--bg', '#07090D'],
  ['--surface-1', '#0D1117'],
  ['--surface-2', '#131923'],
  ['--surface-3', '#1A2230'],
  ['--border', '#263041'],
  ['--border-strong', '#3A475C'],
  ['--text', '#E8EDF5'],
  ['--text-muted', '#94A3B8'],
  ['--text-subtle', '#7C8BA1'],
  ['--accent', '#60A5FA'],
  ['--accent-strong', '#93C5FD'],
  ['--accent-soft', '#60A5FA1F'],
  ['--status-current', '#34D399'],
  ['--status-unverified', '#FBBF24'],
  ['--status-outdated', '#FB7185'],
  ['--status-incorrect', '#E879F9'],
  ['--status-unchecked', '#64748B'],
  ['--dur-press', '80ms'],
  ['--dur-hover', '150ms'],
  ['--dur-enter', '220ms'],
  ['--dur-panel', '240ms'],
  ['--ease-enter', 'cubic-bezier(0.16, 1, 0.3, 1)'],
  ['--ease-exit', 'cubic-bezier(0.23, 1, 0.32, 1)'],
]

describe('design tokens', () => {
  const css = readFileSync(new URL('./index.css', import.meta.url), 'utf8')

  it.each(tokens)('defines %s as %s', (name, value) => {
    expect(css).toContain(`${name}: ${value};`)
  })
})
