import { useSyncExternalStore } from 'react'

const reduced = '(prefers-reduced-motion: reduce)'
const coarse = '(pointer: coarse)'

function subscribe(onChange: () => void) {
  const lists = [window.matchMedia(reduced), window.matchMedia(coarse)]
  lists.forEach((l) => l.addEventListener('change', onChange))
  return () => lists.forEach((l) => l.removeEventListener('change', onChange))
}

// Cursor effects run only for a fine pointer without reduced motion
// (DESIGN.md section 4): touch devices and reduced-motion users get none.
export function useFineMotion(): boolean {
  return useSyncExternalStore(subscribe, () => !window.matchMedia(reduced).matches && !window.matchMedia(coarse).matches)
}
