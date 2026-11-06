import { describe, expect, it } from 'vitest'
import { formatShortDateTime } from './format'

describe('formatShortDateTime', () => {
  it('writes a time briefly, for narrow tables', () => {
    const value = new Date(2026, 10, 6, 19, 48).toISOString()
    expect(formatShortDateTime(value)).toBe('6 Nov, 19:48')
    expect(formatShortDateTime('nonsense')).toBe('')
  })
})
