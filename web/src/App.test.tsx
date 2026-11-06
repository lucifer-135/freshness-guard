import { render, screen } from '@testing-library/react'
import { beforeEach, expect, it } from 'vitest'
import App from './App'
import { clearToken } from '@/api/client'

beforeEach(() => clearToken())

it('asks for the plugin API token first', () => {
  render(<App />)
  expect(screen.getByRole('heading', { level: 2, name: 'Try it' })).toBeInTheDocument()
})
