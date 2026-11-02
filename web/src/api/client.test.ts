import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError, apiFetch, apiJSON, clearToken, getReport, getToken, setToken, setUnauthorizedHandler } from './client'
import { fakeFetch, json, type FakeFetch } from '@/test/fakeFetch'

describe('plugin API client', () => {
  let fake: FakeFetch
  afterEach(() => {
    fake?.restore()
    setUnauthorizedHandler(null)
    vi.restoreAllMocks()
  })

  it('sends the bearer token', async () => {
    setToken('t1')
    fake = fakeFetch({ 'GET /freshness/status': () => json(200, {}) })
    await apiFetch('/freshness/status')
    expect(fake.requests[0].headers.get('Authorization')).toBe('Bearer t1')
  })

  it('calls the unauthorized handler once on 401 and throws', async () => {
    setToken('old')
    const handler = vi.fn()
    setUnauthorizedHandler(handler)
    fake = fakeFetch({ 'GET /freshness/status': () => json(401, { error: 'A valid bearer token is required.' }) })
    const error = await apiFetch('/freshness/status').catch((e: unknown) => e)
    expect(error).toBeInstanceOf(ApiError)
    expect((error as ApiError).status).toBe(401)
    expect(handler).toHaveBeenCalledTimes(1)
  })

  it('throws the API error message for other errors', async () => {
    setToken('t1')
    fake = fakeFetch({ 'GET /freshness/x': () => json(503, { error: 'The verification queue is full. Try again later.' }) })
    const error = await apiJSON('/freshness/x').catch((e: unknown) => e)
    expect(error).toBeInstanceOf(ApiError)
    expect((error as ApiError).status).toBe(503)
    expect((error as ApiError).message).toBe('The verification queue is full. Try again later.')
  })

  it('normalizes a pending report', async () => {
    setToken('t1')
    fake = fakeFetch({ 'GET /freshness/reports/r1': () => json(200, { id: 'r1', status: 'pending', claims: [] }) })
    const report = await getReport('r1')
    expect(report).toMatchObject({ id: 'r1', status: 'pending', answer_text: '', score: null, claims: [] })
  })

  it('normalizes null lists from Go', async () => {
    setToken('t1')
    fake = fakeFetch({
      'GET /freshness/reports/r2': () =>
        json(200, { id: 'r2', status: 'complete', claims: [{ id: 'c1', evidence: null, timeline: null }] }),
    })
    const report = await getReport('r2')
    expect(report.claims[0].evidence).toEqual([])
    expect(report.claims[0].timeline).toEqual([])
  })

  it('returns null when storage cannot be read', () => {
    clearToken()
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('blocked')
    })
    expect(getToken()).toBeNull()
  })
})
