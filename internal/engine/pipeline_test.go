package engine_test

import (
	"context"
	"errors"
	"log/slog"
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
	in     extract.Input
}

func (f *fakeExtractor) Extract(_ context.Context, in extract.Input) ([]report.Claim, error) {
	f.in = in
	return f.claims, f.err
}

type fakeVerifier struct {
	called bool
	got    []report.Claim
	err    error
	edit   func([]report.Claim) []report.Claim
	credit int
}

func (f *fakeVerifier) Verify(_ context.Context, claims []report.Claim) ([]report.Claim, int, error) {
	f.called = true
	f.got = claims
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

type fakeReports struct{ saved []report.Report }

func (f *fakeReports) SaveReport(_ context.Context, r report.Report) error {
	f.saved = append(f.saved, r)
	return nil
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
	fact := report.Fact{Entity: "Example", Attribute: "price", CurrentValue: "$25", AsOf: "2026-09-01",
		SourceURL: "https://example.com/p", LastVerifiedAt: jobAt}
	ex := &fakeExtractor{claims: twoClaims()[:1]}
	v := &fakeVerifier{edit: markStale}
	p := newPipeline(ex, v, &fakeFacts{fresh: map[string]report.Fact{"Example": fact}}, &fakeReports{}, 30)
	p.Run(context.Background(), baseJob)

	require.True(t, v.called)
	require.Len(t, v.got, 1)
	assert.True(t, v.got[0].Checked)
	assert.True(t, v.got[0].CacheHit)
	require.Len(t, v.got[0].Evidence, 1)
	assert.Equal(t, "https://example.com/p", v.got[0].Evidence[0].URL)
	assert.Equal(t, 0, p.Budget.Used())
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

func TestRunSkipsVerifyWhenNothingChecked(t *testing.T) {
	v := &fakeVerifier{}
	r := newPipeline(&fakeExtractor{claims: twoClaims()}, v, &fakeFacts{}, &fakeReports{}, 0).Run(context.Background(), baseJob)
	assert.False(t, v.called)
	assert.Equal(t, report.StatusComplete, r.Status)
	assert.Nil(t, r.Score)
	assert.Equal(t, 0, r.CheckedCount)
	assert.Equal(t, report.VerdictNotChecked, r.Claims[0].Verdict)
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
