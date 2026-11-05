import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  ApiError,
  apiFetch,
  apiJSON,
  clearToken,
  getLeaderboard,
  getReport,
  getSummary,
  getToken,
  setToken,
  setUnauthorizedHandler,
} from './client'
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

  it('requests the leaderboard for the chosen window with the token', async () => {
    setToken('t1')
    const body = { window_days: 30, since: '2026-10-06T00:00:00Z', min_sample: 10, rows: [] }
    fake = fakeFetch({ 'GET /freshness/leaderboard': () => json(200, body) })
    expect(await getLeaderboard('30d')).toEqual(body)
    expect(fake.requests[0].url.search).toBe('?window=30d')
    expect(fake.requests[0].headers.get('Authorization')).toBe('Bearer t1')
  })

  it('requests the summary', async () => {
    setToken('t1')
    const body = {
      window_hours: 24,
      answers: 3,
      checked_claims: 6,
      outdated_claims: 2,
      incorrect_claims: 0,
      cache_purges: 1,
      cache_purge_enabled: true,
      credits: { used: 4, budget: 200 },
    }
    fake = fakeFetch({ 'GET /freshness/summary': () => json(200, body) })
    expect(await getSummary()).toEqual(body)
  })
})
