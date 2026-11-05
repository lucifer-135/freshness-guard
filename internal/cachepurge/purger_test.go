package cachepurge_test

import (
	"bytes"
	"context"
	"log/slog"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lucifer-135/freshness-guard/internal/cachepurge"
	"github.com/lucifer-135/freshness-guard/internal/events"
	"github.com/lucifer-135/freshness-guard/internal/report"
	"github.com/lucifer-135/freshness-guard/internal/store"
)

var purgeNow = time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)

type linkCall struct {
	cacheID, reportID string
	links             []store.CacheLink
}

type fakeStore struct {
	calls   []string
	linked  []linkCall
	entries map[int64]map[string]string // fact ID -> cache ID -> value key
	purges  []store.CachePurge
}

func newFakeStore() *fakeStore { return &fakeStore{entries: map[int64]map[string]string{}} }

func (f *fakeStore) LinkCache(_ context.Context, cacheID, reportID string, links []store.CacheLink, _ time.Time) error {
	f.calls = append(f.calls, "link")
	f.linked = append(f.linked, linkCall{cacheID, reportID, links})
	for _, l := range links {
		if f.entries[l.FactID] == nil {
			f.entries[l.FactID] = map[string]string{}
		}
		f.entries[l.FactID][cacheID] = l.ValueKey
	}
	return nil
}

func (f *fakeStore) CacheEntriesForFact(_ context.Context, factID int64, exceptValueKey string) ([]string, error) {
	f.calls = append(f.calls, "entries")
	var ids []string
	for id, key := range f.entries[factID] {
		if key != exceptValueKey {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids, nil
}

func (f *fakeStore) RecordPurge(_ context.Context, p store.CachePurge) error {
	f.calls = append(f.calls, "purge")
	f.purges = append(f.purges, p)
	return nil
}

type fakeBifrost struct {
	cleared []string
	err     error
}

func (f *fakeBifrost) Clear(_ context.Context, id string) error {
	if f.err != nil {
		return f.err
	}
	f.cleared = append(f.cleared, id)
	return nil
}

type fakePublisher struct{ events []events.Event }

func (f *fakePublisher) Publish(e events.Event) { f.events = append(f.events, e) }

type setup struct {
	st   *fakeStore
	b    *fakeBifrost
	pub  *fakePublisher
	logs *bytes.Buffer
	p    *cachepurge.Purger
}

func newSetup() *setup {
	s := &setup{st: newFakeStore(), b: &fakeBifrost{}, pub: &fakePublisher{}, logs: &bytes.Buffer{}}
	s.p = &cachepurge.Purger{
		Store: s.st, Bifrost: s.b, Events: s.pub, Now: func() time.Time { return purgeNow },
		Logger: slog.New(slog.NewTextHandler(s.logs, nil)),
	}
	return s
}

func claim(verdict report.Verdict, factID int64, value, current string) report.Claim {
	id := factID
	return report.Claim{ID: "c1", Entity: "Example", Attribute: "price", Value: value, CurrentValue: current,
		Verdict: verdict, Checked: true, FactID: &id}
}

func finished(cacheID string, claims ...report.Claim) report.Report {
	return report.Report{ID: "r-1", Status: report.StatusComplete, CacheID: cacheID, Claims: claims}
}

func TestFinishPurgesOwnEntryWhenAnswerIsOutdated(t *testing.T) {
	s := newSetup()
	s.p.Finish(context.Background(), finished("c1", claim(report.VerdictStale, 7, "$20", "$25 per month")))

	assert.Equal(t, []string{"c1"}, s.b.cleared)
	require.Len(t, s.st.purges, 1)
	assert.Equal(t, "outdated_answer", s.st.purges[0].Reason)
	assert.Equal(t, "c1", s.st.purges[0].CacheID)
	assert.Equal(t, "r-1", s.st.purges[0].ReportID)
	assert.Empty(t, s.st.linked, "an outdated answer is not linked")
	require.Len(t, s.pub.events, 1)
	assert.Equal(t, events.TypeCachePurged, s.pub.events[0].Type)
	assert.Equal(t, "r-1", s.pub.events[0].ReportID)
	assert.Equal(t, map[string]any{"cache_ids": []string{"c1"}, "count": 1}, s.pub.events[0].Data)
}

func TestFinishLinksFreshAnswer(t *testing.T) {
	s := newSetup()
	s.p.Finish(context.Background(), finished("c1", claim(report.VerdictFresh, 7, "$25 per month", "")))

	assert.Empty(t, s.b.cleared)
	assert.Empty(t, s.pub.events)
	require.Len(t, s.st.linked, 1)
	assert.Equal(t, "c1", s.st.linked[0].cacheID)
	assert.Equal(t, "r-1", s.st.linked[0].reportID)
	assert.Equal(t, []store.CacheLink{{FactID: 7, ValueKey: "25 per month"}}, s.st.linked[0].links)
}

func TestFinishPurgesEntriesLinkedToChangedFact(t *testing.T) {
	s := newSetup()
	s.st.entries[7] = map[string]string{"c-old": "20 per month"}
	s.p.Finish(context.Background(), finished("", claim(report.VerdictStale, 7, "$20 per month", "$25 per month")))

	assert.Equal(t, []string{"c-old"}, s.b.cleared)
	require.Len(t, s.st.purges, 1)
	assert.Equal(t, "fact_changed", s.st.purges[0].Reason)
	require.NotNil(t, s.st.purges[0].FactID)
	assert.Equal(t, int64(7), *s.st.purges[0].FactID)
}

func TestFinishKeepsEntriesWithTheSameValue(t *testing.T) {
	s := newSetup()
	s.st.entries[7] = map[string]string{"c-same": "25 per month"}
	s.p.Finish(context.Background(), finished("", claim(report.VerdictFresh, 7, "$25 per month", "")))

	assert.Empty(t, s.b.cleared)
}

func TestFinishPurgesWhenValueTextDiffers(t *testing.T) {
	s := newSetup()
	s.st.entries[7] = map[string]string{"c-old": "25 per month"}
	s.p.Finish(context.Background(), finished("", claim(report.VerdictFresh, 7, "$25/month", "")))

	assert.Equal(t, []string{"c-old"}, s.b.cleared, "a reworded value may purge too much, never too little")
}

func TestFinishIgnoresClaimsAnsweredFromTheVerdictCache(t *testing.T) {
	s := newSetup()
	s.st.entries[7] = map[string]string{"c-old": "20 per month"}
	cached := claim(report.VerdictFresh, 7, "$25 per month", "")
	cached.CacheHit = true
	s.p.Finish(context.Background(), finished("", cached))

	assert.Empty(t, s.b.cleared, "a verdict-cache hit did not refresh the fact")
}

func TestFinishPurgesEachEntryOnce(t *testing.T) {
	s := newSetup()
	s.st.entries[7] = map[string]string{"c1": "20 per month"}
	s.p.Finish(context.Background(), finished("c1", claim(report.VerdictStale, 7, "$20 per month", "$25 per month")))

	assert.Equal(t, []string{"c1"}, s.b.cleared)
	assert.Len(t, s.st.purges, 1)
}

func TestFinishSkipsIncompleteReports(t *testing.T) {
	s := newSetup()
	r := finished("c1", claim(report.VerdictStale, 7, "$20", "$25"))
	r.Status = report.StatusError
	s.p.Finish(context.Background(), r)

	assert.Empty(t, s.st.calls)
	assert.Empty(t, s.b.cleared)
	assert.Empty(t, s.pub.events)
}

func TestFinishKeepsLinksWhenClearFails(t *testing.T) {
	s := newSetup()
	s.b.err = &cachepurge.ClearError{Status: 403, Body: "no setup token"}
	s.st.entries[7] = map[string]string{"c-old": "20 per month"}

	assert.NotPanics(t, func() {
		s.p.Finish(context.Background(), finished("c1", claim(report.VerdictStale, 7, "$20 per month", "$25 per month")))
	})
	assert.Empty(t, s.st.purges, "nothing is recorded, so the links stay for a retry")
	assert.Empty(t, s.pub.events)
	assert.Contains(t, s.logs.String(), "could not be removed from the semantic cache")
	assert.Contains(t, s.logs.String(), "403")
}
