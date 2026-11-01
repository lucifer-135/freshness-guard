package extract_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lucifer-135/freshness-guard/internal/extract"
	"github.com/lucifer-135/freshness-guard/internal/llm"
	"github.com/lucifer-135/freshness-guard/internal/report"
)

type fakeCompleter struct {
	canned string
	err    error
	req    llm.Request
}

func (f *fakeCompleter) CompleteJSON(_ context.Context, req llm.Request, out any) error {
	f.req = req
	if f.err != nil {
		return f.err
	}
	if err := json.Unmarshal([]byte(f.canned), out); err != nil {
		return err
	}
	if v, ok := out.(llm.Validator); ok {
		return v.Validate()
	}
	return nil
}

const threeClaims = `{"claims":[
 {"quote":"Example Cloud Pro costs $20 per month","entity":"Example Cloud Pro","attribute":"monthly price","value":"$20 per month",
  "category":"pricing","temporal_marker":"","search_query":"Example Cloud Pro monthly price"},
 {"quote":"its current version is 4.2","entity":" Example Cloud Pro ","attribute":"current version","value":"4.2",
  "category":" Version ","temporal_marker":"current","search_query":"Example Cloud Pro latest version"},
 {"quote":"costs","entity":"","attribute":"x","value":"y","category":"pricing","temporal_marker":"","search_query":"q"}]}`

var at = time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)

func userJSON(t *testing.T, req llm.Request) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(req.User), &m))
	return m
}

func TestExtractBuildsRequestAndClaims(t *testing.T) {
	f := &fakeCompleter{canned: threeClaims}
	e := &extract.Extractor{LLM: f, Model: "groq/x", Fallbacks: []string{"gemini/y"}, MaxClaims: 12}
	claims, err := e.Extract(context.Background(), extract.Input{Answer: answer, Prompt: "What does it cost?", Model: "groq/m", At: at})
	require.NoError(t, err)

	assert.Equal(t, "groq/x", f.req.Model)
	assert.Equal(t, []string{"gemini/y"}, f.req.Fallbacks)
	assert.Equal(t, "claims", f.req.SchemaName)
	assert.Equal(t, 2048, f.req.MaxTokens)
	assert.Equal(t, extract.Timeout, f.req.Timeout)
	assert.True(t, json.Valid(f.req.Schema))
	assert.Contains(t, f.req.System, "at most 12 claims")
	user := userJSON(t, f.req)
	assert.Equal(t, "2026-11-03", user["today"])
	assert.Equal(t, answer, user["answer"])
	assert.Equal(t, "What does it cost?", user["question"])
	assert.Equal(t, "groq/m", user["model"])
	assert.Equal(t, 12.0, user["max_claims"])

	require.Len(t, claims, 2)
	assert.Equal(t, "c1", claims[0].ID)
	assert.Equal(t, "c2", claims[1].ID)
	assert.Equal(t, report.CategoryPricing, claims[0].Category)
	assert.Equal(t, report.CategoryVersion, claims[1].Category)
	assert.Equal(t, "Example Cloud Pro", claims[1].Entity)
	assert.Equal(t, "current", claims[1].TemporalMarker)
	assert.Equal(t, 0, claims[0].SpanStart)
	assert.Equal(t, 37, claims[0].SpanEnd)
	assert.Equal(t, 43, claims[1].SpanStart)
	for _, c := range claims {
		assert.Equal(t, report.VerdictNotChecked, c.Verdict)
		assert.False(t, c.Checked)
	}
}

func TestExtractCapsClaims(t *testing.T) {
	e := &extract.Extractor{LLM: &fakeCompleter{canned: threeClaims}, Model: "m", MaxClaims: 1}
	claims, err := e.Extract(context.Background(), extract.Input{Answer: answer, At: at})
	require.NoError(t, err)
	require.Len(t, claims, 1)
	assert.Equal(t, "c1", claims[0].ID)
}

func TestExtractUnknownCategoryBecomesStatic(t *testing.T) {
	canned := `{"claims":[{"quote":"costs","entity":"E","attribute":"A","value":"V","category":"weather",
		"temporal_marker":"","search_query":"q"}]}`
	e := &extract.Extractor{LLM: &fakeCompleter{canned: canned}, Model: "m", MaxClaims: 12}
	claims, err := e.Extract(context.Background(), extract.Input{Answer: answer, At: at})
	require.NoError(t, err)
	require.Len(t, claims, 1)
	assert.Equal(t, report.CategoryStatic, claims[0].Category)
}

func TestExtractTruncatesLongAnswerAndPrompt(t *testing.T) {
	f := &fakeCompleter{canned: `{"claims":[]}`}
	e := &extract.Extractor{LLM: f, Model: "m", MaxClaims: 12}
	_, err := e.Extract(context.Background(), extract.Input{
		Answer: strings.Repeat("é", 13000), Prompt: strings.Repeat("q", 2500), At: at})
	require.NoError(t, err)
	user := userJSON(t, f.req)
	assert.Equal(t, 12000, utf8.RuneCountInString(user["answer"].(string)))
	assert.Equal(t, 2000, utf8.RuneCountInString(user["question"].(string)))
}

func TestExtractRejectsMissingClaims(t *testing.T) {
	e := &extract.Extractor{LLM: &fakeCompleter{canned: `{"claims":null}`}, Model: "m", MaxClaims: 12}
	_, err := e.Extract(context.Background(), extract.Input{Answer: answer, At: at})
	assert.Error(t, err)
}

func TestExtractReturnsCompleterError(t *testing.T) {
	e := &extract.Extractor{LLM: &fakeCompleter{err: errors.New("down")}, Model: "m", MaxClaims: 12}
	_, err := e.Extract(context.Background(), extract.Input{Answer: answer, At: at})
	assert.Error(t, err)
}
