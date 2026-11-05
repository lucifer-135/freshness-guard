import { Navigate, type RouteObject } from 'react-router'
import { NotFound } from './NotFound'
import { ChatPage } from '@/chat/ChatPage'
import { AppShell } from '@/shell/AppShell'

// The report pages replace these placeholders in Task 11.
export const routes: RouteObject[] = [
  {
    element: <AppShell />,
    children: [
      { index: true, element: <Navigate to="/chat" replace /> },
      { path: 'chat', element: <ChatPage /> },
      { path: 'reports', element: <h1 className="text-2xl font-semibold">Reports</h1> },
      { path: 'reports/:id', element: <h1 className="text-2xl font-semibold">Freshness report</h1> },
      { path: '*', element: <NotFound /> },
    ],
  },
]
