import { useEffect, useRef } from 'react'
import { Outlet, useLocation } from 'react-router'
import { NavRail } from './NavRail'
import { CursorAura } from '@/motion/CursorAura'
import { SignalField } from '@/motion/SignalField'
import { TopBar } from './TopBar'

export function AppShell() {
  const main = useRef<HTMLElement>(null)
  const { pathname } = useLocation()
  const firstRender = useRef(true)

  // After navigation, screen-reader users land on the new page's content.
  useEffect(() => {
    if (firstRender.current) {
      firstRender.current = false
      return
    }
    main.current?.focus()
  }, [pathname])

  return (
    <div className="relative isolate flex min-h-dvh flex-col text-text lg:flex-row">
      <a
        href="#main"
        className="sr-only z-[100] rounded-md bg-surface-3 px-3 py-2 focus:not-sr-only focus:fixed focus:top-2 focus:left-2"
      >
        Skip to main content
      </a>
      <SignalField />
      <CursorAura />
      <NavRail />
      <div className="flex min-w-0 flex-1 flex-col">
        <TopBar />
        <main id="main" ref={main} tabIndex={-1} className="flex-1 p-4 outline-none lg:p-6">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
