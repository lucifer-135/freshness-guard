import { useQuery, type UseQueryResult } from '@tanstack/react-query'
import { LogOut, Radio } from 'lucide-react'
import { Logo } from '@/brand/Logo'
import { CreditMeter } from './CreditMeter'
import { NavRail } from './NavRail'
import { getStatus, type ApiError } from '@/api/client'
import type { PluginStatus } from '@/api/types'
import { useSignOut } from '@/auth/TokenGate'
import { Button } from '@/components/ui/button'
import { useConnection, type Connection } from '@/live/EventStream'

// eslint-disable-next-line react-refresh/only-export-components -- shared by the top bar and the composer
export function useStatus(): UseQueryResult<PluginStatus, ApiError> {
  return useQuery<PluginStatus, ApiError>({ queryKey: ['status'], queryFn: getStatus, refetchInterval: 30000 })
}

const connectionLabel: Record<Connection, string> = { live: 'Live', connecting: 'Connecting', reconnecting: 'Reconnecting' }

export function TopBar() {
  const status = useStatus()
  const connection = useConnection()
  const signOut = useSignOut()

  let gateway = 'Checking the connection to Bifrost.'
  if (status.data) {
    gateway = status.data.gateway.reachable
      ? `Connected to Bifrost, ${status.data.gateway.models} models available.`
      : 'Bifrost could not be reached.'
  } else if (status.isError) {
    gateway = 'The plugin status could not be loaded.'
  }
  // A healthy gateway needs no words on screen, so the bar stays on one row; a problem is always shown.
  const healthy = status.data?.gateway.reachable === true || (!status.data && !status.isError)

  return (
    <header className="sticky top-0 z-10 border-b border-border bg-surface-1 px-4 lg:px-6">
      {/* The bar spans the window; its contents sit in the same 1200px frame as every page. */}
      <div className="mx-auto flex max-w-[1200px] flex-wrap items-center gap-x-4 gap-y-0">
      <p className="py-3">
        <Logo />
      </p>
      <NavRail className="order-last w-full md:order-none md:w-auto" />
      <div className="ml-auto flex flex-wrap items-center gap-x-4 gap-y-1 py-2">
        <p className={healthy ? 'sr-only' : 'text-sm text-status-unverified'}>{gateway}</p>
        {status.data && <CreditMeter used={status.data.credits.used} budget={status.data.credits.budget} />}
        <p
          className="flex items-center gap-1.5 text-sm text-text-muted"
          title={connection === 'reconnecting' ? 'The live connection was interrupted. The page will reconnect automatically.' : undefined}
        >
          <Radio
            aria-hidden="true"
            className={connection === 'live' ? 'size-4 text-status-current' : 'size-4 text-status-unverified'}
          />
          {connectionLabel[connection]}
        </p>
        <Button variant="ghost" size="sm" onClick={signOut}>
          <LogOut aria-hidden="true" />
          Sign out
        </Button>
      </div>
      </div>
    </header>
  )
}
