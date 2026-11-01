package verify_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lucifer-135/freshness-guard/internal/llm"
	"github.com/lucifer-135/freshness-guard/internal/report"
	"github.com/lucifer-135/freshness-guard/internal/tavily"
	"github.com/lucifer-135/freshness-guard/internal/verify"
)

var now = time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)

func date(y int, m time.Month, d int) *time.Time {
	t := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	return &t
}

type searchKey struct{ depth, query string }

type fakeSearcher struct {
	mu        sync.Mutex
	responses map[searchKey]tavily.SearchResponse
	errs      map[string]error
	requests  []tavily.SearchRequest
}

func (f *fakeSearcher) Search(_ context.Context, req tavily.SearchRequest) (tavily.SearchResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, req)
	if err := f.errs[req.Query]; err != nil {
		return tavily.SearchResponse{}, err
	}
	return f.responses[searchKey{req.Depth, req.Query}], nil
}

type fakeCompleter struct {
	replies  []string
	err      error
	requests []llm.Request
}

func (f *fakeCompleter) CompleteJSON(_ context.Context, req llm.Request, out any) error {
	f.requests = append(f.requests, req)
	if f.err != nil {
		return f.err
	}
	reply := f.replies[min(len(f.requests), len(f.replies))-1]
	if err := json.Unmarshal([]byte(reply), out); err != nil {
		return err
	}
	if v, ok := out.(llm.Validator); ok {
		return v.Validate()
	}
	return nil
}

type fakeSpender struct {
	allow bool
	calls []int
}

func (f *fakeSpender) TrySpend(n int) bool {
	f.calls = append(f.calls, n)
	return f.allow
}

func basicResponses() map[searchKey]tavily.SearchResponse {
	return map[searchKey]tavily.SearchResponse{
		{"basic", "price q"}: {Credits: 1, Results: []tavily.Result{
			{Title: "Pricing", URL: "https://example.com/pricing", Content: "Pro costs $25.", PublishedDate: date(2026, 9, 1)},
			{Title: "News", URL: "https://www.reuters.com/x", Content: "Price rose to $25.", PublishedDate: date(2026, 9, 2)},
		}},
		{"basic", "version q"}: {Credits: 1, Results: []tavily.Result{
			{Title: "Releases", URL: "https://example.com/releases", Content: "4.2 is current.", PublishedDate: date(2026, 10, 15)},
			{Title: "Blog", URL: "https://blog.dev/v", Content: "4.2 is out."},
		}},
	}
}

func testClaims() []report.Claim {
	return []report.Claim{
		{ID: "c1", Entity: "Example Cloud Pro", Attribute: "monthly price", Value: "$20", Category: report.CategoryPricing,
			SearchQuery: "price q", PChanged: 0.9, Checked: true, Verdict: report.VerdictNotChecked},
		{ID: "c2", Entity: "Example Cloud Pro", Attribute: "current version", Value: "4.2", Category: report.CategoryVersion,
			SearchQuery: "version q", PChanged: 0.9, Checked: true, Verdict: report.VerdictNotChecked},
		{ID: "c3", Entity: "Example", Attribute: "founded", Value: "2001", Category: report.CategoryStatic,
			Verdict: report.VerdictNotChecked},
	}
}

const judgeBoth = `{"results":[
 {"claim_id":"c1","verdict":"STALE","current_value":"$25","as_of":"2026-09-01","supporting_evidence_ids":["E2"],
  "contradicting_evidence_ids":["E1"],"timeline":[{"value":"$20","valid_from":"2025-01-01","source":"E1"},
  {"value":"$25","valid_from":"2026-09-01","source":"E2"}],"confidence":0.9,"rationale":"The price rose."},
 {"claim_id":"c2","verdict":"FRESH","current_value":"4.2","as_of":"2026-10-15","supporting_evidence_ids":["E3","E4"],
  "contradicting_evidence_ids":[],"timeline":[],"confidence":0.8,"rationale":"4.2 is current."}]}`

type judgeInput struct {
	Today  string `json:"today"`
	Claims []struct {
		ID       string `json:"id"`
		Evidence []struct {
			Ref           string `json:"ref"`
			URL           string `json:"url"`
			PublishedDate string `json:"published_date"`
			Snippet       string `json:"snippet"`
		} `json:"evidence"`
	} `json:"claims"`
}

func decodeJudgeInput(t *testing.T, req llm.Request) judgeInput {
	t.Helper()
	var in judgeInput
	require.NoError(t, json.Unmarshal([]byte(req.User), &in))
	return in
}

func newVerifier(s *fakeSearcher, c *fakeCompleter, b *fakeSpender) *verify.Verifier {
	return &verify.Verifier{Search: s, LLM: c, Model: "groq/judge", Fallbacks: []string{"gemini/j"}, Budget: b,
		Now: func() time.Time { return now }}
}

func TestVerifySearchesJudgesAndMapsResults(t *testing.T) {
	s := &fakeSearcher{responses: basicResponses()}
	c := &fakeCompleter{replies: []string{judgeBoth}}
	claims, credits, err := newVerifier(s, c, &fakeSpender{}).Verify(context.Background(), testClaims())
	require.NoError(t, err)
	assert.Equal(t, 2, credits)

	require.Len(t, s.requests, 2)
	for _, r := range s.requests {
		assert.Equal(t, "general", r.Topic)
		assert.Equal(t, "basic", r.Depth)
		assert.Equal(t, 5, r.MaxResults)
	}

	require.Len(t, c.requests, 1)
	req := c.requests[0]
	assert.Equal(t, "verdicts", req.SchemaName)
	assert.Equal(t, "groq/judge", req.Model)
	assert.Equal(t, []string{"gemini/j"}, req.Fallbacks)
	assert.Equal(t, verify.JudgeMaxTokens, req.MaxTokens)
	assert.True(t, json.Valid(req.Schema))
	in := decodeJudgeInput(t, req)
	assert.Equal(t, "2026-11-03", in.Today)
	require.Len(t, in.Claims, 2)
	assert.Equal(t, "c1", in.Claims[0].ID)
	assert.Equal(t, "c2", in.Claims[1].ID)
	assert.Equal(t, "E1", in.Claims[0].Evidence[0].Ref)
	assert.Equal(t, "2026-09-01", in.Claims[0].Evidence[0].PublishedDate)
	assert.Equal(t, "E4", in.Claims[1].Evidence[1].Ref)
	assert.Equal(t, "", in.Claims[1].Evidence[1].PublishedDate)

	c1 := claims[0]
	assert.Equal(t, report.VerdictStale, c1.Verdict)
	assert.Equal(t, "$25", c1.CurrentValue)
	assert.Equal(t, "2026-09-01", c1.AsOf)
	assert.Equal(t, 0.9, c1.Confidence)
	assert.Equal(t, report.StanceContradicts, c1.Evidence[0].Stance)
	assert.Equal(t, report.StanceSupports, c1.Evidence[1].Stance)
	assert.Equal(t, "example.com", c1.Evidence[0].Domain)
	assert.Equal(t, "reuters.com", c1.Evidence[1].Domain)
	assert.Equal(t, 1.0, c1.Evidence[0].Authority)
	require.Len(t, c1.Timeline, 2)
	assert.Equal(t, c1.Evidence[0].URL, c1.Timeline[0].SourceURL)
	assert.Equal(t, c1.Evidence[1].URL, c1.Timeline[1].SourceURL)

	assert.Equal(t, report.VerdictFresh, claims[1].Verdict)
	assert.Equal(t, report.StanceSupports, claims[1].Evidence[1].Stance)
	assert.Equal(t, 0.5, claims[1].Evidence[1].Authority)

	assert.Equal(t, testClaims()[2], claims[2])
}

// The live run showed WRONG verdicts with an empty current value, which leaves
// nothing to show, store or heal with.
func TestJudgePromptAsksForCurrentValueOnEveryVerdict(t *testing.T) {
	c := &fakeCompleter{replies: []string{judgeBoth}}
	_, _, err := newVerifier(&fakeSearcher{responses: basicResponses()}, c, &fakeSpender{}).Verify(context.Background(), testClaims())
	require.NoError(t, err)
	system := c.requests[0].System
	assert.Contains(t, system, `For FRESH, STALE and WRONG, "current_value" is the value that holds today according to the evidence`)
	assert.Contains(t, system, "Use WRONG only when the evidence shows that the claimed value never held")
}

func TestVerifyTruncatesSnippets(t *testing.T) {
	responses := basicResponses()
	r := responses[searchKey{"basic", "price q"}]
	r.Results[0].Content = strings.Repeat("ü", 1000)
	responses[searchKey{"basic", "price q"}] = r
	s := &fakeSearcher{responses: responses}
	claims, _, err := newVerifier(s, &fakeCompleter{replies: []string{judgeBoth}}, &fakeSpender{}).Verify(context.Background(), testClaims())
	require.NoError(t, err)
	assert.Equal(t, verify.MaxSnippetRunes, utf8.RuneCountInString(claims[0].Evidence[0].Snippet))
}

func TestVerifyMarksSearchFailures(t *testing.T) {
	for err, want := range map[error]string{
		tavily.ErrCreditLimit: "The Tavily credit limit has been reached.",
		tavily.ErrNoAPIKey:    "No Tavily API key is configured.",
		errors.New("timeout"): "The web search failed.",
	} {
		s := &fakeSearcher{responses: basicResponses(), errs: map[string]error{"price q": err}}
		c := &fakeCompleter{replies: []string{judgeBoth}}
		claims, credits, verr := newVerifier(s, c, &fakeSpender{}).Verify(context.Background(), testClaims())
		require.NoError(t, verr)
		assert.Equal(t, report.VerdictUnverifiable, claims[0].Verdict)
		assert.Equal(t, want, claims[0].Rationale)
		assert.Equal(t, 1, credits)
		in := decodeJudgeInput(t, c.requests[0])
		require.Len(t, in.Claims, 1)
		assert.Equal(t, "c2", in.Claims[0].ID)
		assert.Equal(t, "E1", in.Claims[0].Evidence[0].Ref)
	}
}

func TestVerifyNoResultsIsUnverifiable(t *testing.T) {
	responses := basicResponses()
	responses[searchKey{"basic", "price q"}] = tavily.SearchResponse{Credits: 1}
	s := &fakeSearcher{responses: responses}
	claims, credits, err := newVerifier(s, &fakeCompleter{replies: []string{judgeBoth}}, &fakeSpender{}).Verify(context.Background(), testClaims())
	require.NoError(t, err)
	assert.Equal(t, report.VerdictUnverifiable, claims[0].Verdict)
	assert.Equal(t, "The web search returned no results.", claims[0].Rationale)
	assert.Equal(t, 2, credits)
}

func TestVerifyMissingJudgeResultIsUnverifiable(t *testing.T) {
	only2 := `{"results":[{"claim_id":"c2","verdict":"FRESH","current_value":"4.2","as_of":"","supporting_evidence_ids":["E3"],
		"contradicting_evidence_ids":[],"timeline":[],"confidence":0.8,"rationale":"ok"}]}`
	s := &fakeSearcher{responses: basicResponses()}
	claims, _, err := newVerifier(s, &fakeCompleter{replies: []string{only2}}, &fakeSpender{}).Verify(context.Background(), testClaims())
	require.NoError(t, err)
	assert.Equal(t, report.VerdictUnverifiable, claims[0].Verdict)
	assert.Equal(t, "The judge returned no result for this claim.", claims[0].Rationale)
	assert.Equal(t, report.VerdictFresh, claims[1].Verdict)
}

const firstUnverifiable = `{"results":[
 {"claim_id":"c1","verdict":"UNVERIFIABLE","current_value":"","as_of":"","supporting_evidence_ids":[],
  "contradicting_evidence_ids":[],"timeline":[],"confidence":0.3,"rationale":"Not enough evidence."},
 {"claim_id":"c2","verdict":"FRESH","current_value":"4.2","as_of":"","supporting_evidence_ids":["E3"],
  "contradicting_evidence_ids":[],"timeline":[],"confidence":0.8,"rationale":"ok"}]}`

const secondFresh = `{"results":[{"claim_id":"c1","verdict":"FRESH","current_value":"$20","as_of":"2026-10-01",
 "supporting_evidence_ids":["E5"],"contradicting_evidence_ids":[],"timeline":[],"confidence":0.7,"rationale":"Confirmed."}]}`

func escalationResponses() map[searchKey]tavily.SearchResponse {
	responses := basicResponses()
	responses[searchKey{"advanced", "price q"}] = tavily.SearchResponse{Credits: 2, Results: []tavily.Result{
		{Title: "Pricing", URL: "https://example.com/pricing", Content: "dup"},
		{Title: "Plans", URL: "https://example.com/plans", Content: "Pro is $20.", PublishedDate: date(2026, 10, 1)},
	}}
	return responses
}

func TestVerifyEscalatesUnverifiableOnce(t *testing.T) {
	s := &fakeSearcher{responses: escalationResponses()}
	c := &fakeCompleter{replies: []string{firstUnverifiable, secondFresh}}
	b := &fakeSpender{allow: true}
	claims, credits, err := newVerifier(s, c, b).Verify(context.Background(), testClaims())
	require.NoError(t, err)

	assert.Equal(t, []int{2}, b.calls)
	assert.Equal(t, 4, credits)
	require.Len(t, s.requests, 3)
	assert.Equal(t, "advanced", s.requests[2].Depth)
	assert.Equal(t, "price q", s.requests[2].Query)

	require.Len(t, c.requests, 2)
	in := decodeJudgeInput(t, c.requests[1])
	require.Len(t, in.Claims, 1)
	assert.Equal(t, "c1", in.Claims[0].ID)
	require.Len(t, in.Claims[0].Evidence, 3)
	assert.Equal(t, "E5", in.Claims[0].Evidence[2].Ref)
	assert.Equal(t, "https://example.com/plans", in.Claims[0].Evidence[2].URL)

	assert.Equal(t, report.VerdictFresh, claims[0].Verdict)
	assert.Equal(t, report.StanceSupports, claims[0].Evidence[2].Stance)
	assert.Equal(t, report.StanceNeutral, claims[0].Evidence[0].Stance)
	assert.Equal(t, report.VerdictFresh, claims[1].Verdict)
}

func TestVerifyKeepsFirstResultWhenSecondJudgeFails(t *testing.T) {
	s := &fakeSearcher{responses: escalationResponses()}
	c := &fakeCompleter{replies: []string{firstUnverifiable, `{"results":null}`}}
	claims, _, err := newVerifier(s, c, &fakeSpender{allow: true}).Verify(context.Background(), testClaims())
	require.NoError(t, err)
	assert.Equal(t, report.VerdictUnverifiable, claims[0].Verdict)
	assert.Equal(t, "Not enough evidence.", claims[0].Rationale)
}

func TestVerifyNoEscalationWithoutBudget(t *testing.T) {
	s := &fakeSearcher{responses: escalationResponses()}
	c := &fakeCompleter{replies: []string{firstUnverifiable}}
	claims, credits, err := newVerifier(s, c, &fakeSpender{allow: false}).Verify(context.Background(), testClaims())
	require.NoError(t, err)
	assert.Equal(t, report.VerdictUnverifiable, claims[0].Verdict)
	assert.Len(t, c.requests, 1)
	assert.Equal(t, 2, credits)
}

func TestVerifyUsesCachedEvidenceWithoutSearch(t *testing.T) {
	claims := testClaims()
	claims[0].CacheHit = true
	claims[0].Evidence = []report.Evidence{{URL: "https://example.com/pricing", Title: "Previously verified fact",
		Snippet: "cached", PublishedDate: date(2026, 9, 1), Stance: report.StanceNeutral}}
	s := &fakeSearcher{responses: basicResponses()}
	c := &fakeCompleter{replies: []string{judgeBoth}}
	got, credits, err := newVerifier(s, c, &fakeSpender{}).Verify(context.Background(), claims)
	require.NoError(t, err)
	require.Len(t, s.requests, 1)
	assert.Equal(t, "version q", s.requests[0].Query)
	assert.Equal(t, 1, credits)
	in := decodeJudgeInput(t, c.requests[0])
	assert.Equal(t, "E1", in.Claims[0].Evidence[0].Ref)
	assert.Equal(t, 1.0, got[0].Evidence[0].Authority)
}

func TestVerifyReturnsJudgeError(t *testing.T) {
	s := &fakeSearcher{responses: basicResponses()}
	_, _, err := newVerifier(s, &fakeCompleter{err: errors.New("judge down")}, &fakeSpender{}).Verify(context.Background(), testClaims())
	assert.Error(t, err)
}

func TestVerifyRejectsUnknownVerdict(t *testing.T) {
	bad := `{"results":[{"claim_id":"c1","verdict":"MAYBE","current_value":"","as_of":"","supporting_evidence_ids":[],
		"contradicting_evidence_ids":[],"timeline":[],"confidence":0.5,"rationale":""}]}`
	s := &fakeSearcher{responses: basicResponses()}
	_, _, err := newVerifier(s, &fakeCompleter{replies: []string{bad}}, &fakeSpender{}).Verify(context.Background(), testClaims())
	assert.Error(t, err)
}

func conflictClaim(contraAuthority float64, contraDate *time.Time) report.Claim {
	return report.Claim{Verdict: report.VerdictFresh, Rationale: "Looks current.", Evidence: []report.Evidence{
		{Stance: report.StanceSupports, Authority: 0.5, PublishedDate: date(2026, 1, 1)},
		{Stance: report.StanceContradicts, Authority: contraAuthority, PublishedDate: contraDate},
	}}
}

func TestConflictRuleDowngradesFresh(t *testing.T) {
	got := verify.ApplyConflictRule(conflictClaim(1.0, date(2026, 6, 1)))
	assert.Equal(t, report.VerdictUnverifiable, got.Verdict)
	assert.True(t, strings.HasPrefix(got.Rationale, "A newer authoritative source disagrees with this claim. "))
	assert.True(t, strings.HasSuffix(got.Rationale, "Looks current."))

	assert.Equal(t, report.VerdictFresh, verify.ApplyConflictRule(conflictClaim(1.0, date(2025, 6, 1))).Verdict)
	assert.Equal(t, report.VerdictFresh, verify.ApplyConflictRule(conflictClaim(0.5, date(2026, 6, 1))).Verdict)
	assert.Equal(t, report.VerdictFresh, verify.ApplyConflictRule(conflictClaim(1.0, nil)).Verdict)

	stale := conflictClaim(1.0, date(2026, 6, 1))
	stale.Verdict = report.VerdictStale
	assert.Equal(t, report.VerdictStale, verify.ApplyConflictRule(stale).Verdict)
}
