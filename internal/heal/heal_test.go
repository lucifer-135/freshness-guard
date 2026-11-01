package heal_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lucifer-135/freshness-guard/internal/heal"
	"github.com/lucifer-135/freshness-guard/internal/llm"
	"github.com/lucifer-135/freshness-guard/internal/report"
)

type fakeCompleter struct {
	reply    string
	err      error
	requests []llm.Request
}

func (f *fakeCompleter) CompleteJSON(_ context.Context, req llm.Request, out any) error {
	f.requests = append(f.requests, req)
	if f.err != nil {
		return f.err
	}
	return json.Unmarshal([]byte(f.reply), out)
}

func replyWith(text string) string {
	b, _ := json.Marshal(map[string]string{"text": text})
	return string(b)
}

func newHealer(c *fakeCompleter) *heal.Healer {
	return &heal.Healer{LLM: c, Model: "groq/heal", Fallbacks: []string{"gemini/h"}}
}

func TestCorrectionsSelectsFixableClaims(t *testing.T) {
	fixable := staleClaim()
	noValue := staleClaim()
	noValue.CurrentValue = ""
	noSpan := staleClaim()
	noSpan.SpanStart, noSpan.SpanEnd = -1, -1
	fresh := staleClaim()
	fresh.Verdict = report.VerdictFresh
	unverifiable := staleClaim()
	unverifiable.Verdict = report.VerdictUnverifiable
	wrong := staleClaim()
	wrong.Verdict = report.VerdictWrong

	got := heal.Corrections([]report.Claim{fixable, noValue, noSpan, fresh, unverifiable, wrong})
	require.Len(t, got, 2)
	assert.Equal(t, report.VerdictStale, got[0].Verdict)
	assert.Equal(t, report.VerdictWrong, got[1].Verdict)
}

func TestHealUsesValidEdit(t *testing.T) {
	edited := strings.Replace(answer, "$20", "$25", 1)
	c := &fakeCompleter{reply: replyWith(edited)}
	text, healed, err := newHealer(c).Heal(context.Background(), answer, []report.Claim{staleClaim()})
	require.NoError(t, err)
	assert.Equal(t, edited, text)
	assert.Equal(t, 1, healed)

	require.Len(t, c.requests, 1)
	req := c.requests[0]
	assert.Equal(t, "healed_answer", req.SchemaName)
	assert.Equal(t, "groq/heal", req.Model)
	assert.Equal(t, []string{"gemini/h"}, req.Fallbacks)
	assert.Equal(t, heal.MaxTokens, req.MaxTokens)
	assert.Equal(t, heal.Timeout, req.Timeout)
	assert.True(t, json.Valid(req.Schema))
	var user struct {
		Answer      string `json:"answer"`
		Corrections []struct {
			Quote        string `json:"quote"`
			Value        string `json:"value"`
			CurrentValue string `json:"current_value"`
			AsOf         string `json:"as_of"`
		} `json:"corrections"`
	}
	require.NoError(t, json.Unmarshal([]byte(req.User), &user))
	assert.Equal(t, answer, user.Answer)
	require.Len(t, user.Corrections, 1)
	assert.Equal(t, "$20 per month", user.Corrections[0].Value)
	assert.Equal(t, "$25 per month", user.Corrections[0].CurrentValue)
	assert.Equal(t, "2026-09-01", user.Corrections[0].AsOf)
}

func TestHealFallsBackToAnnotationWhenGuardRejects(t *testing.T) {
	c := &fakeCompleter{reply: replyWith("Something else entirely.")}
	text, healed, err := newHealer(c).Heal(context.Background(), answer, []report.Claim{staleClaim()})
	require.NoError(t, err)
	assert.Equal(t, heal.Annotate(answer, []report.Claim{staleClaim()}), text)
	assert.Equal(t, 1, healed)
}

func TestHealFallsBackToAnnotationWhenLLMFails(t *testing.T) {
	c := &fakeCompleter{err: errors.New("model down")}
	text, healed, err := newHealer(c).Heal(context.Background(), answer, []report.Claim{staleClaim()})
	require.NoError(t, err)
	assert.Equal(t, heal.Annotate(answer, []report.Claim{staleClaim()}), text)
	assert.Equal(t, 1, healed)
}

func TestHealNothingToCorrect(t *testing.T) {
	fresh := staleClaim()
	fresh.Verdict = report.VerdictFresh
	c := &fakeCompleter{}
	text, healed, err := newHealer(c).Heal(context.Background(), answer, []report.Claim{fresh})
	require.NoError(t, err)
	assert.Equal(t, "", text)
	assert.Equal(t, 0, healed)
	assert.Empty(t, c.requests)
}

func TestHealFallsBackWhenModelEchoesAnswer(t *testing.T) {
	c := &fakeCompleter{reply: replyWith(answer)}
	text, healed, err := newHealer(c).Heal(context.Background(), answer, []report.Claim{staleClaim()})
	require.NoError(t, err)
	assert.Equal(t, heal.Annotate(answer, []report.Claim{staleClaim()}), text)
	assert.Equal(t, 1, healed)
}

func TestHealFallsBackWhenACorrectionIsMissing(t *testing.T) {
	version := report.Claim{ID: "c2", Quote: "4.2", SpanStart: 66, SpanEnd: 69, Value: "4.2",
		Verdict: report.VerdictStale, CurrentValue: "4.3"}
	claims := []report.Claim{staleClaim(), version}
	c := &fakeCompleter{reply: replyWith(strings.Replace(answer, "$20", "$25", 1))} // fixes only the price
	text, _, err := newHealer(c).Heal(context.Background(), answer, claims)
	require.NoError(t, err)
	assert.Equal(t, heal.Annotate(answer, claims), text)
}
