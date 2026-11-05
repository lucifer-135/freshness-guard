import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { CheckPanel } from './CheckPanel'
import { setToken } from '@/api/client'
import { reportKey } from '@/live/reportCache'
import { fakeFetch, json, type FakeFetch, type Route } from '@/test/fakeFetch'
import { answer, completeReport, freshClaim, staleClaim } from '@/test/fixtures'

function setup(check: Route) {
  const queryClient = new QueryClient()
  const fake = fakeFetch({
    'POST /freshness/check': check,
    'GET /freshness/reports/r1': () => json(200, { id: 'r1', status: 'pending', claims: [] }),
  })
  render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <CheckPanel />
      </MemoryRouter>
    </QueryClientProvider>,
  )
  return { fake, queryClient }
}

const field = () => screen.getByLabelText('Answer to check')
const button = () => screen.getByRole('button', { name: 'Check this answer' })

describe('CheckPanel', () => {
  let fake: FakeFetch
  beforeEach(() => setToken('t1'))
  afterEach(() => fake?.restore())

  it('counts characters and needs some text', async () => {
    ;({ fake } = setup(() => json(200, completeReport())))
    expect(button()).toBeDisabled()
    await userEvent.type(field(), 'a'.repeat(45))
    expect(screen.getByText('45 of 20,000 characters')).toBeInTheDocument()
    expect(button()).toBeEnabled()
  })

  it('refuses text over the limit', () => {
    ;({ fake } = setup(() => json(200, completeReport())))
    fireEvent.change(field(), { target: { value: 'a'.repeat(20001) } })
    expect(screen.getByText('The text must not be longer than 20,000 characters.')).toBeInTheDocument()
    expect(button()).toBeDisabled()
  })

  it('checks the text and shows its claims', async () => {
    ;({ fake } = setup(() => json(200, completeReport())))
    fireEvent.change(field(), { target: { value: answer } })
    await userEvent.click(button())
    expect(JSON.parse(fake.requests[0].body ?? '{}')).toEqual({ text: answer })
    // Verdicts are revealed one at a time, so wait for the final sentence.
    expect((await screen.findAllByText('2 claims checked. 1 claim is outdated.', {}, { timeout: 3000 })).length).toBeGreaterThan(0)
    expect(screen.getByRole('button', { name: 'Example Cloud Pro costs $20 per month' })).toHaveAttribute(
      'data-claim-status',
      'outdated',
    )
  })

  it('follows a check that is still running', async () => {
    let queryClient: QueryClient
    ;({ fake, queryClient } = setup(() => json(202, { id: 'r1', status: 'pending' })))
    fireEvent.change(field(), { target: { value: answer } })
    await userEvent.click(button())
    expect((await screen.findAllByText('Finding the claims in this answer.')).length).toBeGreaterThan(0)
    const pending = completeReport({ status: 'pending', claims: [staleClaim({ verdict: 'NOT_CHECKED' }), freshClaim({ verdict: 'NOT_CHECKED' })] })
    act(() => queryClient.setQueryData(reportKey('r1'), pending))
    expect((await screen.findAllByText('Checking claims against live sources. 0 of 2 claims checked.')).length).toBeGreaterThan(0)
  })

  it('shows an API error and allows another try', async () => {
    ;({ fake } = setup(() => json(503, { error: 'The verification queue is full. Try again later.' })))
    fireEvent.change(field(), { target: { value: answer } })
    await userEvent.click(button())
    expect(await screen.findByText('The verification queue is full. Try again later.')).toBeInTheDocument()
    expect(button()).toBeEnabled()
  })
})
