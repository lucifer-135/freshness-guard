package engine_test

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lucifer-135/freshness-guard/internal/engine"
	"github.com/lucifer-135/freshness-guard/internal/extract"
	"github.com/lucifer-135/freshness-guard/internal/report"
)

var (
	jobAt   = time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	doneAt  = jobAt.Add(1500 * time.Millisecond)
	baseJob = engine.Job{ReportID: "r-1", Answer: "It costs $20.", Prompt: "Cost?", Model: "groq/x",
		Provider: "groq", VirtualKey: "vk", Mode: "async", At: jobAt}
)

type fakeExtractor struct {
	claims []report.Claim
	err    error
	panics bool
	in     extract.Input
}

func (f *fakeExtractor) Extract(_ context.Context, in extract.Input) ([]report.Claim, error) {
	f.in = in
	if f.panics {
		panic("extractor exploded")
	}
	return f.claims, f.err
}

type fakeVerifier struct {
	called bool
	got    []report.Claim
	err    error
	edit   func([]report.Claim) []report.Claim
	credit int
	block  bool
}

func (f *fakeVerifier) Verify(ctx context.Context, claims []report.Claim) ([]report.Claim, int, error) {
	f.called = true
	f.got = claims
	if f.block {
		<-ctx.Done()
		return claims, 0, ctx.Err()
	}
	out := append([]report.Claim(nil), claims...)
	if f.edit != nil {
		out = f.edit(out)
	}
	return out, f.credit, f.err
}

type fakeFacts struct {
	fresh    map[string]report.Fact
	recorded []report.Claim
}

func (f *fakeFacts) Fresh(_ context.Context, entity, _ string, _ report.Category) (report.Fact, bool) {
	fact, ok := f.fresh[entity]
	return fact, ok
}

func (f *fakeFacts) Record(_ context.Context, c report.Claim) (int64, bool, error) {
	f.recorded = append(f.recorded, c)
	return 7, true, nil
}

// fakeReports fails like SQLite does when the context has ended.
type fakeReports struct {
	mu    sync.Mutex
	saved []report.Report
}

func (f *fakeReports) SaveReport(ctx context.Context, r report.Report) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.saved = append(f.saved, r)
	return nil
}

func (f *fakeReports) last() report.Report {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.saved[len(f.saved)-1]
}

func twoClaims() []report.Claim {
	return []report.Claim{
		{ID: "c1", Entity: "Example", Attribute: "price", Value: "$20", Category: report.CategoryPricing, Verdict: report.VerdictNotChecked},
		{ID: "c2", Entity: "Example", Attribute: "founded", Value: "2001", Category: report.CategoryStatic, Verdict: report.VerdictNotChecked},
	}
}

func markStale(claims []report.Claim) []report.Claim {
	for i := range claims {
		if claims[i].ID == "c1" {
			claims[i].Verdict = report.VerdictStale
			claims[i].Confidence = 0.9
			claims[i].CurrentValue = "$25"
		}
	}
	return claims
}

func newPipeline(ex *fakeExtractor, v *fakeVerifier, fx *fakeFacts, st *fakeReports, daily int) *engine.Pipeline {
	c := &clock{doneAt}
	return &engine.Pipeline{
		Extract: ex, Verify: v, Facts: fx, Store: st,
		Budget:        engine.NewBudget(&fakeCreditStore{}, daily, c.now, slog.Default()),
		RiskThreshold: 0.3, Now: c.now, Logger: slog.Default(),
	}
}

func TestRunCompletesReport(t *testing.T) {
	ex := &fakeExtractor{claims: twoClaims()}
	v := &fakeVerifier{edit: markStale, credit: 1}
	fx := &fakeFacts{}
	st := &fakeReports{}
	r := newPipeline(ex, v, fx, st, 30).Run(context.Background(), baseJob)

	assert.Equal(t, extract.Input{Answer: "It costs $20.", Prompt: "Cost?", Model: "groq/x", At: jobAt}, ex.in)
	require.Len(t, st.saved, 2)
	assert.Equal(t, report.StatusPending, st.saved[0].Status)
	assert.Equal(t, "r-1", st.saved[0].ID)
	assert.Equal(t, "It costs $20.", st.saved[0].AnswerText)
	assert.Equal(t, r, st.saved[1])

	assert.Equal(t, report.StatusComplete, r.Status)
	assert.Equal(t, "r-1", r.ID)
	assert.True(t, jobAt.Equal(r.CreatedAt))
	assert.Equal(t, "groq/x", r.Model)
	assert.Equal(t, "groq", r.Provider)
	assert.Equal(t, "vk", r.VirtualKey)
	assert.Equal(t, "Cost?", r.Prompt)
	require.Len(t, r.Claims, 2)
	c1, c2 := r.Claims[0], r.Claims[1]
	assert.True(t, c1.Checked)
	assert.Greater(t, c1.PChanged, 0.9)
	require.NotNil(t, c1.FactID)
	assert.Equal(t, int64(7), *c1.FactID)
	require.NotNil(t, c1.Score)
	assert.Equal(t, 2, *c1.Score)
	assert.False(t, c2.Checked)
	assert.Equal(t, 0.0, c2.PChanged)
	assert.Nil(t, c2.Score)
	require.NotNil(t, r.Score)
	assert.Equal(t, 2, *r.Score)
	assert.Equal(t, 1, r.StaleCount)
	assert.Equal(t, 1, r.CheckedCount)
	assert.Equal(t, 2, r.TotalClaims)
	assert.Equal(t, 1, r.CreditsUsed)
	assert.Equal(t, int64(1500), r.LatencyMS)
	require.Len(t, fx.recorded, 1)
	assert.Equal(t, "c1", fx.recorded[0].ID)
}

func TestRunUsesVerdictCache(t *testing.T) {
	fact := report.Fact{ID: 42, Entity: "Example", Attribute: "price", CurrentValue: "$25", AsOf: "2026-09-01",
		SourceURL: "https://example.com/p", LastVerifiedAt: jobAt}
	ex := &fakeExtractor{claims: twoClaims()[:1]}
	v := &fakeVerifier{edit: markStale}
	fx := &fakeFacts{fresh: map[string]report.Fact{"Example": fact}}
	p := newPipeline(ex, v, fx, &fakeReports{}, 30)
	r := p.Run(context.Background(), baseJob)

	require.True(t, v.called)
	require.Len(t, v.got, 1)
	assert.True(t, v.got[0].Checked)
	assert.True(t, v.got[0].CacheHit)
	require.Len(t, v.got[0].Evidence, 1)
	assert.Equal(t, "https://example.com/p", v.got[0].Evidence[0].URL)
	assert.Equal(t, 0, p.Budget.Used())

	// A cache hit must not refresh the fact, or it would never be checked live again.
	assert.Empty(t, fx.recorded)
	require.NotNil(t, r.Claims[0].FactID)
	assert.Equal(t, int64(42), *r.Claims[0].FactID)
}

func TestRunSavesErrorWhenJobContextEnds(t *testing.T) {
	st := &fakeReports{}
	p := newPipeline(&fakeExtractor{claims: twoClaims()}, &fakeVerifier{block: true}, &fakeFacts{}, st, 30)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan report.Report, 1)
	go func() { done <- p.Run(ctx, baseJob) }()
	time.Sleep(20 * time.Millisecond)
	cancel()
	r := <-done
	assert.Equal(t, report.StatusError, r.Status)
	assert.Equal(t, report.StatusError, st.last().Status, "the final save must not use the ended job context")
}

func TestRunPanicIsSavedAsError(t *testing.T) {
	st := &fakeReports{}
	p := newPipeline(&fakeExtractor{panics: true}, &fakeVerifier{}, &fakeFacts{}, st, 30)
	var r report.Report
	require.NotPanics(t, func() { r = p.Run(context.Background(), baseJob) })
	assert.Equal(t, report.StatusError, r.Status)
	assert.Equal(t, "An internal error stopped the verification.", r.Reason)
	assert.Equal(t, report.StatusError, st.last().Status)
}

func TestRunExtractionErrorMarksReport(t *testing.T) {
	st := &fakeReports{}
	r := newPipeline(&fakeExtractor{err: errors.New("model down")}, &fakeVerifier{}, &fakeFacts{}, st, 30).
		Run(context.Background(), baseJob)
	assert.Equal(t, report.StatusError, r.Status)
	assert.Contains(t, r.Reason, "Claim extraction failed")
	assert.Contains(t, r.Reason, "model down")
	require.Len(t, st.saved, 2)
	assert.Equal(t, report.StatusError, st.saved[1].Status)
}

func TestRunWithoutBudgetSkipsVerifyAndGivesReason(t *testing.T) {
	v := &fakeVerifier{}
	fx := &fakeFacts{}
	r := newPipeline(&fakeExtractor{claims: twoClaims()}, v, fx, &fakeReports{}, 0).Run(context.Background(), baseJob)
	assert.False(t, v.called)
	assert.Equal(t, report.StatusComplete, r.Status)
	assert.Equal(t, 0, r.CheckedCount)
	assert.Empty(t, fx.recorded)

	risky := r.Claims[0]
	assert.False(t, risky.Checked)
	assert.Equal(t, report.VerdictUnverifiable, risky.Verdict)
	assert.Equal(t, "This claim was not checked because today's Tavily credit budget is used up or kept for riskier claims.", risky.Rationale)
	require.NotNil(t, risky.Score)
	assert.Equal(t, 21, *risky.Score)

	lowRisk := r.Claims[1]
	assert.Equal(t, report.VerdictNotChecked, lowRisk.Verdict)
	assert.Empty(t, lowRisk.Rationale)
	assert.Nil(t, lowRisk.Score)
}

func TestRunVerifyErrorMarksReport(t *testing.T) {
	r := newPipeline(&fakeExtractor{claims: twoClaims()}, &fakeVerifier{err: errors.New("judge down")}, &fakeFacts{}, &fakeReports{}, 30).
		Run(context.Background(), baseJob)
	assert.Equal(t, report.StatusError, r.Status)
	assert.Contains(t, r.Reason, "Verification failed")
	assert.Len(t, r.Claims, 2)
}

func TestRunWithoutClaimsCompletes(t *testing.T) {
	r := newPipeline(&fakeExtractor{claims: []report.Claim{}}, &fakeVerifier{}, &fakeFacts{}, &fakeReports{}, 30).
		Run(context.Background(), baseJob)
	assert.Equal(t, report.StatusComplete, r.Status)
	assert.NotNil(t, r.Claims)
	assert.Equal(t, 0, r.TotalClaims)
}
