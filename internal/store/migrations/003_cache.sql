ALTER TABLE reports ADD COLUMN cache_id TEXT NOT NULL DEFAULT '';
ALTER TABLE reports ADD COLUMN cache_hit INTEGER NOT NULL DEFAULT 0;

-- One row per fact mentioned by an answer that is believed to be in the
-- semantic cache. value_key is facts.NormalizeValue of the fact's value when
-- the answer was linked; rows are deleted when the entry is purged.
CREATE TABLE cache_links (
  cache_id TEXT NOT NULL,
  fact_id INTEGER NOT NULL REFERENCES facts(id) ON DELETE CASCADE,
  value_key TEXT NOT NULL,
  report_id TEXT NOT NULL,
  linked_at TEXT NOT NULL,
  PRIMARY KEY (cache_id, fact_id)
);
CREATE INDEX cache_links_fact ON cache_links(fact_id);

CREATE TABLE cache_purges (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  cache_id TEXT NOT NULL,
  report_id TEXT NOT NULL,
  fact_id INTEGER,
  reason TEXT NOT NULL,
  purged_at TEXT NOT NULL
);
CREATE INDEX cache_purges_at ON cache_purges(purged_at);
