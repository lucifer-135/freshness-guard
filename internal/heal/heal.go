// Package heal corrects outdated claims in an answer with a minimal LLM edit,
// guarded by a character diff, and falls back to inline annotations.
package heal

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/lucifer-135/freshness-guard/internal/llm"
	"github.com/lucifer-135/freshness-guard/internal/report"
)

// Limits for the correction call.
const (
	Timeout   = 6 * time.Second
	MaxTokens = 4096
)

// JSONCompleter is the LLM call the healer needs; *llm.Client implements it.
type JSONCompleter interface {
	CompleteJSON(ctx context.Context, req llm.Request, out any) error
}

// Healer corrects answers.
type Healer struct {
	LLM       JSONCompleter
	Model     string
	Fallbacks []string
}

// Corrections returns the claims an answer can be corrected for: STALE or
// WRONG claims with a current value and a located span.
func Corrections(claims []report.Claim) []report.Claim {
	var out []report.Claim
	for _, c := range claims {
		if (c.Verdict == report.VerdictStale || c.Verdict == report.VerdictWrong) &&
			c.CurrentValue != "" && c.SpanStart >= 0 && c.SpanEnd >= c.SpanStart {
			out = append(out, c)
		}
	}
	return out
}

type correction struct {
	Quote        string `json:"quote"`
	Value        string `json:"value"`
	CurrentValue string `json:"current_value"`
	AsOf         string `json:"as_of"`
}

type healed struct {
	Text string `json:"text"`
}

// Validate rejects an empty correction.
func (h *healed) Validate() error {
	if h.Text == "" {
		return errors.New("text is empty")
	}
	return nil
}

// Heal returns the corrected answer and the number of corrected claims. With
// nothing to correct it returns an empty text without calling the model. When
// the model fails or its edit changes too much, the answer is annotated
// instead, so the error result is reserved for problems with the input.
func (h *Healer) Heal(ctx context.Context, answer string, claims []report.Claim) (string, int, error) {
	fixes := Corrections(claims)
	if len(fixes) == 0 {
		return "", 0, nil
	}
	payload := struct {
		Answer      string       `json:"answer"`
		Corrections []correction `json:"corrections"`
	}{Answer: answer}
	spans := make([][2]int, 0, len(fixes))
	for _, c := range fixes {
		payload.Corrections = append(payload.Corrections,
			correction{Quote: c.Quote, Value: c.Value, CurrentValue: c.CurrentValue, AsOf: c.AsOf})
		spans = append(spans, [2]int{c.SpanStart, c.SpanEnd})
	}
	user, err := json.Marshal(payload)
	if err != nil {
		return "", 0, err
	}

	var out healed
	err = h.LLM.CompleteJSON(ctx, llm.Request{
		Model: h.Model, Fallbacks: h.Fallbacks, System: systemPrompt, User: string(user),
		SchemaName: "healed_answer", Schema: json.RawMessage(schema), MaxTokens: MaxTokens, Timeout: Timeout,
	}, &out)
	if err == nil {
		if ok, _ := Guard(answer, out.Text, spans); ok {
			return out.Text, len(fixes), nil
		}
	}
	return Annotate(answer, fixes), len(fixes), nil
}
