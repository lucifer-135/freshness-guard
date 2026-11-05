import { readFileSync } from 'node:fs'
import path from 'node:path'
import { expect, test } from 'vitest'

// DESIGN.md section 3.2: colour means verdict, so the accent must stay readable
// and must never look like a status colour.
const css = readFileSync(path.resolve(import.meta.dirname, './index.css'), 'utf8')
const rootBlock = /:root\s*\{([\s\S]*?)\n\}/.exec(css)?.[1] ?? ''
const tokens: Record<string, string> = {}
for (const [, name, value] of rootBlock.matchAll(/(--[\w-]+):\s*(#[0-9a-fA-F]{6,8})\s*;/g)) tokens[name] = value

function rgb(hex: string): [number, number, number] {
  return [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16) / 255) as [number, number, number]
}

function luminance(hex: string): number {
  const [r, g, b] = rgb(hex).map((c) => (c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4))
  return 0.2126 * r + 0.7152 * g + 0.0722 * b
}

function contrast(a: string, b: string): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x)
  return (hi + 0.05) / (lo + 0.05)
}

function hsl(hex: string): { h: number; s: number } {
  const [r, g, b] = rgb(hex)
  const max = Math.max(r, g, b)
  const min = Math.min(r, g, b)
  const d = max - min
  if (d === 0) return { h: 0, s: 0 }
  const l = (max + min) / 2
  const s = d / (1 - Math.abs(2 * l - 1))
  const h = max === r ? ((g - b) / d) % 6 : max === g ? (b - r) / d + 2 : (r - g) / d + 4
  return { h: (h * 60 + 360) % 360, s }
}

function hueDistance(a: number, b: number): number {
  const d = Math.abs(a - b)
  return Math.min(d, 360 - d)
}

test('the accent has at least 4.5:1 contrast on the page background', () => {
  expect(contrast(tokens['--accent'], tokens['--bg'])).toBeGreaterThanOrEqual(4.5)
})

test('the accent hue is at least 50 degrees from every saturated status colour', () => {
  for (const name of ['--status-current', '--status-unverified', '--status-outdated', '--status-incorrect']) {
    if (hsl(tokens[name]).s < 0.3) continue
    expect(hueDistance(hsl(tokens['--accent']).h, hsl(tokens[name]).h), name).toBeGreaterThanOrEqual(50)
  }
})

test('status colours keep 3:1 contrast on the page background', () => {
  for (const name of ['--status-current', '--status-unverified', '--status-outdated', '--status-incorrect', '--status-unchecked']) {
    expect(contrast(tokens[name], tokens['--bg']), name).toBeGreaterThanOrEqual(3)
  }
})
