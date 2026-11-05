import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { Composer } from './Composer'
import { ModelPicker } from './ModelPicker'
import { setToken } from '@/api/client'
import type { Model } from '@/api/types'
import { fakeFetch, json, type FakeFetch } from '@/test/fakeFetch'

// Bifrost lists 569 models; the picker has to stay usable with all of them.
const many: Model[] = [
  ...Array.from({ length: 569 }, (_, i) => ({ id: `openrouter/vendor-${i}/model-${i}`, provider: 'openrouter' })),
  { id: 'groq/openai/gpt-oss-20b', provider: 'groq' },
]

function Harness({ models, onChange = () => {}, initial = '' }: { models: Model[]; onChange?: (id: string) => void; initial?: string }) {
  const [value, setValue] = useState(initial)
  return (
    <>
      <label htmlFor="model">Model</label>
      <ModelPicker
        id="model"
        models={models}
        value={value}
        placeholder="Loading models"
        onChange={(id) => {
          setValue(id)
          onChange(id)
        }}
      />
    </>
  )
}

describe('ModelPicker', () => {
  it('shows the chosen model on its button', () => {
    render(<Harness models={many} initial="groq/openai/gpt-oss-20b" />)
    expect(screen.getByRole('combobox')).toHaveTextContent('groq/openai/gpt-oss-20b')
  })

  it('shows the placeholder before a model is chosen', () => {
    render(<Harness models={[]} />)
    expect(screen.getByRole('combobox')).toHaveTextContent('Loading models')
  })

  it('filters the list as you type', async () => {
    const user = userEvent.setup()
    render(<Harness models={many} />)
    await user.click(screen.getByRole('combobox', { name: 'Model' }))
    expect(screen.getByRole('combobox', { name: 'Model' })).toHaveAttribute('aria-expanded', 'true')
    await user.type(screen.getByRole('combobox', { name: 'Search models' }), 'oss')
    const options = screen.getAllByRole('option')
    expect(options).toHaveLength(1)
    expect(options[0]).toHaveTextContent('groq/openai/gpt-oss-20b')
  })

  it('chooses a model with the keyboard and returns focus to the button', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<Harness models={many} onChange={onChange} />)
    const button = screen.getByRole('combobox', { name: 'Model' })
    await user.click(button)
    await user.type(screen.getByRole('combobox', { name: 'Search models' }), 'gpt-oss')
    await user.keyboard('{Enter}')
    expect(onChange).toHaveBeenCalledWith('groq/openai/gpt-oss-20b')
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
    expect(button).toHaveFocus()
    expect(button).toHaveTextContent('groq/openai/gpt-oss-20b')
  })

  it('says when no model matches', async () => {
    const user = userEvent.setup()
    render(<Harness models={many} />)
    await user.click(screen.getByRole('combobox', { name: 'Model' }))
    await user.type(screen.getByRole('combobox', { name: 'Search models' }), 'zzzzzz')
    expect(screen.getByText('No models match this search.')).toBeInTheDocument()
  })
})

describe('Composer model choice', () => {
  let fake: FakeFetch
  beforeEach(() => {
    setToken('t1')
    localStorage.clear()
  })
  afterEach(() => fake?.restore())

  it('sends with the model picked from the list and remembers it', async () => {
    fake = fakeFetch({
      'GET /freshness/models': () =>
        json(200, {
          models: [
            { id: 'openai/fake-a', provider: 'openai' },
            { id: 'openai/fake-b', provider: 'openai' },
          ],
        }),
      'GET /freshness/status': () =>
        json(200, {
          gateway: { reachable: true, models: 2 },
          credits: { used: 0, budget: 30 },
          settings: { default_mode: 'async', default_policy: 'heal', sync_timeout_ms: 8000 },
        }),
    })
    const onSend = vi.fn()
    const user = userEvent.setup()
    render(
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <Composer busy={false} onSend={onSend} />
      </QueryClientProvider>,
    )
    const button = await screen.findByRole('combobox', { name: 'Model' })
    await screen.findByText('openai/fake-a')
    await user.click(button)
    await user.click(await screen.findByRole('option', { name: 'openai/fake-b' }))
    await user.type(screen.getByLabelText('Message'), 'What does it cost?')
    await user.click(screen.getByRole('button', { name: 'Send' }))

    expect(onSend).toHaveBeenCalledWith('What does it cost?', expect.objectContaining({ model: 'openai/fake-b' }))
    expect(localStorage.getItem('fg.model')).toBe('openai/fake-b')
  })
})
