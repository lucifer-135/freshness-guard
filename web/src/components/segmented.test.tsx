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
    expect(screen.getByRole('radio', { name: 'Background' })).toHaveAttribute('aria-checked', 'true')
    expect(screen.getByRole('radio', { name: 'Off' })).toHaveAttribute('aria-checked', 'false')
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

  it('draws unselected labels in the primary text colour, which stays readable at the thumb overlay opacity', () => {
    render(<Segmented label="Verification mode" value="async" options={options} onChange={() => {}} />)
    expect(screen.getByRole('radiogroup').style.getPropertyValue('--rs-ink')).toBe('var(--text)')
  })

  it('moves to the next option with the right arrow key', async () => {
    const onChange = vi.fn()
    render(<Segmented label="Verification mode" value="async" options={options} onChange={onChange} />)
    screen.getByRole('radio', { name: 'Background' }).focus()
    await userEvent.keyboard('{ArrowRight}')
    expect(onChange).toHaveBeenCalledWith('sync')
  })
})
