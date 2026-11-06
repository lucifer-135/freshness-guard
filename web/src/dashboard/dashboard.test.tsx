import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, fireEvent, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { DashboardPage } from './DashboardPage'
import { setToken } from '@/api/client'
import { feedKey } from '@/live/keys'
import { SpotlightCard } from '@/motion/SpotlightCard'
import { fakeFetch, json, type FakeFetch, type Route as FakeRoute } from '@/test/fakeFetch'
import { completeReport } from '@/test/fixtures'

const summary = {
  window_hours: 24,
  answers: 3,
  checked_claims: 6,
  outdated_claims: 2,
  incorrect_claims: 0,
  cache_purges: 1,
  cache_purge_enabled: true,
  credits: { used: 4, budget: 200 },
}

// Two providers; groq has two models, so a provider's subtotal differs from its models.
const leaderboard = {
  window_days: 7,
  since: '2026-10-29T00:00:00Z',
  min_sample: 10,
  rows: [
    {
      model: 'gemini/gemini-3.5-flash',
      provider: 'gemini',
      total: { checked: 50, outdated: 5, incorrect: 0 },
      cells: { pricing: { checked: 50, outdated: 5, incorrect: 0 } },
    },
    {
      model: 'groq/openai/gpt-oss-20b',
      provider: 'groq',
      total: { checked: 122, outdated: 41, incorrect: 1 },
      cells: { pricing: { checked: 118, outdated: 40, incorrect: 0 }, law: { checked: 4, outdated: 1, incorrect: 1 } },
    },
    {
      model: 'groq/openai/gpt-oss-120b',
      provider: 'groq',
      total: { checked: 12, outdated: 3, incorrect: 0 },
      cells: { pricing: { checked: 12, outdated: 3, incorrect: 0 } },
    },
  ],
}

const pricingSentence = 'Outdated claims on pricing: 34% of 118 checked claims.'
const lawSentence =
  'Outdated claims on law: 25% of 4 checked claims. This value is based on fewer than 10 claims and has low confidence.'

function setup(routes: Record<string, FakeRoute> = {}, queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })) {
  const fake = fakeFetch({
    'GET /freshness/summary': () => json(200, summary),
    'GET /freshness/leaderboard': () => json(200, leaderboard),
    'GET /freshness/reports': () => json(200, { reports: [] }),
    ...routes,
  })
  render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter basename="/freshness/ui" initialEntries={['/freshness/ui/dashboard']}>
        <Routes>
          <Route path="/dashboard" element={<DashboardPage />} />
          <Route path="/reports/:id" element={<p>report page</p>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
  return { fake, queryClient }
}

function matchMediaWith(matching: (query: string) => boolean) {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    configurable: true,
    value: (query: string): MediaQueryList =>
      ({
        matches: matching(query),
        media: query,
        addEventListener: () => {},
        removeEventListener: () => {},
      }) as unknown as MediaQueryList,
  })
}

// Tests that change matchMedia must not leak the change into the next test.
afterEach(() => matchMediaWith(() => false))

describe('DashboardPage summary row', () => {
  let fake: FakeFetch
  beforeEach(() => setToken('t1'))
  afterEach(() => fake?.restore())

  it('shows the four summary cards', async () => {
    ;({ fake } = setup())
    const card = async (label: string) => (await screen.findByText(label)).parentElement as HTMLElement
    expect(await card('Answers checked in the last 24 hours')).toHaveTextContent('3')
    const share = await card('Share of checked claims that were outdated')
    expect(share).toHaveTextContent('33%')
    expect(share).toHaveTextContent('of 6 checked claims')
    expect(await card('Tavily credits used today')).toHaveTextContent('4 of 200')
    expect(await card('Cached answers removed in the last 24 hours')).toHaveTextContent('1')
  })

  it('writes card labels in sentence case, not as eyebrows', async () => {
    ;({ fake } = setup())
    const label = await screen.findByText('Answers checked in the last 24 hours')
    expect(label).not.toHaveClass('eyebrow')
    expect(label.nextElementSibling).not.toHaveClass('font-mono')
  })

  it('shows credit use as a meter', async () => {
    ;({ fake } = setup())
    const meter = await screen.findByRole('meter', { name: 'Tavily credits used today' })
    expect(meter).toHaveAttribute('aria-valuenow', '4')
    expect(meter).toHaveAttribute('aria-valuemin', '0')
    expect(meter).toHaveAttribute('aria-valuemax', '200')
  })

  it('shows an empty meter when the budget is 0', async () => {
    ;({ fake } = setup({ 'GET /freshness/summary': () => json(200, { ...summary, credits: { used: 0, budget: 0 } }) }))
    const meter = await screen.findByRole('meter', { name: 'Tavily credits used today' })
    expect(meter).toHaveAttribute('aria-valuemax', '0')
    expect((meter.firstElementChild as HTMLElement).style.width).toBe('0%')
  })

  it('says when the cache purge is switched off', async () => {
    ;({ fake } = setup({ 'GET /freshness/summary': () => json(200, { ...summary, cache_purge_enabled: false }) }))
    expect(await screen.findByText('Cache purge is switched off.')).toBeInTheDocument()
  })

  it('says when the summary cannot be loaded', async () => {
    ;({ fake } = setup({ 'GET /freshness/summary': () => json(500, { error: 'The summary could not be loaded.' }) }))
    expect(await screen.findByText('The summary could not be loaded.')).toBeInTheDocument()
  })
})

describe('DashboardPage heatmap', () => {
  let fake: FakeFetch
  beforeEach(() => setToken('t1'))
  afterEach(() => fake?.restore())

  it('shows the rates as a table grouped by provider with a value in every cell', async () => {
    ;({ fake } = setup())
    const table = await screen.findByRole('table', { name: 'Claim rates by model and category' })
    const headers = within(table).getAllByRole('columnheader').map((h) => h.textContent)
    expect(headers).toEqual(['Model', 'pricing', 'law'])
    expect(within(table).getAllByRole('rowheader').map((h) => h.textContent)).toEqual([
      'All gemini models',
      'gemini/gemini-3.5-flash',
      'All groq models',
      'groq/openai/gpt-oss-20b',
      'groq/openai/gpt-oss-120b',
    ])
    const cell = within(table).getByLabelText(pricingSentence)
    expect(cell).toHaveTextContent('34%')
    // The groq subtotal adds both models: 43 of 130.
    expect(within(table).getByLabelText('Outdated claims on pricing: 33% of 130 checked claims.')).toBeInTheDocument()
  })

  it('marks values with few claims as low sample and says when there is no data', async () => {
    ;({ fake } = setup())
    // The groq subtotal and its only model with law data show the same numbers.
    const lawCells = await screen.findAllByLabelText(lawSentence)
    expect(lawCells).toHaveLength(2)
    for (const cell of lawCells) {
      expect(cell).toHaveAttribute('data-low-sample', 'true')
      expect(cell).toHaveTextContent('Low sample')
    }
    expect(screen.getAllByText('No data').length).toBeGreaterThan(0)
    expect(screen.getByLabelText(pricingSentence)).not.toHaveAttribute('data-low-sample', 'true')
  })

  it('colours cells in oklab so a high rate never turns violet', async () => {
    ;({ fake } = setup())
    const cell = await screen.findByLabelText(pricingSentence)
    expect(cell.style.backgroundColor).toContain('in oklab')
    expect(cell.style.backgroundColor).not.toContain('in oklch')
  })

  it('hatches low-sample cells', async () => {
    ;({ fake } = setup())
    for (const cell of await screen.findAllByLabelText(lawSentence)) {
      expect(cell.style.backgroundImage).toContain('repeating-linear-gradient')
    }
  })

  it('uses segmented controls for the rate and the time window', async () => {
    ;({ fake } = setup())
    await screen.findByLabelText(pricingSentence)
    expect(screen.getByRole('radiogroup', { name: 'Show the rate of' })).toBeInTheDocument()
    expect(screen.getByRole('radiogroup', { name: 'Time window' })).toBeInTheDocument()
  })

  it('switches between the outdated and the incorrect rate', async () => {
    ;({ fake } = setup())
    const user = userEvent.setup()
    await screen.findByLabelText(pricingSentence)
    await user.click(screen.getByRole('radio', { name: 'Incorrect claims' }))
    const cells = await screen.findAllByLabelText(
      'Incorrect claims on law: 25% of 4 checked claims. This value is based on fewer than 10 claims and has low confidence.',
    )
    expect(cells).toHaveLength(2)
  })

  it('loads the 30-day window when asked', async () => {
    ;({ fake } = setup())
    const user = userEvent.setup()
    await screen.findByLabelText(pricingSentence)
    await user.click(screen.getByRole('radio', { name: '30 days' }))
    await screen.findByLabelText(pricingSentence)
    const windows = fake.requests.filter((r) => r.path === '/freshness/leaderboard').map((r) => r.url.searchParams.get('window'))
    expect(windows).toEqual(['7d', '30d'])
  })

  it('shows the exact value of the focused cell below the table', async () => {
    ;({ fake } = setup())
    const cell = await screen.findByLabelText(pricingSentence)
    expect(screen.queryByText(pricingSentence)).not.toBeInTheDocument()
    act(() => cell.focus())
    expect(screen.getByText(pricingSentence)).toBeInTheDocument()
    expect(cell).toHaveAttribute('data-active', 'true')
  })

  it('offers the same numbers as a plain table', async () => {
    ;({ fake } = setup())
    const user = userEvent.setup()
    await screen.findByLabelText(pricingSentence)
    await user.click(screen.getByRole('button', { name: 'View as table' }))
    const table = screen.getByRole('table', { name: 'Claim counts by model and category' })
    expect(within(table).getAllByRole('columnheader').map((h) => h.textContent)).toEqual([
      'Model',
      'Category',
      'Checked claims',
      'Outdated',
      'Incorrect',
    ])
    // One row per model and category with data: 1 + 2 + 1, plus the header row.
    expect(within(table).getAllByRole('row')).toHaveLength(5)
  })

  it('explains an empty leaderboard', async () => {
    ;({ fake } = setup({ 'GET /freshness/leaderboard': () => json(200, { ...leaderboard, rows: [] }) }))
    expect(await screen.findByText('No claims have been checked yet. Send a message in the chat to see results here.')).toBeInTheDocument()
    expect(screen.queryByRole('table', { name: 'Claim rates by model and category' })).not.toBeInTheDocument()
  })

  it('offers a retry when the leaderboard cannot be loaded', async () => {
    let calls = 0
    ;({ fake } = setup({
      'GET /freshness/leaderboard': () => {
        calls++
        return calls === 1 ? json(500, { error: 'boom' }) : json(200, leaderboard)
      },
    }))
    const user = userEvent.setup()
    expect(await screen.findByText('The leaderboard could not be loaded.')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Try again' }))
    expect(await screen.findByLabelText(pricingSentence)).toBeInTheDocument()
  })
})

describe('DashboardPage live feed', () => {
  let fake: FakeFetch
  beforeEach(() => setToken('t1'))
  afterEach(() => fake?.restore())

  const reports = (...ids: string[]) => ids.map((id) => completeReport({ id, model: 'openai/fake-model' }))

  it('lists recent reports newest first with links', async () => {
    ;({ fake } = setup({ 'GET /freshness/reports': () => json(200, { reports: reports('r2', 'r1') }) }))
    await screen.findAllByRole('link', { name: /openai\/fake-model/ })
    const links = within(screen.getByRole('list', { name: 'Recent reports' })).getAllByRole('link')
    expect(links.map((l) => l.getAttribute('href'))).toEqual(['/freshness/ui/reports/r2', '/freshness/ui/reports/r1'])
    expect(links[0]).toHaveTextContent('openai/fake-model')
    expect(fake.requests.find((r) => r.path === '/freshness/reports')?.url.searchParams.get('limit')).toBe('50')
  })

  it('freezes the feed while it is paused and keeps focus on the pause button', async () => {
    let list = reports('r1')
    const { queryClient, fake: f } = setup({ 'GET /freshness/reports': () => json(200, { reports: list }) })
    fake = f
    const user = userEvent.setup()
    await screen.findAllByRole('link', { name: /openai\/fake-model/ })
    const pause = screen.getByRole('button', { name: 'Pause live feed' })
    await user.click(pause)
    expect(pause).toHaveAttribute('aria-pressed', 'true')
    // The status line exists from the moment of pausing, so a new report never shifts the list.
    const region = screen.getByRole('status')
    expect(region).toBeEmptyDOMElement()

    list = reports('r2', 'r1')
    await act(() => queryClient.invalidateQueries({ queryKey: feedKey }))
    expect(screen.getAllByRole('link', { name: /openai\/fake-model/ })).toHaveLength(1)
    expect(await screen.findByText('1 new report is waiting.')).toBeInTheDocument()
    expect(screen.getByRole('status')).toBe(region)
    expect(screen.getAllByRole('link', { name: /openai\/fake-model/ })).toHaveLength(1)
    expect(pause).toHaveFocus()

    await user.click(pause)
    expect(pause).toHaveAttribute('aria-pressed', 'false')
    expect(await screen.findAllByRole('link', { name: /openai\/fake-model/ })).toHaveLength(2)
  })

  it('slides a new report in', async () => {
    let list = reports('r1')
    const { queryClient, fake: f } = setup({ 'GET /freshness/reports': () => json(200, { reports: list }) })
    fake = f
    await screen.findAllByRole('link', { name: /openai\/fake-model/ })
    list = reports('r2', 'r1')
    await act(() => queryClient.invalidateQueries({ queryKey: feedKey }))
    const items = screen.getAllByRole('listitem')
    expect(items[0].style.transform).toContain('translateY')
  })

  it('shows a new report without movement when reduced motion is on', async () => {
    matchMediaWith((q) => q.includes('prefers-reduced-motion'))
    let list = reports('r1')
    const { queryClient, fake: f } = setup({ 'GET /freshness/reports': () => json(200, { reports: list }) })
    fake = f
    await screen.findAllByRole('link', { name: /openai\/fake-model/ })
    list = reports('r2', 'r1')
    await act(() => queryClient.invalidateQueries({ queryKey: feedKey }))
    expect(screen.getAllByRole('listitem')[0].style.transform).toBe('')
  })
})

describe('SpotlightCard', () => {
  it('lights up under a fine pointer', () => {
    matchMediaWith(() => false)
    render(<SpotlightCard>content</SpotlightCard>)
    const card = screen.getByText('content')
    fireEvent.pointerEnter(card)
    expect(card.style.getPropertyValue('--spot')).toBe('1')
    fireEvent.pointerLeave(card)
    expect(card.style.getPropertyValue('--spot')).toBe('0')
  })

  it('does nothing with reduced motion or a touch pointer', () => {
    matchMediaWith(() => true)
    render(<SpotlightCard>content</SpotlightCard>)
    const card = screen.getByText('content')
    fireEvent.pointerEnter(card)
    expect(card.style.getPropertyValue('--spot')).toBe('')
  })
})
