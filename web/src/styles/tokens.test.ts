// @vitest-environment node
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

// The values of DESIGN.md sections 3.1 to 3.3 and 3.6. The app has one light theme (10 November 2026).
const tokens: [string, string][] = [
  ['--bg', '#FFFFFF'],
  ['--surface-1', '#F8FAFC'],
  ['--surface-2', '#F1F4F8'],
  ['--surface-3', '#E6EAF0'],
  ['--border', '#E2E7EE'],
  ['--border-strong', '#CBD3DD'],
  ['--text', '#0F172A'],
  ['--text-muted', '#475569'],
  ['--text-subtle', '#576475'],
  ['--accent', '#2563EB'],
  ['--accent-strong', '#1D4ED8'],
  ['--accent-soft', '#2563EB14'],
  ['--status-current', '#047857'],
  ['--status-unverified', '#B45309'],
  ['--status-outdated', '#BE123C'],
  ['--status-incorrect', '#A21CAF'],
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
