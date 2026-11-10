import { fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
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
    expect(screen.getByRole('radio', { name: 'Background' })).toBeChecked()
    expect(screen.getByRole('radio', { name: 'Off' })).not.toBeChecked()
  })

  it('reports a clicked option', async () => {
    const onChange = vi.fn()
    render(<Segmented label="Verification mode" value="async" options={options} onChange={onChange} />)
    await userEvent.click(screen.getByRole('radio', { name: 'Before reply' }))
    expect(onChange).toHaveBeenCalledWith('sync')
  })

  it('accepts a plain click event, as sent by screen readers and voice control', () => {
    const onChange = vi.fn()
    render(<Segmented label="Verification mode" value="async" options={options} onChange={onChange} />)
    fireEvent.click(screen.getByRole('radio', { name: 'Before reply' }))
    expect(onChange).toHaveBeenCalledWith('sync')
  })

  it('reports a pointer click once, not again from the click event', async () => {
    const onChange = vi.fn()
    function Harness() {
      const [value, setValue] = useState('async')
      return (
        <Segmented
          label="Verification mode"
          value={value}
          options={options}
          onChange={(next) => {
            onChange(next)
            setValue(next)
          }}
        />
      )
    }
    render(<Harness />)
    await userEvent.click(screen.getByRole('radio', { name: 'Off' }))
    expect(onChange).toHaveBeenCalledTimes(1)
    expect(onChange).toHaveBeenCalledWith('off')
  })

  it('draws unselected labels muted and the chosen one in the primary text colour', () => {
    render(<Segmented label="Verification mode" value="async" options={options} onChange={() => {}} />)
    // text-muted is about 7.4:1 on the surface-1 track, so unselected labels stay readable.
    const label = screen.getByText('Off')
    expect(label).toHaveClass('text-text-muted', 'peer-checked:text-text')
  })

  it('raises the chosen option as a white segment on a grey track, as light interfaces do', () => {
    render(<Segmented label="Verification mode" value="async" options={options} onChange={() => {}} />)
    expect(screen.getByRole('radiogroup')).toHaveClass('bg-surface-2')
    expect(screen.getByText('Off')).toHaveClass('peer-checked:bg-bg', 'peer-checked:shadow-[var(--shadow-raised)]')
  })

  it('moves to the next option with the right arrow key', async () => {
    const onChange = vi.fn()
    render(<Segmented label="Verification mode" value="async" options={options} onChange={onChange} />)
    screen.getByRole('radio', { name: 'Background' }).focus()
    await userEvent.keyboard('{ArrowRight}')
    expect(onChange).toHaveBeenCalledWith('sync')
  })
})
