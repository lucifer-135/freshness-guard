import { FileText, MessageSquare, ShieldCheck } from 'lucide-react'
import { NavLink } from 'react-router'
import { cn } from '@/lib/utils'

// Only pages that exist are listed; the dashboard and the other pages arrive in M3.
const items = [
  { to: '/chat', label: 'Chat', icon: MessageSquare },
  { to: '/reports', label: 'Reports', icon: FileText },
]

// NavRail is a 240px rail at 1024px and wider, and a top menu below that.
export function NavRail() {
  return (
    <nav aria-label="Main" className="border-b border-border bg-surface-1 lg:w-60 lg:shrink-0 lg:border-r lg:border-b-0">
      <p className="flex items-center gap-2 px-4 py-3 font-semibold">
        <ShieldCheck aria-hidden="true" className="size-5 text-primary" />
        Freshness Guard
      </p>
      <ul className="flex gap-1 px-2 pb-2 lg:flex-col">
        {items.map(({ to, label, icon: Icon }) => (
          <li key={to}>
            <NavLink
              to={to}
              className={({ isActive }) =>
                cn(
                  'flex min-h-10 items-center gap-3 rounded-md border-l-2 border-transparent px-3 text-text-muted transition-colors duration-[var(--dur-hover)] hover:bg-accent hover:text-text',
                  isActive && 'border-primary bg-accent-soft font-semibold text-text',
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
