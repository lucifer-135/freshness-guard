import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createMemoryRouter } from 'react-router'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { AppProviders } from '@/App'
import { routes } from '@/app/routes'
import { clearToken, getToken, setToken } from '@/api/client'
import { fakeFetch, json, streamBody, type FakeFetch, type Route } from '@/test/fakeFetch'

function status(reachable: boolean, models: number) {
  return json(200, {
    gateway: { reachable, models },
    credits: { used: 12, budget: 200 },
    settings: { default_mode: 'async', default_policy: 'heal', sync_timeout_ms: 8000 },
  })
}

// The event stream ends at once, so the shell shows that it is reconnecting.
const endedStream: Route = () => new Response(streamBody([]), { headers: { 'Content-Type': 'text/event-stream' } })

function renderAt(path: string) {
  return render(<AppProviders router={createMemoryRouter(routes, { initialEntries: [path] })} />)
}

describe('AppShell', () => {
  let fake: FakeFetch
  beforeEach(() => {
    clearToken()
    setToken('t1')
  })
  afterEach(() => fake?.restore())

  it('starts with a skip link and marks the current page', async () => {
    fake = fakeFetch({ 'GET /freshness/status': () => status(true, 9), 'GET /freshness/events': endedStream })
    renderAt('/chat')
    await userEvent.tab()
    const skip = screen.getByRole('link', { name: 'Skip to main content' })
    expect(skip).toHaveFocus()
    expect(skip).toHaveAttribute('href', '#main')
    expect(screen.getByRole('link', { name: 'Chat' })).toHaveAttribute('aria-current', 'page')
    expect(screen.getByRole('link', { name: 'Reports' })).not.toHaveAttribute('aria-current')
  })

  it('puts the main navigation inside the page header', async () => {
    fake = fakeFetch({ 'GET /freshness/status': () => status(true, 9), 'GET /freshness/events': endedStream })
    renderAt('/chat')
    const nav = within(screen.getByRole('banner')).getByRole('navigation', { name: 'Main' })
    for (const name of ['Chat', 'Dashboard', 'Reports']) {
      expect(within(nav).getByRole('link', { name })).toBeInTheDocument()
    }
  })

  // The developer asked on 7 November 2026 for a ground that answers the mouse; it is CSS, not a canvas.
  it('draws the pointer field behind the content, without a canvas', () => {
    fake = fakeFetch({ 'GET /freshness/status': () => status(true, 9), 'GET /freshness/events': endedStream })
    renderAt('/chat')
    expect(document.querySelector('[data-pointer-field]')).not.toBeNull()
    expect(document.querySelector('canvas')).toBeNull()
    // Every page shares one frame, and the field lives in the margins outside it, never under text.
    expect(screen.getByRole('main').firstElementChild).toHaveClass('mx-auto', 'max-w-[1200px]')
    expect(screen.getByRole('banner').firstElementChild).toHaveClass('mx-auto', 'max-w-[1200px]')
  })

  it('draws no cursor aura', () => {
    fake = fakeFetch({ 'GET /freshness/status': () => status(true, 9), 'GET /freshness/events': endedStream })
    renderAt('/chat')
    expect(document.querySelector('[data-aura]')).toBeNull()
  })

  it('shows gateway status, credits and the live connection', async () => {
    fake = fakeFetch({ 'GET /freshness/status': () => status(true, 9), 'GET /freshness/events': endedStream })
    renderAt('/chat')
    // A healthy gateway is said to screen readers only; the top bar keeps to one row at 1440px.
    expect(await screen.findByText('Connected to Bifrost, 9 models available.')).toHaveClass('sr-only')
    expect(screen.getByRole('img', { name: 'Tavily credits today: 12 of 200' })).toBeInTheDocument()
    // Only the figure is mono, and the meter fill is neutral: blue is reserved for focus, links and selection.
    expect(screen.getByText('12 of 200')).toHaveClass('font-mono')
    expect(screen.getByRole('img', { name: 'Tavily credits today: 12 of 200' }).innerHTML).not.toContain('--accent')
    expect(await screen.findByText('Reconnecting')).toBeInTheDocument()
  })

  it('says when Bifrost cannot be reached', async () => {
    fake = fakeFetch({ 'GET /freshness/status': () => status(false, 0), 'GET /freshness/events': endedStream })
    renderAt('/chat')
    expect(await screen.findByText('Bifrost could not be reached.')).not.toHaveClass('sr-only')
  })

  it('signs out', async () => {
    fake = fakeFetch({ 'GET /freshness/status': () => status(true, 9), 'GET /freshness/events': endedStream })
    renderAt('/chat')
    await userEvent.click(screen.getByRole('button', { name: 'Sign out' }))
    expect(getToken()).toBeNull()
    expect(screen.getByRole('heading', { level: 2, name: 'Try it' })).toBeInTheDocument()
  })

  it('shows a not-found page', () => {
    fake = fakeFetch({ 'GET /freshness/status': () => status(true, 9), 'GET /freshness/events': endedStream })
    renderAt('/nowhere')
    expect(screen.getByText('This page does not exist.')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Go to the chat' })).toBeInTheDocument()
  })
})
