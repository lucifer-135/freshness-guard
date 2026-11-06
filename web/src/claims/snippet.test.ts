import { describe, expect, it } from 'vitest'
import { plainSnippet } from './snippet'

describe('plainSnippet', () => {
  it('removes headings and bold markers from a search result', () => {
    expect(plainSnippet('### Python 3.14 **Python 3.14.10**, released')).toBe('Python 3.14 Python 3.14.10, released')
  })

  it('keeps the text of links and code', () => {
    expect(plainSnippet('[Docs](https://x.org) and `code`')).toBe('Docs and code')
  })

  it('returns nothing for text that is only Markdown syntax', () => {
    expect(plainSnippet('### ')).toBe('')
    expect(plainSnippet('')).toBe('')
  })

  it('collapses lines and runs of spaces into single spaces', () => {
    expect(plainSnippet('## Release\n\nPython   3.15.2\nreleased')).toBe('Release Python 3.15.2 released')
  })

  it('leaves HTML-looking text alone so it can be shown as text', () => {
    expect(plainSnippet('<script>alert(1)</script>')).toBe('<script>alert(1)</script>')
  })
})
