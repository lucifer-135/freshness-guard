import { describe, expect, it } from 'vitest'
import { modeHint, modeOptions, policyOptions } from './modes'

describe('modeHint', () => {
  it('says that background checking happens after the answer', () => {
    expect(modeHint('async', 8000)).toBe('Claims are checked after the answer is shown.')
  })

  it('gives the sync timeout in whole seconds', () => {
    expect(modeHint('sync', 8000)).toBe('The answer waits up to 8 seconds while its claims are checked.')
    expect(modeHint('sync', 12500)).toBe('The answer waits up to 13 seconds while its claims are checked.')
  })

  it('says that nothing is checked when checking is off', () => {
    expect(modeHint('off', 8000)).toBe('This answer will not be checked.')
  })
})

describe('options', () => {
  it('uses short, plain labels', () => {
    expect(modeOptions.map((o) => o.label)).toEqual(['Background', 'Before reply', 'Off'])
    expect(policyOptions.map((o) => o.label)).toEqual(['Warn only', 'Correct', 'Block'])
  })
})
