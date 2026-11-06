import { useSyncExternalStore } from 'react'

// useMediaQuery follows a CSS media query. It is false where matchMedia does
// not exist, so the layout falls back to the narrow one.
export function useMediaQuery(query: string): boolean {
  return useSyncExternalStore(
    (notify) => {
      const list = window.matchMedia?.(query)
      list?.addEventListener?.('change', notify)
      return () => list?.removeEventListener?.('change', notify)
    },
    () => window.matchMedia?.(query).matches ?? false,
    () => false,
  )
}
