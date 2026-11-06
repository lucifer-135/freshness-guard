import type { Mode, Policy } from '@/api/types'

export const modeOptions: { value: Mode; label: string }[] = [
  { value: 'async', label: 'Background' },
  { value: 'sync', label: 'Before reply' },
  { value: 'off', label: 'Off' },
]

export const policyOptions: { value: Policy; label: string }[] = [
  { value: 'annotate', label: 'Warn only' },
  { value: 'heal', label: 'Correct' },
  { value: 'block', label: 'Block' },
]

// modeHint is the one sentence under the composer that says what the chosen
// verification mode does (DESIGN.md section 7.1).
export function modeHint(mode: Mode, syncTimeoutMs: number): string {
  switch (mode) {
    case 'async':
      return 'Claims are checked after the answer is shown.'
    case 'sync':
      return `The answer waits up to ${Math.round(syncTimeoutMs / 1000)} seconds while its claims are checked.`
    case 'off':
      return 'This answer will not be checked.'
  }
}
