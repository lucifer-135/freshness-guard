// Package facts keeps the latest verified value of each (entity, attribute)
// pair. The facts table doubles as the verdict cache: a recent fact is given to
// the judge as evidence instead of a new Tavily search.
package facts

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/lucifer-135/freshness-guard/internal/report"
	"github.com/lucifer-135/freshness-guard/internal/triage"
)

// Store is the fact storage the cache needs.
type Store interface {
	LookupFact(ctx context.Context, key string) (report.Fact, error)
	UpsertFact(ctx context.Context, f report.Fact) (int64, error)
	AddTimeline(ctx context.Context, factID int64, entries []report.TimelineEntry) error
}

// NormalizeKey returns the cache key for an (entity, attribute) pair: each part
// lowercased with every run of non-alphanumeric characters turned into one
// space, joined with "|".
func NormalizeKey(entity, attribute string) string {
	return normalize(entity) + "|" + normalize(attribute)
}

func normalize(s string) string {
	return strings.Join(strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}), " ")
}

// Cache looks up and records verified facts.
type Cache struct {
	Store Store
	Now   func() time.Time
}

// Fresh returns the stored fact when it was verified less than one half-life
// of cat ago. Categories without a half-life never use the cache.
func (c *Cache) Fresh(ctx context.Context, entity, attribute string, cat report.Category) (report.Fact, bool) {
	hl, ok := triage.HalfLife(cat)
	if !ok {
		return report.Fact{}, false
	}
	f, err := c.Store.LookupFact(ctx, NormalizeKey(entity, attribute))
	if err != nil {
		return report.Fact{}, false
	}
	if c.Now().Sub(f.LastVerifiedAt) >= hl {
		return report.Fact{}, false
	}
	return f, true
}

// Record stores the current value that a FRESH, STALE or WRONG verdict
// established, together with the claim's timeline. It returns the fact ID and
// whether anything was stored.
func (c *Cache) Record(ctx context.Context, cl report.Claim) (int64, bool, error) {
	switch cl.Verdict {
	case report.VerdictFresh, report.VerdictStale, report.VerdictWrong:
	default:
		return 0, false, nil
	}
	value := cl.CurrentValue
	if value == "" && cl.Verdict == report.VerdictFresh {
		value = cl.Value
	}
	if value == "" {
		return 0, false, nil
	}

	id, err := c.Store.UpsertFact(ctx, report.Fact{
		Key:            NormalizeKey(cl.Entity, cl.Attribute),
		Entity:         cl.Entity,
		Attribute:      cl.Attribute,
		Category:       cl.Category,
		CurrentValue:   value,
		AsOf:           cl.AsOf,
		SourceURL:      sourceURL(cl.Evidence),
		LastVerifiedAt: c.Now(),
	})
	if err != nil {
		return 0, false, err
	}
	if len(cl.Timeline) > 0 {
		if err := c.Store.AddTimeline(ctx, id, cl.Timeline); err != nil {
			return id, true, err
		}
	}
	return id, true, nil
}

func sourceURL(evidence []report.Evidence) string {
	for _, e := range evidence {
		if e.Stance == report.StanceSupports {
			return e.URL
		}
	}
	if len(evidence) > 0 {
		return evidence[0].URL
	}
	return ""
}

// CachedEvidence describes a stored fact as one evidence item for the judge.
// The verifier assigns its ref and authority.
func CachedEvidence(f report.Fact) report.Evidence {
	published := f.LastVerifiedAt.UTC()
	asOf := "an unknown date"
	if f.AsOf != "" {
		asOf = f.AsOf
		if t, err := time.Parse("2006-01-02", f.AsOf); err == nil {
			published = t
		}
	}
	return report.Evidence{
		URL:           f.SourceURL,
		Title:         "Previously verified fact",
		Domain:        domain(f.SourceURL),
		PublishedDate: &published,
		Snippet: fmt.Sprintf("%s, %s: %s (as of %s; verified on %s)",
			f.Entity, f.Attribute, f.CurrentValue, asOf, f.LastVerifiedAt.UTC().Format("2006-01-02")),
		Stance: report.StanceNeutral,
	}
}

func domain(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
}
