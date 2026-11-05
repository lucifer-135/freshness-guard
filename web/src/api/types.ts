// The plugin API's JSON contract. Names and keys follow internal/report/report.go
// and the event payloads published in internal/engine/pipeline.go.

export type Verdict = 'FRESH' | 'STALE' | 'WRONG' | 'UNVERIFIABLE' | 'NOT_CHECKED'
export type ReportStatus = 'pending' | 'complete' | 'error'
export type Stance = 'supports' | 'contradicts' | 'neutral'
export type Category = 'pricing' | 'version' | 'leadership' | 'law' | 'statistic' | 'event' | 'feature' | 'static'

export interface ScoreParts {
  recency: number
  authority: number
  agreement: number
}

export interface Evidence {
  ref: string
  url: string
  title: string
  domain: string
  published_date: string | null
  snippet: string
  authority: number
  stance: Stance
}

export interface TimelineEntry {
  value: string
  valid_from: string
  source_url: string
}

export interface Claim {
  id: string
  quote: string
  // UTF-16 code-unit offsets into the answer (JavaScript string indices), or -1.
  span_start: number
  span_end: number
  entity: string
  attribute: string
  value: string
  category: Category
  temporal_marker: string
  search_query: string
  p_changed: number
  checked: boolean
  cache_hit: boolean
  verdict: Verdict
  current_value: string
  as_of: string
  score: number | null
  score_parts: ScoreParts | null
  half_life_days: number
  confidence: number
  rationale: string
  fact_id: number | null
  evidence: Evidence[]
  timeline: TimelineEntry[]
}

export interface Report {
  id: string
  created_at: string
  model: string
  provider: string
  mode: string
  status: ReportStatus
  reason?: string
  prompt: string
  answer_text: string
  score: number | null
  stale_count: number
  checked_count: number
  total_claims: number
  latency_ms: number
  credits_used: number
  policy: string
  healed_text: string
  healed_count: number
  blocked: boolean
  model_age_days: number
  claims: Claim[]
}

export type Mode = 'async' | 'sync' | 'off'
export type Policy = 'annotate' | 'heal' | 'block'
export type FreshnessStatus = 'pending' | 'complete' | 'skipped' | 'error'

export interface Model {
  id: string
  provider: string
}

export interface PluginStatus {
  gateway: { reachable: boolean; models: number }
  credits: { used: number; budget: number }
  settings: { default_mode: Mode; default_policy: Policy; sync_timeout_ms: number }
}

interface EventBase {
  id: number
  report_id: string
  at: string
}

// ReportEvent is an event that changes one report.
export type ReportEvent =
  | (EventBase & { type: 'report.started'; data: { model: string; provider: string; mode: string; policy: string } })
  | (EventBase & { type: 'report.claims'; data: { claims: Claim[] } })
  | (EventBase & { type: 'claim.verdict'; data: { claim: Claim } })
  | (EventBase & {
      type: 'report.complete'
      data: {
        status: ReportStatus
        score: number | null
        stale_count: number
        checked_count: number
        total_claims: number
        healed_count: number
        blocked: boolean
        latency_ms: number
        credits_used: number
      }
    })
  | (EventBase & { type: 'report.error'; data: { reason: string } })

// CachePurgedEvent says cached answers were removed from the semantic cache.
// Its report_id is the report that caused it.
export type CachePurgedEvent = EventBase & { type: 'cache.purged'; data: { cache_ids: string[]; count: number } }

export type LiveEvent = ReportEvent | CachePurgedEvent

export interface Counts {
  checked: number
  outdated: number
  incorrect: number
}

export interface LeaderboardRow {
  model: string
  provider: string
  total: Counts
  cells: Partial<Record<Category, Counts>>
}

export interface Leaderboard {
  window_days: number
  since: string
  min_sample: number
  rows: LeaderboardRow[]
}

export interface Summary {
  window_hours: number
  answers: number
  checked_claims: number
  outdated_claims: number
  incorrect_claims: number
  cache_purges: number
  cache_purge_enabled: boolean
  credits: { used: number; budget: number }
}

export type Metric = 'outdated' | 'incorrect'
export type StatsWindow = '7d' | '30d'
