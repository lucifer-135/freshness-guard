import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { Segmented } from './Segmented'

const options = [
  { value: 'async', label: 'Background' },
  { value: 'sync', label: 'Before reply' },
  { value: 'off', label: 'Off' },
]

describe('Segmented', () => {
  it('is a labelled radio group that marks the chosen option', () => {
    render(<Segmented label="Verification mode" value="async" options={options} onChange={() => {}} />)
    expect(screen.getByRole('radiogroup', { name: 'Verification mode' })).toBeInTheDocument()
    expect(screen.getByRole('radio', { name: 'Background' })).toHaveAttribute('aria-checked', 'true')
    expect(screen.getByRole('radio', { name: 'Off' })).toHaveAttribute('aria-checked', 'false')
  })

  it('reports a clicked option', async () => {
    const onChange = vi.fn()
    render(<Segmented label="Verification mode" value="async" options={options} onChange={onChange} />)
    await userEvent.click(screen.getByRole('radio', { name: 'Before reply' }))
    expect(onChange).toHaveBeenCalledWith('sync')
  })

  it('moves to the next option with the right arrow key', async () => {
    const onChange = vi.fn()
    render(<Segmented label="Verification mode" value="async" options={options} onChange={onChange} />)
    screen.getByRole('radio', { name: 'Background' }).focus()
    await userEvent.keyboard('{ArrowRight}')
    expect(onChange).toHaveBeenCalledWith('sync')
  })
})
