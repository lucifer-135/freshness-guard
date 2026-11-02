import type { Claim, Model, PluginStatus, Report } from './types'

const TOKEN_KEY = 'fg.token'

export class ApiError extends Error {
  readonly status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

// The token is kept in memory as well, so the app still works for the session
// when the browser blocks storage.
let memoryToken: string | null = null
let onUnauthorized: (() => void) | null = null

export function getToken(): string | null {
  try {
    return localStorage.getItem(TOKEN_KEY) ?? memoryToken
  } catch {
    return memoryToken
  }
}

export function setToken(token: string): void {
  memoryToken = token
  try {
    localStorage.setItem(TOKEN_KEY, token)
  } catch {
    // Storage is blocked; the in-memory token lasts for this session.
  }
}

export function clearToken(): void {
  memoryToken = null
  try {
    localStorage.removeItem(TOKEN_KEY)
  } catch {
    // Storage is blocked; nothing was stored.
  }
}

export function setUnauthorizedHandler(fn: (() => void) | null): void {
  onUnauthorized = fn
}

async function errorMessage(resp: Response): Promise<string> {
  try {
    const body: unknown = await resp.json()
    if (body && typeof body === 'object' && 'error' in body && typeof body.error === 'string') return body.error
  } catch {
    // Not JSON; fall through to the generic message.
  }
  return `The plugin API answered with HTTP ${resp.status}.`
}

// apiFetch adds the bearer token. A 401 calls the unauthorized handler and
// throws; every other status is returned to the caller.
export async function apiFetch(path: string, init: RequestInit = {}): Promise<Response> {
  const headers = new Headers(init.headers)
  const token = getToken()
  if (token) headers.set('Authorization', `Bearer ${token}`)
  const resp = await fetch(path, { ...init, headers })
  if (resp.status === 401) {
    onUnauthorized?.()
    throw new ApiError(401, await errorMessage(resp))
  }
  return resp
}

export async function apiJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const resp = await apiFetch(path, init)
  if (!resp.ok) throw new ApiError(resp.status, await errorMessage(resp))
  return (await resp.json()) as T
}

export function normalizeClaim(claim: Partial<Claim> & { id: string }): Claim {
  return {
    quote: '',
    span_start: -1,
    span_end: -1,
    entity: '',
    attribute: '',
    value: '',
    category: 'static',
    temporal_marker: '',
    search_query: '',
    p_changed: 0,
    checked: false,
    cache_hit: false,
    verdict: 'NOT_CHECKED',
    current_value: '',
    as_of: '',
    score: null,
    score_parts: null,
    half_life_days: 0,
    confidence: 0,
    rationale: '',
    fact_id: null,
    ...claim,
    // Go encodes empty lists as null.
    evidence: claim.evidence ?? [],
    timeline: claim.timeline ?? [],
  }
}

// normalizeReport fills every missing field with its zero value, so a pending
// placeholder ({id, status, claims}) has the same shape as a full report.
export function normalizeReport(json: Partial<Report> & { id: string }): Report {
  return {
    created_at: '',
    model: '',
    provider: '',
    mode: '',
    status: 'pending',
    prompt: '',
    answer_text: '',
    score: null,
    stale_count: 0,
    checked_count: 0,
    total_claims: 0,
    latency_ms: 0,
    credits_used: 0,
    policy: '',
    healed_text: '',
    healed_count: 0,
    blocked: false,
    model_age_days: 0,
    ...json,
    claims: (json.claims ?? []).map(normalizeClaim),
  }
}

export async function getReport(id: string): Promise<Report> {
  return normalizeReport(await apiJSON<Report>(`/freshness/reports/${encodeURIComponent(id)}`))
}

export async function listReports(params: { model?: string; limit?: number } = {}): Promise<Report[]> {
  const query = new URLSearchParams()
  if (params.model) query.set('model', params.model)
  if (params.limit) query.set('limit', String(params.limit))
  const suffix = query.size > 0 ? `?${query}` : ''
  const body = await apiJSON<{ reports: Report[] | null }>(`/freshness/reports${suffix}`)
  return (body.reports ?? []).map(normalizeReport)
}

// checkText returns the report: complete (HTTP 200) or still pending (HTTP 202).
export async function checkText(text: string): Promise<Report> {
  const report = await apiJSON<Report>('/freshness/check', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ text }),
  })
  return normalizeReport(report)
}

export async function getModels(): Promise<Model[]> {
  const body = await apiJSON<{ models: Model[] | null }>('/freshness/models')
  return body.models ?? []
}

export function getStatus(): Promise<PluginStatus> {
  return apiJSON<PluginStatus>('/freshness/status')
}
