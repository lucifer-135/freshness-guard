import { Navigate, type RouteObject } from 'react-router'
import { NotFound } from './NotFound'
import { AppShell } from '@/shell/AppShell'

// The chat and report pages replace these placeholders in later tasks.
export const routes: RouteObject[] = [
  {
    element: <AppShell />,
    children: [
      { index: true, element: <Navigate to="/chat" replace /> },
      { path: 'chat', element: <h1 className="text-2xl font-semibold">Chat</h1> },
      { path: 'reports', element: <h1 className="text-2xl font-semibold">Reports</h1> },
      { path: 'reports/:id', element: <h1 className="text-2xl font-semibold">Freshness report</h1> },
      { path: '*', element: <NotFound /> },
    ],
  },
]
