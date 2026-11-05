import { Navigate, type RouteObject } from 'react-router'
import { NotFound } from './NotFound'
import { ChatPage } from '@/chat/ChatPage'
import { AppShell } from '@/shell/AppShell'

export const routes: RouteObject[] = [
  {
    element: <AppShell />,
    children: [
      { index: true, element: <Navigate to="/chat" replace /> },
      { path: 'chat', element: <ChatPage /> },
      // The dashboard and the report pages are split into their own chunks (DESIGN.md section 10).
      { path: 'dashboard', lazy: async () => ({ Component: (await import('@/dashboard/DashboardPage')).DashboardPage }) },
      { path: 'reports', lazy: async () => ({ Component: (await import('@/reports/ReportsPage')).ReportsPage }) },
      { path: 'reports/:id', lazy: async () => ({ Component: (await import('@/reports/ReportPage')).ReportPage }) },
      { path: '*', element: <NotFound /> },
    ],
  },
]
