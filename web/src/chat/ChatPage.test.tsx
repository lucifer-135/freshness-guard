import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createMemoryRouter } from 'react-router'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { AppProviders } from '@/App'
import { routes } from '@/app/routes'
import { clearToken, setToken } from '@/api/client'
import type { Report } from '@/api/types'
import { fakeFetch, json, streamBody, type FakeFetch, type RecordedRequest, type Route } from '@/test/fakeFetch'
import { answer, completeReport } from '@/test/fixtures'

const status = json(200, {
  gateway: { reachable: true, models: 2 },
  credits: { used: 2, budget: 200 },
  settings: { default_mode: 'async', default_policy: 'heal', sync_timeout_ms: 8000 },
})

function chunk(content: string): string {
  return `data: ${JSON.stringify({ choices: [{ delta: { content } }] })}\n\n`
}

const streamedAnswer: Route = () =>
  new Response(streamBody([chunk('Example Cloud Pro costs $20 per month, '), chunk('and its current version is 4.2.'), 'data: [DONE]\n\n']), {
    headers: { 'Content-Type': 'text/event-stream', 'X-Freshness-Report-Id': 'r1', 'X-Freshness-Status': 'pending' },
  })

function syncAnswer(text: string, headers: Record<string, string>): Route {
  return () => json(200, { choices: [{ message: { content: text } }] }, headers)
}

function setup(chat: Route, report?: Report) {
  const routesMap: Record<string, Route> = {
    'GET /freshness/status': () => status.clone(),
    'GET /freshness/models': () => json(200, { models: [{ id: 'openai/fake-model', provider: 'openai' }] }),
    'GET /freshness/events': () => new Response(new ReadableStream({ start() {} }), { headers: { 'Content-Type': 'text/event-stream' } }),
    'POST /freshness/chat': chat,
  }
  if (report) routesMap[`GET /freshness/reports/${report.id}`] = () => json(200, report)
  const fake = fakeFetch(routesMap)
  render(
    <AppProviders
      router={createMemoryRouter(routes, { initialEntries: ['/freshness/ui/chat'], basename: '/freshness/ui' })}
    />,
  )
  return fake
}

async function send(text = 'What does it cost?') {
  const field = await screen.findByLabelText('Message')
  await userEvent.type(field, text)
  await userEvent.click(screen.getByRole('button', { name: 'Send' }))
}

function chatBody(fake: FakeFetch): Record<string, unknown> {
  const req = fake.requests.find((r: RecordedRequest) => r.path === '/freshness/chat')
  return JSON.parse(req?.body ?? '{}') as Record<string, unknown>
}

describe('ChatPage', () => {
  let fake: FakeFetch
  beforeEach(() => {
    clearToken()
    setToken('t1')
  })
  afterEach(() => fake?.restore())

  it('streams an async answer and underlines its claims', async () => {
    fake = setup(streamedAnswer, completeReport())
    await screen.findByText('Ask a question to see which parts of the answer are still current.')
    await send()
    expect(chatBody(fake)).toMatchObject({ mode: 'async', stream: true, model: 'openai/fake-model' })
    const claim = await screen.findByRole('button', { name: 'Example Cloud Pro costs $20 per month' }, { timeout: 3000 })
    await screen.findByText('2 claims checked. 1 claim is outdated.', {}, { timeout: 3000 })
    expect(claim).toHaveAttribute('data-claim-status', 'outdated')
  })

  it('shows the policy choices only for sync checks', async () => {
    fake = setup(streamedAnswer)
    await screen.findByLabelText('Message')
    expect(screen.queryByRole('radio', { name: 'Correct' })).toBeNull()
    await userEvent.click(screen.getByRole('radio', { name: 'Before reply' }))
    expect(screen.getByRole('radio', { name: 'Correct' })).toBeChecked()
  })

  it('explains the chosen verification mode under the composer', async () => {
    fake = setup(streamedAnswer)
    await screen.findByLabelText('Message')
    expect(screen.getByText('Claims are checked after the answer is shown.')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('radio', { name: 'Before reply' }))
    expect(screen.getByText('The answer waits up to 8 seconds while its claims are checked.')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('radio', { name: 'Off' }))
    expect(screen.getByText('This answer will not be checked.')).toBeInTheDocument()
  })

  it('keeps the page heading for screen readers only', async () => {
    fake = setup(streamedAnswer)
    await screen.findByLabelText('Message')
    expect(screen.getByRole('heading', { level: 1, name: 'Freshness-checked chat' })).toHaveClass('sr-only')
  })

  it('shows a corrected sync answer', async () => {
    const healed = answer.replace('$20', '$25')
    fake = setup(
      syncAnswer(healed, { 'X-Freshness-Report-Id': 'r1', 'X-Freshness-Status': 'complete', 'X-Freshness-Healed': '1' }),
      completeReport({ mode: 'sync', policy: 'heal', healed_text: healed, healed_count: 1 }),
    )
    await screen.findByLabelText('Message')
    await userEvent.click(screen.getByRole('radio', { name: 'Before reply' }))
    await send()
    expect(chatBody(fake)).toMatchObject({ mode: 'sync', policy: 'heal', stream: false })
    expect(await screen.findByRole('switch', { name: 'Show corrected answer' })).toBeChecked()
    expect((await screen.findAllByText('$25', { selector: 'ins' })).length).toBeGreaterThan(0)
  })

  it('explains a blocked answer and links its report', async () => {
    fake = setup(() =>
      json(422, {
        error: {
          type: 'freshness_blocked',
          message: 'The answer was blocked because it contains outdated or incorrect claims. Report ID: abc-123.',
        },
      }),
    )
    await screen.findByLabelText('Message')
    await userEvent.click(screen.getByRole('radio', { name: 'Before reply' }))
    await userEvent.click(screen.getByRole('radio', { name: 'Block' }))
    await send()
    expect(await screen.findByText('The answer was blocked because it contains outdated or incorrect claims.')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Open the full report' })).toHaveAttribute('href', '/freshness/ui/reports/abc-123')
    await userEvent.type(screen.getByLabelText('Message'), 'again')
    expect(screen.getByRole('button', { name: 'Send' })).toBeEnabled()
  })

  it('explains a sync check that took too long', async () => {
    fake = setup(syncAnswer(answer, { 'X-Freshness-Report-Id': 'r1', 'X-Freshness-Status': 'pending' }), completeReport({ status: 'pending' }))
    await screen.findByLabelText('Message')
    await userEvent.click(screen.getByRole('radio', { name: 'Before reply' }))
    await send()
    expect(
      await screen.findByText(
        'Verification is taking longer than 8 seconds. The answer is shown now, and the results will appear when they are ready.',
      ),
    ).toBeInTheDocument()
  })

  it('explains a skipped check and a failed request', async () => {
    fake = setup(syncAnswer(answer, { 'X-Freshness-Status': 'skipped' }))
    await screen.findByLabelText('Message')
    await userEvent.click(screen.getByRole('radio', { name: 'Before reply' }))
    await send()
    expect(
      await screen.findByText(
        'This answer was not checked, because the verification queue was full or the request was not selected for sampling.',
      ),
    ).toBeInTheDocument()
  })

  it('reports a failed request and lets the reader try again', async () => {
    fake = setup(() => json(502, { error: 'Bifrost could not be reached.' }))
    await send()
    expect(await screen.findByText('The answer could not be loaded. Bifrost could not be reached.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Send' })).toBeDisabled()
    await userEvent.type(screen.getByLabelText('Message'), 'again')
    expect(screen.getByRole('button', { name: 'Send' })).toBeEnabled()
  })

  it('does not load a report when checking is off', async () => {
    fake = setup(streamedAnswer, completeReport())
    await screen.findByLabelText('Message')
    await userEvent.click(screen.getByRole('radio', { name: 'Off' }))
    await send()
    expect(await screen.findByText('Freshness checking was off for this answer.')).toBeInTheDocument()
    expect(fake.requests.some((r) => r.path.startsWith('/freshness/reports/'))).toBe(false)
  })

  it('keeps the conversation across tabs and pages', async () => {
    fake = setup(streamedAnswer, completeReport())
    await send()
    await screen.findByText('2 claims checked. 1 claim is outdated.', {}, { timeout: 3000 })
    await userEvent.click(screen.getByRole('radio', { name: 'Check any answer' }))
    await userEvent.click(screen.getByRole('radio', { name: 'Chat' }))
    expect(screen.getByText('What does it cost?')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('link', { name: 'Reports' }))
    await userEvent.click(screen.getByRole('link', { name: 'Chat' }))
    expect(await screen.findByText('What does it cost?')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Example Cloud Pro costs $20 per month' })).toBeInTheDocument()
  })

  it('scrolls to the claim in the latest answer, not an earlier one with the same claim ID', async () => {
    let n = 0
    const twoAnswers: Route = () => {
      n++
      return new Response(streamBody([chunk(answer), 'data: [DONE]\n\n']), {
        headers: { 'Content-Type': 'text/event-stream', 'X-Freshness-Report-Id': `r${n}`, 'X-Freshness-Status': 'pending' },
      })
    }
    fake = setup(twoAnswers, completeReport())
    fake.restore()
    fake = fakeFetch({
      'GET /freshness/status': () => status.clone(),
      'GET /freshness/models': () => json(200, { models: [{ id: 'openai/fake-model', provider: 'openai' }] }),
      'GET /freshness/events': () => new Response(new ReadableStream({ start() {} }), { headers: { 'Content-Type': 'text/event-stream' } }),
      'POST /freshness/chat': twoAnswers,
      'GET /freshness/reports/r1': () => json(200, completeReport({ id: 'r1' })),
      'GET /freshness/reports/r2': () => json(200, completeReport({ id: 'r2' })),
    })
    const scrolled: Element[] = []
    Element.prototype.scrollIntoView = function (this: Element) {
      scrolled.push(this)
    }
    await send('first')
    await screen.findAllByText('2 claims checked. 1 claim is outdated.', {}, { timeout: 3000 })
    await send('second')
    await waitFor(() => expect(screen.getAllByText('2 claims checked. 1 claim is outdated.')).toHaveLength(2), { timeout: 3000 })
    const inspector = screen.getByRole('complementary', { name: 'Claims in this answer' })
    await userEvent.click(within(inspector).getAllByRole('button')[0])
    const articles = screen.getAllByRole('article')
    expect(scrolled).toHaveLength(1)
    expect(articles[articles.length - 1]).toContainElement(scrolled[0] as HTMLElement)
  })

  it('closes the claims sheet on narrow screens and opens the chosen claim', async () => {
    fake = setup(streamedAnswer, completeReport())
    await send()
    await screen.findByText('2 claims checked. 1 claim is outdated.', {}, { timeout: 3000 })
    await userEvent.click(screen.getByRole('button', { name: 'Claims' }))
    const sheet = await screen.findByRole('dialog', { name: 'Claims in this answer' })
    await userEvent.click(within(sheet).getAllByRole('button', { name: /Example Cloud Pro costs/ })[0])
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Claims in this answer' })).toBeNull())
    expect(await screen.findByRole('dialog')).toHaveTextContent('Current value: $25 per month')
  })

  it('lists claims by risk and opens a claim from the inspector', async () => {
    fake = setup(streamedAnswer, completeReport())
    await send()
    // Wait until every staggered verdict is shown.
    await screen.findByText('2 claims checked. 1 claim is outdated.', {}, { timeout: 3000 })
    const inspector = await screen.findByRole('complementary', { name: 'Claims in this answer' })
    const rows = await within(inspector).findAllByRole('button', {}, { timeout: 3000 })
    expect(rows[0]).toHaveTextContent('Example Cloud Pro costs $20 per month')
    expect(rows[1]).toHaveTextContent('its current version is 4.2')
    await userEvent.click(rows[0])
    expect(await screen.findByRole('dialog')).toHaveTextContent('Current value: $25 per month')
  })
})
