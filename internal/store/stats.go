package store

import (
	"context"
	"fmt"
	"time"

	"github.com/lucifer-135/freshness-guard/internal/report"
)

// LeaderboardCell counts the checked claims of one model in one category.
type LeaderboardCell struct {
	Model     string
	Provider  string
	Category  report.Category
	Checked   int
	Outdated  int // claims judged STALE
	Incorrect int // claims judged WRONG
}

// SummaryCounts are the totals the dashboard's summary row shows.
type SummaryCounts struct {
	Answers         int
	CheckedClaims   int
	OutdatedClaims  int
	IncorrectClaims int
	CachePurges     int
}

// Leaderboard counts the checked claims of finished reports created at or
// after since, per model and category. Only answers from models count: pasted
// text checks have no model, and a pending report has no final verdicts.
func (s *SQLite) Leaderboard(ctx context.Context, since time.Time) ([]LeaderboardCell, error) {
	rows, err := s.r.QueryContext(ctx, `SELECT r.model, r.provider, c.category, COUNT(*),
			COALESCE(SUM(c.verdict = 'STALE'), 0), COALESCE(SUM(c.verdict = 'WRONG'), 0)
		FROM claims c JOIN reports r ON r.id = c.report_id
		WHERE c.checked = 1 AND r.status = 'complete' AND r.mode IN ('async', 'sync')
			AND r.model <> '' AND r.created_at >= ?
		GROUP BY r.model, r.provider, c.category`, formatTime(since))
	if err != nil {
		return nil, fmt.Errorf("count claims per model: %w", err)
	}
	defer rows.Close()
	var cells []LeaderboardCell
	for rows.Next() {
		var c LeaderboardCell
		var category string
		if err := rows.Scan(&c.Model, &c.Provider, &category, &c.Checked, &c.Outdated, &c.Incorrect); err != nil {
			return nil, err
		}
		c.Category = report.Category(category)
		cells = append(cells, c)
	}
	return cells, rows.Err()
}

// Summary counts finished reports of every mode created at or after since, the
// checked claims in them, and the cache entries removed in the same period.
func (s *SQLite) Summary(ctx context.Context, since time.Time) (SummaryCounts, error) {
	var out SummaryCounts
	from := formatTime(since)
	if err := s.r.QueryRowContext(ctx, `SELECT COUNT(*) FROM reports WHERE status = 'complete' AND created_at >= ?`, from).
		Scan(&out.Answers); err != nil {
		return out, fmt.Errorf("count answers: %w", err)
	}
	if err := s.r.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(c.verdict = 'STALE'), 0), COALESCE(SUM(c.verdict = 'WRONG'), 0)
		FROM claims c JOIN reports r ON r.id = c.report_id
		WHERE c.checked = 1 AND r.status = 'complete' AND r.created_at >= ?`, from).
		Scan(&out.CheckedClaims, &out.OutdatedClaims, &out.IncorrectClaims); err != nil {
		return out, fmt.Errorf("count claims: %w", err)
	}
	purges, err := s.CountPurges(ctx, since)
	if err != nil {
		return out, fmt.Errorf("count cache purges: %w", err)
	}
	out.CachePurges = purges
	return out, nil
}
