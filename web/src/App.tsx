import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { useState } from 'react'
import { createBrowserRouter, RouterProvider, type DataRouter } from 'react-router'
import { ApiError } from '@/api/client'
import { routes } from '@/app/routes'
import { TokenGate } from '@/auth/TokenGate'
import { EventStreamProvider } from '@/live/EventStream'

function newQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        // A rejected token signs the user out; retrying would only repeat the 401.
        retry: (failures, error) => !(error instanceof ApiError && error.status === 401) && failures < 2,
      },
    },
  })
}

export function AppProviders({ router }: { router: DataRouter }) {
  const [queryClient] = useState(newQueryClient)
  return (
    <QueryClientProvider client={queryClient}>
      <TokenGate>
        <EventStreamProvider>
          <RouterProvider router={router} />
        </EventStreamProvider>
      </TokenGate>
    </QueryClientProvider>
  )
}

const router = createBrowserRouter(routes, { basename: import.meta.env.BASE_URL })

export default function App() {
  return <AppProviders router={router} />
}
