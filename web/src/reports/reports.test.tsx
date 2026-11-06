import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, within } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { ReportPage } from './ReportPage'
import { ReportsPage } from './ReportsPage'
import { setToken } from '@/api/client'
import { fakeFetch, json, type FakeFetch, type Route as FakeRoute } from '@/test/fakeFetch'
import { answer, completeReport, freshClaim } from '@/test/fixtures'

function renderAt(path: string, routes: Record<string, FakeRoute>) {
  const fake = fakeFetch(routes)
  render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <MemoryRouter basename="/freshness/ui" initialEntries={[`/freshness/ui${path}`]}>
        <Routes>
          <Route path="/reports" element={<ReportsPage />} />
          <Route path="/reports/:id" element={<ReportPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
  return fake
}

describe('ReportsPage', () => {
  let fake: FakeFetch
  beforeEach(() => setToken('t1'))
  afterEach(() => fake?.restore())

  it('lists reports with links to each one', async () => {
    fake = renderAt('/reports', {
      'GET /freshness/reports': () => json(200, { reports: [completeReport(), completeReport({ id: 'r2', score: null })] }),
    })
    const links = await screen.findAllByRole('link')
    expect(links.map((l) => l.getAttribute('href'))).toEqual(['/freshness/ui/reports/r1', '/freshness/ui/reports/r2'])
    expect(screen.getByRole('columnheader', { name: 'Outdated' })).toBeInTheDocument()
    expect(fake.requests[0].url.searchParams.get('limit')).toBe('50')
    expect(screen.getAllByText('Background').length).toBeGreaterThan(0)
    expect(screen.getByRole('table').parentElement).not.toHaveClass('rounded-xl')
  })

  it('shows each score as a number, not a ring, and keeps the date link quiet', async () => {
    fake = renderAt('/reports', { 'GET /freshness/reports': () => json(200, { reports: [completeReport()] }) })
    const [link] = await screen.findAllByRole('link')
    expect(link).not.toHaveClass('underline')
    const row = link.closest('tr')!
    expect(row.querySelector('svg')).toBeNull()
    expect(within(row).getByLabelText(/^Freshness score \d+ out of 100$/)).toHaveClass('font-mono')
    // Mode and status give way to the figures on narrow screens.
    expect(screen.getByRole('columnheader', { name: 'Mode' })).toHaveClass('hidden', 'sm:table-cell')
    for (const name of ['Model', 'Checked']) expect(screen.getByRole('columnheader', { name })).toHaveClass('hidden', 'sm:table-cell')
    expect(link.querySelector('[class~="sm:hidden"]')).not.toBeNull()
  })

  it('explains an empty list', async () => {
    fake = renderAt('/reports', { 'GET /freshness/reports': () => json(200, { reports: null }) })
    expect(
      await screen.findByText('No answers have been checked yet. Send a message in the chat to see results here.'),
    ).toBeInTheDocument()
  })
})

describe('ReportPage', () => {
  let fake: FakeFetch
  beforeEach(() => setToken('t1'))
  afterEach(() => fake?.restore())

  it('shows the original and the corrected answer', async () => {
    fake = renderAt('/reports/r1', { 'GET /freshness/reports/r1': () => json(200, completeReport()) })
    expect(await screen.findByRole('heading', { name: 'Original answer' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Corrected answer' })).toBeInTheDocument()
    expect(screen.getAllByText('$25 per month', { selector: 'ins' }).length).toBeGreaterThan(0)
    expect(screen.getByText('Checked in 6.3 s')).toBeInTheDocument()
    expect(screen.getByText('2 Tavily credits used')).toBeInTheDocument()
    expect(screen.getByText('2 claims checked. 1 claim is outdated.')).toBeInTheDocument()
  })

  it('names the mode and the policy in words, and sets the report on the page rather than in cards', async () => {
    fake = renderAt('/reports/r1', { 'GET /freshness/reports/r1': () => json(200, completeReport({ mode: 'sync', policy: 'heal' })) })
    const original = await screen.findByRole('heading', { name: 'Original answer' })
    expect(screen.getByText('Before reply')).toBeInTheDocument()
    expect(screen.getByText('Correct')).toBeInTheDocument()
    expect(original.parentElement).not.toHaveClass('rounded-xl')
    for (const item of screen.getAllByRole('listitem').filter((li) => li.closest('[aria-labelledby="claims-title"]'))) {
      expect(item).not.toHaveClass('rounded-xl')
    }
  })

  it('says when no policy applied, as for pasted text', async () => {
    fake = renderAt('/reports/r1', { 'GET /freshness/reports/r1': () => json(200, completeReport({ mode: 'check', policy: '' })) })
    await screen.findByRole('heading', { name: 'Original answer' })
    expect(screen.getByText('Pasted text check')).toBeInTheDocument()
    expect(screen.getByText('Not applied')).toBeInTheDocument()
  })

  it('does not link a claim to the report the reader is already on', async () => {
    fake = renderAt('/reports/r1', { 'GET /freshness/reports/r1': () => json(200, completeReport()) })
    await screen.findByRole('heading', { name: 'Original answer' })
    expect(screen.queryByRole('link', { name: 'Open full report' })).toBeNull()
  })

  it('says when the answer was blocked', async () => {
    fake = renderAt('/reports/r1', { 'GET /freshness/reports/r1': () => json(200, completeReport({ blocked: true })) })
    expect(await screen.findByText('This answer was blocked before it reached the user.')).toBeInTheDocument()
  })

  it('says when nothing needed correction', async () => {
    fake = renderAt('/reports/r1', {
      'GET /freshness/reports/r1': () => json(200, completeReport({ claims: [freshClaim()], answer_text: answer })),
    })
    expect(await screen.findByText('No claims needed correction.')).toBeInTheDocument()
  })

  it('says when the report does not exist', async () => {
    fake = renderAt('/reports/gone', { 'GET /freshness/reports/gone': () => json(404, { error: 'The report was not found.' }) })
    expect(await screen.findByText('The report was not found.')).toBeInTheDocument()
  })
})
