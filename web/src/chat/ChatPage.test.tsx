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

// The claim inspector sits beside the chat from 80rem (Tailwind xl, 1280px at the default font size); below that it is a sheet.
const realMatchMedia = window.matchMedia
function setScreenWide(wide: boolean) {
  window.matchMedia = (query: string) =>
    ({ ...realMatchMedia(query), matches: wide && query.includes('min-width: 80rem') }) as MediaQueryList
}

describe('ChatPage', () => {
  let fake: FakeFetch
  beforeEach(() => {
    clearToken()
    setToken('t1')
  })
  afterEach(() => {
    fake?.restore()
    window.matchMedia = realMatchMedia
  })

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

  it('publishes the docked composer height so scrolling keeps focused elements clear of it', async () => {
    fake = setup(streamedAnswer)
    await screen.findByLabelText('Message')
    expect(document.documentElement.style.getPropertyValue('--composer-inset')).toMatch(/^\d+px$/)
    await userEvent.click(screen.getByRole('radio', { name: 'Check any answer' }))
    expect(document.documentElement.style.getPropertyValue('--composer-inset')).toBe('')
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
    const version = await screen.findByRole('radiogroup', { name: 'Answer version' })
    expect(within(version).getByRole('radio', { name: 'Corrected' })).toHaveAttribute('aria-checked', 'true')
    expect((await screen.findAllByText('$25', { selector: 'ins' })).length).toBeGreaterThan(0)
    await userEvent.click(within(version).getByRole('radio', { name: 'Original' }))
    expect(document.querySelectorAll('ins')).toHaveLength(0)
  })

  it('summarises the verdicts in the message header', async () => {
    fake = setup(streamedAnswer, completeReport())
    await send()
    expect(await screen.findByText(`Freshness ${completeReport().score} of 100`, {}, { timeout: 3000 })).toBeInTheDocument()
    expect(screen.getByText('1 outdated')).toBeInTheDocument()
    expect(screen.getByText('1 current')).toBeInTheDocument()
  })

  it('shows no summary chips when the answer has no claims', async () => {
    fake = setup(streamedAnswer, completeReport({ claims: [], total_claims: 0, checked_count: 0, stale_count: 0 }))
    await send()
    // The sentence appears in the message header and in the page's live region.
    await screen.findAllByText('No time-sensitive claims were found in this answer.', {}, { timeout: 3000 })
    expect(screen.queryByText(/^\d+ (incorrect|outdated|could not be verified|current|not checked|being checked)$/)).toBeNull()
  })

  it('shows a short loader label next to the longer progress sentence while claims are checked', async () => {
    fake = setup(streamedAnswer, completeReport({ status: 'pending' }))
    await send()
    // The loader keeps to one line; the sentence says how far the check is.
    expect(await screen.findByText('Checking claims', {}, { timeout: 3000 })).toBeInTheDocument()
    expect(screen.getAllByText(/^Checking claims against live sources\./).length).toBeGreaterThan(0)
  })

  it('ends the checking loader with an error when the check fails', async () => {
    fake = setup(streamedAnswer, completeReport({ status: 'error', reason: 'The judge model did not respond.' }))
    await send()
    expect((await screen.findAllByText('The claims could not be checked.', {}, { timeout: 3000 })).length).toBeGreaterThan(0)
    // A failed check has no verdicts to count, and the working loader is gone.
    expect(screen.queryByRole('list', { name: 'Verdict summary' })).toBeNull()
    expect(screen.queryByText('Checking claims')).toBeNull()
    // The long sentence is shown once as text; the one-line loader never carries it, so it cannot overflow a narrow screen.
    const sentence = 'The freshness check could not finish. The judge model did not respond.'
    for (const el of screen.getAllByText(sentence)) expect(el.closest('[aria-hidden="true"]')).toBeNull()
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

  it('expands the chosen claim inside the claims sheet on narrow screens', async () => {
    fake = setup(streamedAnswer, completeReport())
    await send()
    await screen.findByText('2 claims checked. 1 claim is outdated.', {}, { timeout: 3000 })
    await userEvent.click(screen.getByRole('button', { name: 'Claims' }))
    const sheet = await screen.findByRole('dialog', { name: 'Claims in this answer' })
    await userEvent.click(within(sheet).getAllByRole('button', { name: /Example Cloud Pro costs/ })[0])
    expect(within(sheet).getByText('Current value: $25 per month')).toBeInTheDocument()
    expect(screen.getByRole('dialog', { name: 'Claims in this answer' })).toBeInTheDocument()
  })

  it('lists claims by risk and expands a claim in the inspector', async () => {
    setScreenWide(true)
    fake = setup(streamedAnswer, completeReport())
    await send()
    // Wait until every staggered verdict is shown.
    await screen.findByText('2 claims checked. 1 claim is outdated.', {}, { timeout: 3000 })
    const inspector = await screen.findByRole('complementary', { name: 'Claims in this answer' })
    const rows = await within(inspector).findAllByRole('button', {}, { timeout: 3000 })
    expect(rows[0]).toHaveTextContent('Example Cloud Pro costs $20 per month')
    expect(rows[0]).toHaveTextContent('$20 per month → $25 per month')
    expect(rows[1]).toHaveTextContent('its current version is 4.2')
    await userEvent.click(rows[0])
    expect(rows[0]).toHaveAttribute('aria-expanded', 'true')
    expect(within(inspector).getByText('Current value: $25 per month')).toBeInTheDocument()
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('collapses an expanded claim when its row is chosen again', async () => {
    setScreenWide(true)
    fake = setup(streamedAnswer, completeReport())
    await send()
    await screen.findByText('2 claims checked. 1 claim is outdated.', {}, { timeout: 3000 })
    const inspector = screen.getByRole('complementary', { name: 'Claims in this answer' })
    const row = within(inspector).getAllByRole('button')[0]
    await userEvent.click(row)
    expect(row).toHaveAttribute('aria-expanded', 'true')
    await userEvent.click(row)
    expect(row).toHaveAttribute('aria-expanded', 'false')
    expect(within(inspector).queryByText('Why this claim was checked')).toBeNull()
  })

  it('does not carry a selected claim over to the next answer', async () => {
    setScreenWide(true)
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
    await send('first')
    await screen.findAllByText('2 claims checked. 1 claim is outdated.', {}, { timeout: 3000 })
    const inspector = () => screen.getByRole('complementary', { name: 'Claims in this answer' })
    await userEvent.click(within(inspector()).getAllByRole('button')[0])
    expect(within(inspector()).getAllByRole('button')[0]).toHaveAttribute('aria-expanded', 'true')
    await send('second')
    await waitFor(() => expect(screen.getAllByText('2 claims checked. 1 claim is outdated.')).toHaveLength(2), { timeout: 3000 })
    // Claim IDs repeat across reports (c1, c2, ...), so the new answer's first claim must not open on its own.
    await waitFor(() => expect(within(inspector()).getAllByRole('button')[0]).toHaveAttribute('aria-expanded', 'false'))
    expect(within(inspector()).queryByText('Why this claim was checked')).toBeNull()
  })

  it('shows a short preview when a claim is hovered', async () => {
    fake = setup(streamedAnswer, completeReport())
    await send()
    await screen.findByText('2 claims checked. 1 claim is outdated.', {}, { timeout: 3000 })
    await userEvent.hover(screen.getByRole('button', { name: 'Example Cloud Pro costs $20 per month' }))
    const preview = await screen.findByRole('dialog')
    expect(within(preview).getByText('Outdated')).toBeInTheDocument()
    expect(within(preview).getByText('was $20 per month, now $25 per month as of 1 October 2026')).toBeInTheDocument()
    expect(within(preview).getByRole('button', { name: 'Show details' })).toBeInTheDocument()
    expect(within(preview).queryByText('Why this claim was checked')).toBeNull()
  })

  it('at 1280px Show details opens the claim in the inspector', async () => {
    setScreenWide(true)
    fake = setup(streamedAnswer, completeReport())
    await send()
    await screen.findByText('2 claims checked. 1 claim is outdated.', {}, { timeout: 3000 })
    await userEvent.hover(screen.getByRole('button', { name: 'Example Cloud Pro costs $20 per month' }))
    await userEvent.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Show details' }))
    const inspector = screen.getByRole('complementary', { name: 'Claims in this answer' })
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(within(inspector).getByRole('button', { name: /Example Cloud Pro costs/ })).toHaveAttribute('aria-expanded', 'true')
    expect(within(inspector).getByText('Why this claim was checked')).toBeInTheDocument()
  })

  it("below 1280px Show details opens the sheet with the claim's details", async () => {
    setScreenWide(false)
    fake = setup(streamedAnswer, completeReport())
    await send()
    await screen.findByText('2 claims checked. 1 claim is outdated.', {}, { timeout: 3000 })
    await userEvent.hover(screen.getByRole('button', { name: 'Example Cloud Pro costs $20 per month' }))
    await userEvent.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Show details' }))
    const sheet = await screen.findByRole('dialog', { name: 'Claims in this answer' })
    expect(within(sheet).getByText('Why this claim was checked')).toBeInTheDocument()
    expect(within(sheet).getByRole('button', { name: /Example Cloud Pro costs/ })).toHaveAttribute('aria-expanded', 'true')
  })
})
