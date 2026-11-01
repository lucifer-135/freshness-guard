package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	_ "github.com/mattn/go-sqlite3" // Registers the "sqlite3" driver; Bifrost links the same version.

	"github.com/lucifer-135/freshness-guard/internal/report"
)

//go:embed migrations/*.sql
var migrations embed.FS

// timeLayout has a fixed width, so stored times sort correctly as text.
const timeLayout = "2006-01-02T15:04:05.000000000Z07:00"

// SQLite is the Store implementation. Writes go through a single connection;
// reads use a small pool on the same WAL-mode database.
type SQLite struct {
	w *sql.DB
	r *sql.DB
}

var _ Store = (*SQLite)(nil)

// OpenSQLite opens or creates the database at path and applies migrations.
func OpenSQLite(path string) (*SQLite, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}
	dsn := "file:" + path + "?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on&_synchronous=NORMAL"

	w, err := sql.Open("sqlite3", dsn+"&_txlock=immediate")
	if err != nil {
		return nil, err
	}
	w.SetMaxOpenConns(1)
	if err := w.Ping(); err != nil {
		w.Close()
		return nil, fmt.Errorf("open database %s: %w", path, err)
	}
	if err := migrate(w); err != nil {
		w.Close()
		return nil, err
	}

	r, err := sql.Open("sqlite3", dsn)
	if err != nil {
		w.Close()
		return nil, err
	}
	r.SetMaxOpenConns(4)
	return &SQLite{w: w, r: r}, nil
}

func migrate(db *sql.DB) error {
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	names, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)
	for i := version; i < len(names); i++ {
		script, err := migrations.ReadFile(names[i])
		if err != nil {
			return err
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(script)); err != nil {
			tx.Rollback()
			return fmt.Errorf("apply %s: %w", names[i], err)
		}
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", i+1)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// Close closes both connection pools.
func (s *SQLite) Close() error {
	return errors.Join(s.r.Close(), s.w.Close())
}

func formatTime(t time.Time) string { return t.UTC().Format(timeLayout) }

func parseTime(s string) (time.Time, error) { return time.Parse(timeLayout, s) }

func nullInt(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullInt64(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullTime(p *time.Time) any {
	if p == nil {
		return nil
	}
	return formatTime(*p)
}

// SaveReport inserts or updates r and replaces its claims and evidence.
func (s *SQLite) SaveReport(ctx context.Context, r report.Report) error {
	tx, err := s.w.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `INSERT INTO reports (id, created_at, model, provider, virtual_key, mode, status, reason,
		prompt, answer_text, score, stale_count, checked_count, total_claims, latency_ms, credits_used)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET created_at = excluded.created_at, model = excluded.model,
		provider = excluded.provider, virtual_key = excluded.virtual_key, mode = excluded.mode, status = excluded.status,
		reason = excluded.reason, prompt = excluded.prompt, answer_text = excluded.answer_text, score = excluded.score,
		stale_count = excluded.stale_count, checked_count = excluded.checked_count, total_claims = excluded.total_claims,
		latency_ms = excluded.latency_ms, credits_used = excluded.credits_used`,
		r.ID, formatTime(r.CreatedAt), r.Model, r.Provider, r.VirtualKey, r.Mode, string(r.Status), r.Reason,
		r.Prompt, r.AnswerText, nullInt(r.Score), r.StaleCount, r.CheckedCount, r.TotalClaims, r.LatencyMS, r.CreditsUsed)
	if err != nil {
		return fmt.Errorf("save report: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM claims WHERE report_id = ?`, r.ID); err != nil {
		return fmt.Errorf("replace claims: %w", err)
	}

	for i, c := range r.Claims {
		res, err := tx.ExecContext(ctx, `INSERT INTO claims (report_id, position, claim_ref, quote, span_start, span_end,
			entity, attribute, value, category, temporal_marker, search_query, p_changed, checked, cache_hit, verdict,
			current_value, as_of, score, confidence, rationale, fact_id)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			r.ID, i, c.ID, c.Quote, c.SpanStart, c.SpanEnd, c.Entity, c.Attribute, c.Value, string(c.Category),
			c.TemporalMarker, c.SearchQuery, c.PChanged, c.Checked, c.CacheHit, string(c.Verdict), c.CurrentValue, c.AsOf,
			nullInt(c.Score), c.Confidence, c.Rationale, nullInt64(c.FactID))
		if err != nil {
			return fmt.Errorf("save claim %s: %w", c.ID, err)
		}
		claimID, err := res.LastInsertId()
		if err != nil {
			return err
		}
		for j, e := range c.Evidence {
			_, err := tx.ExecContext(ctx, `INSERT INTO evidence (claim_id, position, ref, url, title, domain,
				published_date, snippet, authority, stance) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				claimID, j, e.Ref, e.URL, e.Title, e.Domain, nullTime(e.PublishedDate), e.Snippet, e.Authority, string(e.Stance))
			if err != nil {
				return fmt.Errorf("save evidence for claim %s: %w", c.ID, err)
			}
		}
	}
	return tx.Commit()
}

const reportColumns = `id, created_at, model, provider, virtual_key, mode, status, reason, prompt, answer_text, score,
	stale_count, checked_count, total_claims, latency_ms, credits_used`

type scanner interface{ Scan(dest ...any) error }

func scanReport(row scanner) (report.Report, error) {
	var (
		r       report.Report
		created string
		status  string
		score   sql.NullInt64
	)
	err := row.Scan(&r.ID, &created, &r.Model, &r.Provider, &r.VirtualKey, &r.Mode, &status, &r.Reason, &r.Prompt,
		&r.AnswerText, &score, &r.StaleCount, &r.CheckedCount, &r.TotalClaims, &r.LatencyMS, &r.CreditsUsed)
	if err != nil {
		return r, err
	}
	if r.CreatedAt, err = parseTime(created); err != nil {
		return r, err
	}
	r.Status = report.Status(status)
	if score.Valid {
		v := int(score.Int64)
		r.Score = &v
	}
	return r, nil
}

// GetReport returns the report with its claims, evidence and fact timelines.
func (s *SQLite) GetReport(ctx context.Context, id string) (report.Report, error) {
	r, err := scanReport(s.r.QueryRowContext(ctx, `SELECT `+reportColumns+` FROM reports WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return report.Report{}, ErrNotFound
	}
	if err != nil {
		return report.Report{}, fmt.Errorf("load report: %w", err)
	}

	claimIDs, err := s.loadClaims(ctx, &r)
	if err != nil {
		return report.Report{}, err
	}
	if err := s.loadEvidence(ctx, &r, claimIDs); err != nil {
		return report.Report{}, err
	}
	for i := range r.Claims {
		if r.Claims[i].FactID == nil {
			continue
		}
		if r.Claims[i].Timeline, err = s.timeline(ctx, *r.Claims[i].FactID); err != nil {
			return report.Report{}, err
		}
	}
	return r, nil
}

// loadClaims fills r.Claims and returns a map from the claims' row IDs to
// their index in r.Claims.
func (s *SQLite) loadClaims(ctx context.Context, r *report.Report) (map[int64]int, error) {
	rows, err := s.r.QueryContext(ctx, `SELECT id, claim_ref, quote, span_start, span_end, entity, attribute, value,
		category, temporal_marker, search_query, p_changed, checked, cache_hit, verdict, current_value, as_of, score,
		confidence, rationale, fact_id FROM claims WHERE report_id = ? ORDER BY position`, r.ID)
	if err != nil {
		return nil, fmt.Errorf("load claims: %w", err)
	}
	defer rows.Close()

	r.Claims = []report.Claim{}
	index := map[int64]int{}
	for rows.Next() {
		var (
			rowID             int64
			c                 report.Claim
			category, verdict string
			score             sql.NullInt64
			factID            sql.NullInt64
		)
		err := rows.Scan(&rowID, &c.ID, &c.Quote, &c.SpanStart, &c.SpanEnd, &c.Entity, &c.Attribute, &c.Value, &category,
			&c.TemporalMarker, &c.SearchQuery, &c.PChanged, &c.Checked, &c.CacheHit, &verdict, &c.CurrentValue, &c.AsOf,
			&score, &c.Confidence, &c.Rationale, &factID)
		if err != nil {
			return nil, fmt.Errorf("read claim: %w", err)
		}
		c.Category = report.Category(category)
		c.Verdict = report.Verdict(verdict)
		if score.Valid {
			v := int(score.Int64)
			c.Score = &v
		}
		if factID.Valid {
			v := factID.Int64
			c.FactID = &v
		}
		c.Evidence = []report.Evidence{}
		c.Timeline = []report.TimelineEntry{}
		index[rowID] = len(r.Claims)
		r.Claims = append(r.Claims, c)
	}
	return index, rows.Err()
}

func (s *SQLite) loadEvidence(ctx context.Context, r *report.Report, claimIDs map[int64]int) error {
	rows, err := s.r.QueryContext(ctx, `SELECT e.claim_id, e.ref, e.url, e.title, e.domain, e.published_date, e.snippet,
		e.authority, e.stance FROM evidence e JOIN claims c ON c.id = e.claim_id
		WHERE c.report_id = ? ORDER BY e.claim_id, e.position`, r.ID)
	if err != nil {
		return fmt.Errorf("load evidence: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			claimID   int64
			e         report.Evidence
			published sql.NullString
			stance    string
		)
		if err := rows.Scan(&claimID, &e.Ref, &e.URL, &e.Title, &e.Domain, &published, &e.Snippet, &e.Authority, &stance); err != nil {
			return fmt.Errorf("read evidence: %w", err)
		}
		if published.Valid {
			t, err := parseTime(published.String)
			if err != nil {
				return err
			}
			e.PublishedDate = &t
		}
		e.Stance = report.Stance(stance)
		i, ok := claimIDs[claimID]
		if !ok {
			continue
		}
		r.Claims[i].Evidence = append(r.Claims[i].Evidence, e)
	}
	return rows.Err()
}

func (s *SQLite) timeline(ctx context.Context, factID int64) ([]report.TimelineEntry, error) {
	rows, err := s.r.QueryContext(ctx, `SELECT value, valid_from, source_url FROM fact_timeline
		WHERE fact_id = ? ORDER BY valid_from, id`, factID)
	if err != nil {
		return nil, fmt.Errorf("load timeline: %w", err)
	}
	defer rows.Close()
	entries := []report.TimelineEntry{}
	for rows.Next() {
		var e report.TimelineEntry
		var source sql.NullString
		if err := rows.Scan(&e.Value, &e.ValidFrom, &source); err != nil {
			return nil, err
		}
		e.SourceURL = source.String
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// ListReports returns reports newest first, without claims.
func (s *SQLite) ListReports(ctx context.Context, f ListFilter) ([]report.Report, error) {
	since := ""
	if !f.Since.IsZero() {
		since = formatTime(f.Since)
	}
	rows, err := s.r.QueryContext(ctx, `SELECT `+reportColumns+` FROM reports
		WHERE (? = '' OR model = ?) AND (? = '' OR created_at >= ?)
		ORDER BY created_at DESC LIMIT ?`, f.Model, f.Model, since, since, EffectiveLimit(f.Limit))
	if err != nil {
		return nil, fmt.Errorf("list reports: %w", err)
	}
	defer rows.Close()
	reports := []report.Report{}
	for rows.Next() {
		r, err := scanReport(rows)
		if err != nil {
			return nil, fmt.Errorf("read report: %w", err)
		}
		reports = append(reports, r)
	}
	return reports, rows.Err()
}

// LookupFact returns the fact with the given normalized key.
func (s *SQLite) LookupFact(ctx context.Context, key string) (report.Fact, error) {
	var (
		f                  report.Fact
		category, verified string
	)
	err := s.r.QueryRowContext(ctx, `SELECT id, normalized_key, entity, attribute, category, current_value, as_of,
		source_url, last_verified_at FROM facts WHERE normalized_key = ?`, key).
		Scan(&f.ID, &f.Key, &f.Entity, &f.Attribute, &category, &f.CurrentValue, &f.AsOf, &f.SourceURL, &verified)
	if errors.Is(err, sql.ErrNoRows) {
		return report.Fact{}, ErrNotFound
	}
	if err != nil {
		return report.Fact{}, fmt.Errorf("load fact: %w", err)
	}
	f.Category = report.Category(category)
	if f.LastVerifiedAt, err = parseTime(verified); err != nil {
		return report.Fact{}, err
	}
	return f, nil
}

// UpsertFact inserts or updates the fact with f.Key and returns its ID.
func (s *SQLite) UpsertFact(ctx context.Context, f report.Fact) (int64, error) {
	var id int64
	err := s.w.QueryRowContext(ctx, `INSERT INTO facts (normalized_key, entity, attribute, category, current_value,
		as_of, source_url, last_verified_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(normalized_key) DO UPDATE SET entity = excluded.entity, attribute = excluded.attribute,
		category = excluded.category, current_value = excluded.current_value, as_of = excluded.as_of,
		source_url = excluded.source_url, last_verified_at = excluded.last_verified_at
		RETURNING id`,
		f.Key, f.Entity, f.Attribute, string(f.Category), f.CurrentValue, f.AsOf, f.SourceURL, formatTime(f.LastVerifiedAt)).
		Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("save fact: %w", err)
	}
	return id, nil
}

// AddTimeline stores timeline entries for a fact; duplicates are ignored.
func (s *SQLite) AddTimeline(ctx context.Context, factID int64, entries []report.TimelineEntry) error {
	tx, err := s.w.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, e := range entries {
		_, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO fact_timeline (fact_id, value, valid_from, source_url)
			VALUES (?, ?, ?, ?)`, factID, e.Value, e.ValidFrom, e.SourceURL)
		if err != nil {
			return fmt.Errorf("save timeline entry: %w", err)
		}
	}
	return tx.Commit()
}

// CreditsUsed returns the Tavily credits used on day (YYYY-MM-DD).
func (s *SQLite) CreditsUsed(ctx context.Context, day string) (int, error) {
	var used int
	err := s.r.QueryRowContext(ctx, `SELECT used FROM credit_ledger WHERE day = ?`, day).Scan(&used)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return used, err
}

// AddCredits adds n credits to day's total and records the day's budget.
func (s *SQLite) AddCredits(ctx context.Context, day string, n, budget int) error {
	_, err := s.w.ExecContext(ctx, `INSERT INTO credit_ledger (day, used, budget) VALUES (?, ?, ?)
		ON CONFLICT(day) DO UPDATE SET used = used + excluded.used, budget = excluded.budget`, day, n, budget)
	return err
}
