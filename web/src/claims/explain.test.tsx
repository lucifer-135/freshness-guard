import { act, fireEvent, render, screen } from '@testing-library/react'
import type { ReactNode } from 'react'
import { MemoryRouter } from 'react-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ClaimText } from './ClaimText'
import { decayModel, probabilityAt, scrubSentence } from './decay'
import { DecayCurve } from './DecayCurve'
import { ExplainCard } from './ExplainCard'
import { safeHref } from './links'
import { normalizeReport } from '@/api/client'
import { answer, completeReport, freshClaim, staleClaim } from '@/test/fixtures'

const day = 24 * 60 * 60 * 1000

function inRouter(node: ReactNode) {
  return render(<MemoryRouter>{node}</MemoryRouter>)
}

describe('safeHref', () => {
  it('links only http and https URLs', () => {
    expect(safeHref('https://a.com/x')).toBe('https://a.com/x')
    expect(safeHref('http://a.com')).toBe('http://a.com/')
    expect(safeHref('javascript:alert(1)')).toBeNull()
    expect(safeHref('data:text/html,x')).toBeNull()
    expect(safeHref('not a url')).toBeNull()
  })
})

describe('decay', () => {
  const report = (days: number) => normalizeReport({ id: 'r', created_at: '2026-11-01T00:00:00Z', model_age_days: days })
  const created = new Date('2026-11-01T00:00:00Z')

  it('matches the triage formula for claims that were not verified', () => {
    const notChecked = staleClaim({ verdict: 'NOT_CHECKED', checked: false, half_life_days: 60 })
    expect(probabilityAt(decayModel(notChecked, report(60))!, created)).toBeCloseTo(0.5, 3)
    const marked = { ...notChecked, temporal_marker: 'currently' }
    expect(probabilityAt(decayModel(marked, report(60))!, created)).toBeCloseTo(0.75, 3)
    const version = freshClaim({ verdict: 'NOT_CHECKED', checked: false, half_life_days: 90 })
    expect(probabilityAt(decayModel(version, report(180))!, created)).toBeCloseTo(0.75, 3)
  })

  it('starts a verified claim at the time it was checked', () => {
    const model = decayModel(freshClaim({ half_life_days: 60 }), report(412))!
    expect(probabilityAt(model, created)).toBeCloseTo(0, 3)
    expect(probabilityAt(model, new Date(created.getTime() + 60 * day))).toBeCloseTo(0.5, 3)
  })

  it('has no curve for outdated claims or facts without a half-life', () => {
    expect(decayModel(staleClaim(), report(10))).toBeNull()
    expect(decayModel(freshClaim({ half_life_days: 0 }), report(10))).toBeNull()
  })

  it('describes a point on the curve', () => {
    expect(scrubSentence(new Date('2027-01-15T00:00:00Z'), 0.42)).toBe(
      'On 15 January 2027, this claim has a 42% probability of being outdated.',
    )
  })
})

describe('ExplainCard', () => {
  it('shows the current value, the evidence and the inference apart', () => {
    inRouter(<ExplainCard claim={staleClaim()} report={completeReport()} />)
    expect(screen.getByText('Outdated')).toBeInTheDocument()
    expect(screen.getByText('Current value: $25 per month')).toBeInTheDocument()
    expect(screen.getByText('as of 1 October 2026')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Evidence from live sources' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Inference by the judge model' })).toBeInTheDocument()
    expect(screen.getByText('Current pricing pages list $25 per month.')).toBeInTheDocument()
    expect(screen.getByText('Judge confidence: 85%')).toBeInTheDocument()
    expect(screen.getByText('Contradicts the claim')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Open full report' })).toHaveAttribute('href', '/reports/r1')
  })

  it('does not link a javascript URL', () => {
    const claim = staleClaim()
    claim.evidence = [{ ...claim.evidence[0], url: 'javascript:alert(1)', title: 'Bad source' }]
    inRouter(<ExplainCard claim={claim} report={completeReport()} />)
    expect(screen.getByText('Bad source').closest('a')).toBeNull()
  })

  it('opens an https source in a new tab without a referrer', () => {
    inRouter(<ExplainCard claim={staleClaim()} report={completeReport()} />)
    const link = screen.getByRole('link', { name: 'Example Cloud pricing' })
    expect(link).toHaveAttribute('target', '_blank')
    expect(link).toHaveAttribute('rel', 'noopener noreferrer')
  })

  it('renders HTML in a snippet as text', () => {
    const claim = staleClaim()
    claim.evidence = [{ ...claim.evidence[0], snippet: '<script>alert(1)</script>' }]
    const { container } = inRouter(<ExplainCard claim={claim} report={completeReport()} />)
    expect(screen.getByText('<script>alert(1)</script>')).toBeInTheDocument()
    expect(container.querySelector('script')).toBeNull()
  })

  it('shows the score parts of a current claim', () => {
    inRouter(<ExplainCard claim={freshClaim()} report={completeReport()} />)
    expect(screen.getByText('Evidence recency 90%')).toBeInTheDocument()
    expect(screen.getByText('Source authority 100%')).toBeInTheDocument()
    expect(screen.getByText('Agreement between sources 67%')).toBeInTheDocument()
  })

  it('explains why there is no curve for an outdated claim', () => {
    inRouter(<ExplainCard claim={staleClaim()} report={completeReport()} />)
    expect(screen.getByText('This claim is already outdated, so no decay curve is shown.')).toBeInTheDocument()
  })
})

describe('ClaimText explain cards', () => {
  afterEach(() => vi.useRealTimers())

  function renderAnswer() {
    const report = completeReport()
    inRouter(<ClaimText text={answer} claims={report.claims} reportStatus="complete" report={report} />)
    return screen.getByRole('button', { name: 'Example Cloud Pro costs $20 per month' })
  }

  it('opens on a resting pointer without moving focus and closes after the pointer leaves', () => {
    vi.useFakeTimers()
    const claim = renderAnswer()
    fireEvent.pointerOver(claim)
    act(() => vi.advanceTimersByTime(120))
    expect(screen.getByRole('dialog')).toHaveTextContent('Current value: $25 per month')
    expect(document.activeElement).toBe(document.body)
    fireEvent.pointerOut(claim)
    act(() => vi.advanceTimersByTime(150))
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('stays open after a click and closes on Escape with focus back on the claim', () => {
    vi.useFakeTimers()
    const claim = renderAnswer()
    fireEvent.click(claim)
    fireEvent.pointerOut(claim)
    act(() => vi.advanceTimersByTime(500))
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    fireEvent.keyDown(document.activeElement ?? document.body, { key: 'Escape' })
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(claim).toHaveFocus()
  })

  it('opens with Enter and dims the rest of the answer', () => {
    const claim = renderAnswer()
    claim.focus()
    fireEvent.keyDown(claim, { key: 'Enter' })
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    const other = screen.getByRole('button', { name: 'its current version is 4.2' })
    expect(other).toHaveAttribute('data-dimmed')
    expect(claim).not.toHaveAttribute('data-dimmed')
    const textSegments = screen.getByTestId('answer-text').querySelectorAll(':scope > span:not([role])')
    expect(textSegments[0]).toHaveTextContent(', and')
    expect(textSegments[0]).toHaveAttribute('data-dimmed')
  })
})

describe('DecayCurve', () => {
  it('moves the scrubber with the arrow keys', () => {
    const model = decayModel(freshClaim(), completeReport())!
    const today = new Date('2026-11-05T00:00:00Z')
    render(<DecayCurve model={model} today={today} />)
    const slider = screen.getByRole('slider')
    slider.focus()
    fireEvent.keyDown(slider, { key: 'ArrowRight' })
    const week = new Date(today.getTime() + 7 * day)
    expect(slider).toHaveAttribute('aria-valuetext', scrubSentence(week, probabilityAt(model, week)))
    fireEvent.keyDown(slider, { key: 'ArrowRight', shiftKey: true })
    const later = new Date(today.getTime() + 37 * day)
    expect(slider).toHaveAttribute('aria-valuetext', scrubSentence(later, probabilityAt(model, later)))
  })
})
