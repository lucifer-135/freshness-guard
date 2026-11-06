import { FileText, LayoutDashboard, MessageSquare } from 'lucide-react'
import { NavLink } from 'react-router'
import { cn } from '@/lib/utils'

// Only pages that exist are listed; the facts, watchlist and settings pages are not built yet.
const items = [
  { to: '/chat', label: 'Chat', icon: MessageSquare },
  { to: '/dashboard', label: 'Dashboard', icon: LayoutDashboard },
  { to: '/reports', label: 'Reports', icon: FileText },
]

// NavRail is the page links in the top bar (DESIGN.md section 6). The current
// page has a 2px accent underline and aria-current="page".
export function NavRail({ className }: { className?: string }) {
  return (
    <nav aria-label="Main" className={className}>
      <ul className="flex items-center gap-1">
        {items.map(({ to, label, icon: Icon }) => (
          <li key={to}>
            <NavLink
              to={to}
              className={({ isActive }) =>
                cn(
                  'flex h-11 items-center gap-2 border-b-2 border-transparent px-3 text-text-muted transition-colors duration-[var(--dur-hover)] hover:text-text',
                  isActive && 'border-[var(--accent)] font-medium text-text',
                )
              }
            >
              <Icon aria-hidden="true" className="size-4" />
              {label}
            </NavLink>
          </li>
        ))}
      </ul>
    </nav>
  )
}
