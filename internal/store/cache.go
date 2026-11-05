package store

import (
	"context"
	"fmt"
	"time"
)

// CacheLink ties a cached answer to a fact it mentions. ValueKey is the
// normalized fact value the answer stated when it was linked.
type CacheLink struct {
	FactID   int64
	ValueKey string
}

// CachePurge records one semantic cache entry the plugin removed. FactID is
// nil when the answer itself was outdated.
type CachePurge struct {
	CacheID  string
	ReportID string
	FactID   *int64
	Reason   string
	At       time.Time
}

// LinkCache links the cached answer cacheID to its facts. Linking the same
// entry and fact again replaces the earlier link.
func (s *SQLite) LinkCache(ctx context.Context, cacheID, reportID string, links []CacheLink, at time.Time) error {
	tx, err := s.w.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, l := range links {
		_, err := tx.ExecContext(ctx, `INSERT INTO cache_links (cache_id, fact_id, value_key, report_id, linked_at)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(cache_id, fact_id) DO UPDATE SET value_key = excluded.value_key,
			report_id = excluded.report_id, linked_at = excluded.linked_at`,
			cacheID, l.FactID, l.ValueKey, reportID, formatTime(at))
		if err != nil {
			return fmt.Errorf("link cache entry %s to fact %d: %w", cacheID, l.FactID, err)
		}
	}
	return tx.Commit()
}

// CacheEntriesForFact returns the IDs of cached answers linked to the fact
// with a value other than exceptValueKey, in ascending order.
func (s *SQLite) CacheEntriesForFact(ctx context.Context, factID int64, exceptValueKey string) ([]string, error) {
	rows, err := s.r.QueryContext(ctx, `SELECT DISTINCT cache_id FROM cache_links
		WHERE fact_id = ? AND value_key <> ? ORDER BY cache_id`, factID, exceptValueKey)
	if err != nil {
		return nil, fmt.Errorf("list cache entries for fact %d: %w", factID, err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// RecordPurge stores the purge and deletes the entry's links in one transaction.
func (s *SQLite) RecordPurge(ctx context.Context, p CachePurge) error {
	tx, err := s.w.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM cache_links WHERE cache_id = ?`, p.CacheID); err != nil {
		return fmt.Errorf("delete cache links of %s: %w", p.CacheID, err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO cache_purges (cache_id, report_id, fact_id, reason, purged_at)
		VALUES (?, ?, ?, ?, ?)`, p.CacheID, p.ReportID, nullInt64(p.FactID), p.Reason, formatTime(p.At)); err != nil {
		return fmt.Errorf("record purge of %s: %w", p.CacheID, err)
	}
	return tx.Commit()
}

// CountPurges returns how many cache entries were removed at or after since.
func (s *SQLite) CountPurges(ctx context.Context, since time.Time) (int, error) {
	var n int
	err := s.r.QueryRowContext(ctx, `SELECT COUNT(*) FROM cache_purges WHERE purged_at >= ?`, formatTime(since)).Scan(&n)
	return n, err
}
