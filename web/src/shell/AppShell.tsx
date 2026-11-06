import { useEffect, useRef } from 'react'
import { Outlet, useLocation } from 'react-router'
import { ChatProvider } from '@/chat/ChatProvider'
import { PointerField } from './PointerField'
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
    <div className="relative isolate flex min-h-dvh flex-col text-text">
      <PointerField />
      <a
        href="#main"
        className="sr-only z-[100] rounded-md bg-surface-3 px-3 py-2 focus:not-sr-only focus:fixed focus:top-2 focus:left-2"
      >
        Skip to main content
      </a>
      <TopBar />
      <main id="main" ref={main} tabIndex={-1} className="min-w-0 flex-1 p-4 outline-none lg:p-6">
        {/* One frame for every page; the pointer field lives in the margins outside it. */}
        <div className="mx-auto max-w-[1200px]">
          <ChatProvider>
            <Outlet />
          </ChatProvider>
        </div>
      </main>
    </div>
  )
}
