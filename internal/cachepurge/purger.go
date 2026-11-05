package cachepurge

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/lucifer-135/freshness-guard/internal/events"
	"github.com/lucifer-135/freshness-guard/internal/facts"
	"github.com/lucifer-135/freshness-guard/internal/report"
	"github.com/lucifer-135/freshness-guard/internal/store"
)

// Purge reasons stored with each removed entry.
const (
	ReasonOutdatedAnswer = "outdated_answer"
	ReasonFactChanged    = "fact_changed"
)

// Store keeps the links between cached answers and facts, and the purge log;
// *store.SQLite implements it.
type Store interface {
	LinkCache(ctx context.Context, cacheID, reportID string, links []store.CacheLink, at time.Time) error
	CacheEntriesForFact(ctx context.Context, factID int64, exceptValueKey string) ([]string, error)
	RecordPurge(ctx context.Context, p store.CachePurge) error
}

// Clearer removes one semantic cache entry; *Client implements it.
type Clearer interface {
	Clear(ctx context.Context, cacheID string) error
}

// Purger keeps outdated answers out of the semantic cache (spec 9.3).
type Purger struct {
	Store   Store
	Bifrost Clearer
	Events  events.Publisher // nil means no events
	Now     func() time.Time
	Logger  *slog.Logger
}

// Finish applies the purge rules to a finished report:
//   - An answer with an outdated or incorrect claim is removed from the cache;
//     any other cached answer is linked to the facts it mentions.
//   - A claim that refreshed its fact removes every cached answer linked to
//     that fact under a different value.
//
// A removal that fails is logged and leaves the links, so a later report can
// try again. Finish never returns an error: the answer is already delivered.
func (p *Purger) Finish(ctx context.Context, r report.Report) {
	if r.Status != report.StatusComplete {
		return
	}
	run := &purgeRun{Purger: p, ctx: ctx, reportID: r.ID, seen: map[string]bool{}}

	if r.CacheID != "" {
		if hasOutdatedClaim(r.Claims) {
			run.remove(r.CacheID, ReasonOutdatedAnswer, nil)
		} else if links := linksFor(r.Claims); len(links) > 0 {
			// Linking first means the claim loop below never sees this answer's
			// own entry as a stale link.
			if err := p.Store.LinkCache(ctx, r.CacheID, r.ID, links, p.Now()); err != nil {
				p.Logger.Error("A cached answer could not be linked to its facts.", "report", r.ID, "cache_id", r.CacheID, "error", err)
			}
		}
	}

	for _, c := range r.Claims {
		// A verdict-cache hit leaves the fact as it was, so nothing changed.
		if !c.Checked || c.CacheHit || c.FactID == nil {
			continue
		}
		value := linkValue(c)
		if value == "" {
			continue
		}
		ids, err := p.Store.CacheEntriesForFact(ctx, *c.FactID, facts.NormalizeValue(value))
		if err != nil {
			p.Logger.Error("The cached answers linked to a fact could not be listed.", "report", r.ID, "fact", *c.FactID, "error", err)
			continue
		}
		for _, id := range ids {
			run.remove(id, ReasonFactChanged, c.FactID)
		}
	}

	if len(run.removed) > 0 && p.Events != nil {
		p.Events.Publish(events.Event{
			Type: events.TypeCachePurged, ReportID: r.ID, At: p.Now(),
			Data: map[string]any{"cache_ids": run.removed, "count": len(run.removed)},
		})
	}
}

// purgeRun is the state of one Finish call.
type purgeRun struct {
	*Purger
	ctx      context.Context
	reportID string
	seen     map[string]bool
	removed  []string
}

// remove clears one entry, at most once per report.
func (r *purgeRun) remove(cacheID, reason string, factID *int64) {
	if r.seen[cacheID] {
		return
	}
	r.seen[cacheID] = true
	if err := r.Bifrost.Clear(r.ctx, cacheID); err != nil {
		attrs := []any{"report", r.reportID, "cache_id", cacheID, "error", err}
		var clearErr *ClearError
		if errors.As(err, &clearErr) {
			attrs = append(attrs, "status", clearErr.Status)
		}
		r.Logger.Warn("A cached answer could not be removed from the semantic cache.", attrs...)
		return
	}
	purge := store.CachePurge{CacheID: cacheID, ReportID: r.reportID, FactID: factID, Reason: reason, At: r.Now()}
	if err := r.Store.RecordPurge(r.ctx, purge); err != nil {
		r.Logger.Error("A cache purge could not be recorded.", "report", r.reportID, "cache_id", cacheID, "error", err)
	}
	r.removed = append(r.removed, cacheID)
}

func hasOutdatedClaim(claims []report.Claim) bool {
	for _, c := range claims {
		if c.Verdict == report.VerdictStale || c.Verdict == report.VerdictWrong {
			return true
		}
	}
	return false
}

// linkValue is the value the claim established for its fact: the current
// value, or the claimed value of a FRESH claim without one. It follows
// facts.Cache.Record, which stores the same value.
func linkValue(c report.Claim) string {
	switch c.Verdict {
	case report.VerdictFresh, report.VerdictStale, report.VerdictWrong:
	default:
		return ""
	}
	if c.CurrentValue == "" && c.Verdict == report.VerdictFresh {
		return c.Value
	}
	return c.CurrentValue
}

func linksFor(claims []report.Claim) []store.CacheLink {
	var links []store.CacheLink
	for _, c := range claims {
		if c.FactID == nil {
			continue
		}
		if value := linkValue(c); value != "" {
			links = append(links, store.CacheLink{FactID: *c.FactID, ValueKey: facts.NormalizeValue(value)})
		}
	}
	return links
}
