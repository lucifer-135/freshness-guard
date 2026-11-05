import { Link } from 'react-router'

export function NotFound() {
  return (
    <section className="space-y-3">
      <h1 className="text-2xl font-semibold">This page does not exist.</h1>
      <Link to="/chat" className="text-link underline underline-offset-4 hover:text-link-strong">
        Go to the chat
      </Link>
    </section>
  )
}
