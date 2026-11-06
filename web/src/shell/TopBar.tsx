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

  return (
    <header className="sticky top-0 z-10 flex flex-wrap items-center gap-x-4 gap-y-0 border-b border-border bg-surface-1 px-4">
      <p className="py-3">
        <Logo />
      </p>
      <NavRail className="order-last w-full md:order-none md:w-auto" />
      <div className="ml-auto flex flex-wrap items-center gap-x-4 gap-y-1 py-2">
        <p className="sr-only text-sm text-text-muted md:not-sr-only">{gateway}</p>
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
    </header>
  )
}
