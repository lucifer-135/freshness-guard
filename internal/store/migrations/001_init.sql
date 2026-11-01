CREATE TABLE reports (
  id TEXT PRIMARY KEY,
  created_at TEXT NOT NULL,
  model TEXT,
  provider TEXT,
  virtual_key TEXT,
  mode TEXT,
  status TEXT NOT NULL,
  reason TEXT,
  prompt TEXT,
  answer_text TEXT,
  score INTEGER,
  stale_count INTEGER,
  checked_count INTEGER,
  total_claims INTEGER,
  latency_ms INTEGER,
  credits_used INTEGER
);
CREATE INDEX reports_created ON reports(created_at);
CREATE INDEX reports_model_created ON reports(model, created_at);

CREATE TABLE facts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  normalized_key TEXT NOT NULL UNIQUE,
  entity TEXT,
  attribute TEXT,
  category TEXT,
  current_value TEXT,
  as_of TEXT,
  source_url TEXT,
  last_verified_at TEXT NOT NULL
);

CREATE TABLE fact_timeline (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  fact_id INTEGER NOT NULL REFERENCES facts(id) ON DELETE CASCADE,
  value TEXT NOT NULL,
  valid_from TEXT NOT NULL,
  source_url TEXT,
  UNIQUE(fact_id, value, valid_from)
);

CREATE TABLE claims (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  report_id TEXT NOT NULL REFERENCES reports(id) ON DELETE CASCADE,
  position INTEGER NOT NULL,
  claim_ref TEXT NOT NULL,
  quote TEXT,
  span_start INTEGER,
  span_end INTEGER,
  entity TEXT,
  attribute TEXT,
  value TEXT,
  category TEXT,
  temporal_marker TEXT,
  search_query TEXT,
  p_changed REAL,
  checked INTEGER,
  cache_hit INTEGER,
  verdict TEXT,
  current_value TEXT,
  as_of TEXT,
  score INTEGER,
  confidence REAL,
  rationale TEXT,
  fact_id INTEGER REFERENCES facts(id)
);
CREATE INDEX claims_report ON claims(report_id);

CREATE TABLE evidence (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  claim_id INTEGER NOT NULL REFERENCES claims(id) ON DELETE CASCADE,
  position INTEGER NOT NULL,
  ref TEXT,
  url TEXT,
  title TEXT,
  domain TEXT,
  published_date TEXT,
  snippet TEXT,
  authority REAL,
  stance TEXT
);

CREATE TABLE credit_ledger (
  day TEXT PRIMARY KEY,
  used INTEGER NOT NULL,
  budget INTEGER NOT NULL
);
