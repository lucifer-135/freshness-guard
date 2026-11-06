import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { TokenGate } from './TokenGate'
import { apiFetch, clearToken, getToken, setToken } from '@/api/client'
import { fakeFetch, json, type FakeFetch } from '@/test/fakeFetch'

const status = { gateway: { reachable: true, models: 9 }, credits: { used: 12, budget: 200 }, settings: {} }

function renderGate() {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <TokenGate>
        <p>Signed in</p>
      </TokenGate>
    </QueryClientProvider>,
  )
}

async function connect(token: string) {
  await userEvent.type(screen.getByLabelText('API token'), token)
  await userEvent.click(screen.getByRole('button', { name: 'Open the chat' }))
}

describe('TokenGate', () => {
  let fake: FakeFetch
  beforeEach(() => clearToken())
  afterEach(() => fake?.restore())

  it('asks for the token when none is stored', () => {
    fake = fakeFetch({})
    renderGate()
    expect(screen.getByRole('heading', { level: 2, name: 'Try it' })).toBeInTheDocument()
  })

  it('opens on the home page, which explains the product before any API call', () => {
    fake = fakeFetch({})
    renderGate()
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(
      'Freshness Guard finds the facts in an LLM answer that may have changed',
    )
    expect(screen.getByRole('heading', { level: 2, name: 'How it works' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { level: 2, name: 'What was measured' })).toBeInTheDocument()
    expect(screen.getByText(/not yet independently reviewed/)).toBeInTheDocument()
    expect(screen.getByText('x-bf-freshness: async | sync | off')).toBeInTheDocument()
    expect(fake.requests).toHaveLength(0)
    expect(document.querySelector('[data-pointer-field]')).not.toBeNull()
  })

  it('lets the reader move the scan line across the example answer with the keyboard', async () => {
    fake = fakeFetch({})
    renderGate()
    const scan = screen.getByRole('slider', { name: 'Scan position' })
    scan.focus()
    await userEvent.keyboard('{Home}')
    expect(scan).toHaveAttribute('aria-valuenow', '0')
    await userEvent.keyboard('{ArrowRight}')
    expect(scan).toHaveAttribute('aria-valuenow', '5')
    await userEvent.keyboard('{End}')
    expect(scan).toHaveAttribute('aria-valuenow', '100')
  })

  it('rejects a token the API does not accept', async () => {
    fake = fakeFetch({ 'GET /freshness/status': () => json(401, { error: 'A valid bearer token is required.' }) })
    renderGate()
    await connect('bad')
    expect(await screen.findByRole('alert')).toHaveTextContent('The token was not accepted. Check the value and try again.')
    expect(getToken()).toBeNull()
  })

  it('explains when the plugin API cannot be reached', async () => {
    fake = fakeFetch({
      'GET /freshness/status': () => {
        throw new TypeError('fetch failed')
      },
    })
    renderGate()
    await connect('any')
    expect(await screen.findByRole('alert')).toHaveTextContent('The plugin API could not be reached. Check that Bifrost is running.')
  })

  it('signs in with a valid token', async () => {
    fake = fakeFetch({ 'GET /freshness/status': () => json(200, status) })
    renderGate()
    await connect('good')
    expect(await screen.findByText('Signed in')).toBeInTheDocument()
    expect(getToken()).toBe('good')
  })

  it('a later 401 returns to sign-in', async () => {
    setToken('old')
    fake = fakeFetch({ 'GET /freshness/status': () => json(401, { error: 'A valid bearer token is required.' }) })
    renderGate()
    expect(screen.getByText('Signed in')).toBeInTheDocument()
    await act(async () => {
      await apiFetch('/freshness/status').catch(() => {})
      await apiFetch('/freshness/status').catch(() => {})
    })
    expect(screen.getByRole('heading', { level: 2, name: 'Try it' })).toBeInTheDocument()
    expect(screen.getAllByText('The token is no longer accepted. Enter the token again.')).toHaveLength(1)
    expect(getToken()).toBeNull()
  })
})
