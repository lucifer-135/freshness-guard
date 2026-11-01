package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatCompletionNonStreaming(t *testing.T) {
	rec := httptest.NewRecorder()
	body := `{"model":"fake-model","messages":[{"role":"user","content":"hi"}]}`
	newHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body)))
	assert.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Choices []struct{ Message struct{ Content string } }
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, Answer, resp.Choices[0].Message.Content)
}

func TestChatCompletionStreaming(t *testing.T) {
	rec := httptest.NewRecorder()
	body := `{"model":"fake-model","stream":true,"messages":[{"role":"user","content":"hi"}]}`
	newHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body)))
	assert.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
	assert.Equal(t, 4, strings.Count(rec.Body.String(), "data: "))
	assert.True(t, strings.HasSuffix(strings.TrimSpace(rec.Body.String()), "data: [DONE]"))
}

func TestListModels(t *testing.T) {
	rec := httptest.NewRecorder()
	newHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/models", nil))
	assert.Contains(t, rec.Body.String(), `"fake-model"`)
}

func TestModelsListsAllFakeModels(t *testing.T) {
	rec := httptest.NewRecorder()
	newHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/models", nil))
	var resp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	var ids []string
	for _, m := range resp.Data {
		ids = append(ids, m.ID)
	}
	assert.ElementsMatch(t, []string{"fake-model", ExtractModel, JudgeModel, HealModel}, ids)
}

// chatContent posts a chat request and returns the first choice's content.
func chatContent(t *testing.T, model, user string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"model":    model,
		"messages": []map[string]string{{"role": "system", "content": "s"}, {"role": "user", "content": user}},
	})
	rec := httptest.NewRecorder()
	newHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(body))))
	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Choices []struct{ Message struct{ Content string } }
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	return resp.Choices[0].Message.Content
}

func TestFakeExtractReturnsClaims(t *testing.T) {
	var out struct {
		Claims []struct {
			Quote    string `json:"quote"`
			Category string `json:"category"`
		} `json:"claims"`
	}
	require.NoError(t, json.Unmarshal([]byte(chatContent(t, ExtractModel, "{}")), &out))
	require.Len(t, out.Claims, 2)
	for _, c := range out.Claims {
		assert.Contains(t, Answer, c.Quote)
	}
	assert.Equal(t, "pricing", out.Claims[0].Category)
	assert.Equal(t, "version", out.Claims[1].Category)
}

func TestFakeJudgeAnswersEachClaim(t *testing.T) {
	user := `{"claims":[{"id":"c1","attribute":"monthly price","evidence":[{"ref":"E1"}]},
		{"id":"c2","attribute":"current version","evidence":[{"ref":"E2"}]}]}`
	var out struct {
		Results []struct {
			ClaimID      string   `json:"claim_id"`
			Verdict      string   `json:"verdict"`
			CurrentValue string   `json:"current_value"`
			Supporting   []string `json:"supporting_evidence_ids"`
			Timeline     []struct {
				Source string `json:"source"`
			} `json:"timeline"`
		} `json:"results"`
	}
	require.NoError(t, json.Unmarshal([]byte(chatContent(t, JudgeModel, user)), &out))
	require.Len(t, out.Results, 2)
	assert.Equal(t, "c1", out.Results[0].ClaimID)
	assert.Equal(t, "STALE", out.Results[0].Verdict)
	assert.Equal(t, "$25 per month", out.Results[0].CurrentValue)
	assert.Equal(t, []string{"E1"}, out.Results[0].Supporting)
	require.Len(t, out.Results[0].Timeline, 2)
	assert.Equal(t, "E1", out.Results[0].Timeline[0].Source)
	assert.Equal(t, "c2", out.Results[1].ClaimID)
	assert.Equal(t, "FRESH", out.Results[1].Verdict)
	assert.Equal(t, []string{"E2"}, out.Results[1].Supporting)
}

func search(t *testing.T, auth, query string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader(`{"query":"`+query+`"}`))
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	rec := httptest.NewRecorder()
	newHandler().ServeHTTP(rec, req)
	return rec
}

func TestSearchNeedsKey(t *testing.T) {
	assert.Equal(t, http.StatusUnauthorized, search(t, "", "price").Code)
	assert.Equal(t, http.StatusUnauthorized, search(t, "Bearer wrong", "price").Code)
}

type searchResults struct {
	Results []struct {
		URL           string `json:"url"`
		PublishedDate string `json:"published_date"`
	} `json:"results"`
}

func TestSearchReturnsPricingResults(t *testing.T) {
	rec := search(t, "Bearer "+TavilyKey, "Example Cloud Pro monthly PRICE")
	require.Equal(t, http.StatusOK, rec.Code)
	var out searchResults
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Results, 2)
	assert.Equal(t, "https://example.com/pricing", out.Results[0].URL)
	assert.Equal(t, "2026-09-01", out.Results[0].PublishedDate)
}

func TestSearchReturnsVersionResults(t *testing.T) {
	rec := search(t, "Bearer "+TavilyKey, "Example Cloud Pro latest version")
	var out searchResults
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Results, 2)
	assert.Equal(t, "https://example.com/releases", out.Results[0].URL)
}

func TestRateLimitedModelReturns429(t *testing.T) {
	rec := httptest.NewRecorder()
	body := `{"model":"` + RateLimitedModel + `","messages":[{"role":"user","content":"hi"}]}`
	newHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body)))
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Contains(t, rec.Body.String(), "rate_limit_exceeded")
}

func TestHealthz(t *testing.T) {
	rec := httptest.NewRecorder()
	newHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestFakeHealReplacesValues(t *testing.T) {
	user, _ := json.Marshal(map[string]any{
		"answer":      Answer,
		"corrections": []map[string]string{{"value": "$20 per month", "current_value": "$25 per month"}},
	})
	var out struct {
		Text string `json:"text"`
	}
	require.NoError(t, json.Unmarshal([]byte(chatContent(t, HealModel, string(user))), &out))
	assert.Equal(t, strings.Replace(Answer, "$20 per month", "$25 per month", 1), out.Text)
}
