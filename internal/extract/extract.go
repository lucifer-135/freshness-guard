// Package extract finds checkable factual claims in an answer with one LLM call
// and locates each claim's quote in the answer.
package extract

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lucifer-135/freshness-guard/internal/llm"
	"github.com/lucifer-135/freshness-guard/internal/report"
)

// Limits for the extraction call.
const (
	Timeout        = 6 * time.Second
	MaxTokens      = 2048
	MaxAnswerRunes = 12000
	MaxPromptRunes = 2000
)

// JSONCompleter is the LLM call the extractor needs; *llm.Client implements it.
type JSONCompleter interface {
	CompleteJSON(ctx context.Context, req llm.Request, out any) error
}

// Input is the answer to extract claims from and its context.
type Input struct {
	Answer string
	Prompt string
	Model  string
	At     time.Time
}

// Extractor extracts claims with an LLM.
type Extractor struct {
	LLM       JSONCompleter
	Model     string
	Fallbacks []string
	MaxClaims int
}

type rawClaim struct {
	Quote          string `json:"quote"`
	Entity         string `json:"entity"`
	Attribute      string `json:"attribute"`
	Value          string `json:"value"`
	Category       string `json:"category"`
	TemporalMarker string `json:"temporal_marker"`
	SearchQuery    string `json:"search_query"`
}

type extraction struct {
	Claims []rawClaim `json:"claims"`
}

// Validate rejects a reply without a claims list.
func (e *extraction) Validate() error {
	if e.Claims == nil {
		return errors.New("claims is missing")
	}
	return nil
}

type userPayload struct {
	Today     string `json:"today"`
	Model     string `json:"model"`
	Question  string `json:"question"`
	Answer    string `json:"answer"`
	MaxClaims int    `json:"max_claims"`
}

var categories = map[report.Category]bool{
	report.CategoryPricing: true, report.CategoryVersion: true, report.CategoryLeadership: true,
	report.CategoryLaw: true, report.CategoryStatistic: true, report.CategoryEvent: true,
	report.CategoryFeature: true, report.CategoryStatic: true,
}

// Extract returns the claims in in.Answer, with IDs c1, c2, ... and spans.
func (e *Extractor) Extract(ctx context.Context, in Input) ([]report.Claim, error) {
	user, err := json.Marshal(userPayload{
		Today:     in.At.UTC().Format("2006-01-02"),
		Model:     in.Model,
		Question:  truncateRunes(in.Prompt, MaxPromptRunes),
		Answer:    truncateRunes(in.Answer, MaxAnswerRunes),
		MaxClaims: e.MaxClaims,
	})
	if err != nil {
		return nil, err
	}
	req := llm.Request{
		Model:      e.Model,
		Fallbacks:  e.Fallbacks,
		System:     fmt.Sprintf(systemPrompt, e.MaxClaims),
		User:       string(user),
		SchemaName: "claims",
		Schema:     json.RawMessage(schema),
		MaxTokens:  MaxTokens,
		Timeout:    Timeout,
	}
	var out extraction
	if err := e.LLM.CompleteJSON(ctx, req, &out); err != nil {
		return nil, err
	}

	claims := make([]report.Claim, 0, len(out.Claims))
	for _, raw := range out.Claims {
		c := report.Claim{
			Quote:          strings.TrimSpace(raw.Quote),
			Entity:         strings.TrimSpace(raw.Entity),
			Attribute:      strings.TrimSpace(raw.Attribute),
			Value:          strings.TrimSpace(raw.Value),
			Category:       report.Category(strings.ToLower(strings.TrimSpace(raw.Category))),
			TemporalMarker: strings.TrimSpace(raw.TemporalMarker),
			SearchQuery:    strings.TrimSpace(raw.SearchQuery),
			Verdict:        report.VerdictNotChecked,
		}
		if c.Quote == "" || c.Entity == "" || c.Attribute == "" {
			continue
		}
		if !categories[c.Category] {
			c.Category = report.CategoryStatic
		}
		if len(claims) == e.MaxClaims {
			break
		}
		c.ID = fmt.Sprintf("c%d", len(claims)+1)
		c.SpanStart, c.SpanEnd, _ = Locate(in.Answer, c.Quote)
		claims = append(claims, c)
	}
	return claims, nil
}

func truncateRunes(s string, n int) string {
	count := 0
	for i := range s {
		if count == n {
			return s[:i]
		}
		count++
	}
	return s
}
