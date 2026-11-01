//go:build e2e

// Package e2e runs requests against a live Bifrost with the plugin loaded.
// Start the stack first: docker compose -f deploy/docker-compose.yml --profile tools up -d --build bifrost-e2e
package e2e

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// answer repeats fakeprovider.Answer, which a test package cannot import.
const answer = "Example Cloud Pro costs $20 per month, and its current version is 4.2."

var client = &http.Client{Timeout: 10 * time.Second}

func bifrostURL() string { return os.Getenv("BIFROST_URL") }
func pluginURL() string  { return os.Getenv("FG_URL") }
func token() string      { return os.Getenv("FG_TOKEN") }

func waitReady(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(bifrostURL() + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("Bifrost at %s did not become ready within 60 seconds", bifrostURL())
}

func postChat(t *testing.T, body string, headers map[string]string) *http.Response {
	t.Helper()
	waitReady(t)
	req, err := http.NewRequest(http.MethodPost, bifrostURL()+"/v1/chat/completions", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	require.NoError(t, err)
	return resp
}

func pluginRequest(t *testing.T, method, path, body string, withToken bool) *http.Response {
	t.Helper()
	waitReady(t)
	req, err := http.NewRequest(method, pluginURL()+path, strings.NewReader(body))
	require.NoError(t, err)
	if withToken {
		req.Header.Set("Authorization", "Bearer "+token())
	}
	resp, err := client.Do(req)
	require.NoError(t, err)
	return resp
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(data)
}

type claim struct {
	Quote        string `json:"quote"`
	SpanStart    int    `json:"span_start"`
	SpanEnd      int    `json:"span_end"`
	Verdict      string `json:"verdict"`
	CurrentValue string `json:"current_value"`
	Score        *int   `json:"score"`
}

type freshnessReport struct {
	ID           string  `json:"id"`
	Status       string  `json:"status"`
	Reason       string  `json:"reason"`
	Score        *int    `json:"score"`
	StaleCount   int     `json:"stale_count"`
	CheckedCount int     `json:"checked_count"`
	Claims       []claim `json:"claims"`
}

// getReport polls the report until it is no longer pending.
func getReport(t *testing.T, id string) freshnessReport {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		resp := pluginRequest(t, http.MethodGet, "/freshness/reports/"+id, "", true)
		body := readBody(t, resp)
		require.Equal(t, http.StatusOK, resp.StatusCode, body)
		var r freshnessReport
		require.NoError(t, json.Unmarshal([]byte(body), &r))
		if r.Status != "pending" {
			return r
		}
		require.True(t, time.Now().Before(deadline), "report %s stayed pending for 30 seconds", id)
		time.Sleep(500 * time.Millisecond)
	}
}

func assertAnswerReport(t *testing.T, r freshnessReport) {
	t.Helper()
	require.Equal(t, "complete", r.Status, r.Reason)
	require.Len(t, r.Claims, 2)
	assert.Equal(t, 1, r.StaleCount)
	assert.Equal(t, 2, r.CheckedCount)
	assert.NotNil(t, r.Score)

	stale := r.Claims[0]
	assert.Equal(t, "STALE", stale.Verdict)
	assert.Equal(t, "$25 per month", stale.CurrentValue)
	assert.Equal(t, strings.Index(answer, stale.Quote), stale.SpanStart)
	assert.Equal(t, stale.SpanStart+len(stale.Quote), stale.SpanEnd)
	require.NotNil(t, stale.Score)
	assert.LessOrEqual(t, *stale.Score, 20)

	fresh := r.Claims[1]
	assert.Equal(t, "FRESH", fresh.Verdict)
	require.NotNil(t, fresh.Score)
	assert.GreaterOrEqual(t, *fresh.Score, 60)
}

func TestChatReturnsPendingReport(t *testing.T) {
	resp := postChat(t, `{"model":"openai/fake-model","messages":[{"role":"user","content":"What does Example Cloud Pro cost?"}]}`, nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "pending", resp.Header.Get("X-Freshness-Status"))
	id := resp.Header.Get("X-Freshness-Report-Id")
	require.NotEmpty(t, id)
	assert.Contains(t, readBody(t, resp), "Example Cloud Pro costs $20 per month")
	assertAnswerReport(t, getReport(t, id))
}

// Bifrost runs the plugin's LLM hooks once per attempt; the answer from the
// fallback must still be checked.
func TestFallbackAnswerIsChecked(t *testing.T) {
	resp := postChat(t, `{"model":"openai/fake-ratelimited","fallbacks":["openai/fake-model"],
		"messages":[{"role":"user","content":"What does Example Cloud Pro cost?"}]}`, nil)
	body := readBody(t, resp)
	require.Equal(t, http.StatusOK, resp.StatusCode, body)
	assert.Contains(t, body, "Example Cloud Pro costs $20 per month")
	assert.Equal(t, "pending", resp.Header.Get("X-Freshness-Status"))
	id := resp.Header.Get("X-Freshness-Report-Id")
	require.NotEmpty(t, id)
	assertAnswerReport(t, getReport(t, id))
}

func TestStreamingChatIsSkippedAndIntact(t *testing.T) {
	resp := postChat(t, `{"model":"openai/fake-model","stream":true,"messages":[{"role":"user","content":"hi"}]}`, nil)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "skipped", resp.Header.Get("X-Freshness-Status"))

	var text strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		data, ok := strings.CutPrefix(scanner.Text(), "data: ")
		if !ok || data == "[DONE]" {
			continue
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		require.NoError(t, json.Unmarshal([]byte(data), &chunk))
		for _, c := range chunk.Choices {
			text.WriteString(c.Delta.Content)
		}
	}
	require.NoError(t, scanner.Err())
	assert.Equal(t, answer, text.String())
}

func TestOffModeHasNoFreshnessHeaders(t *testing.T) {
	resp := postChat(t, `{"model":"openai/fake-model","messages":[{"role":"user","content":"hi"}]}`,
		map[string]string{"x-bf-freshness": "off"})
	readBody(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Empty(t, resp.Header.Get("X-Freshness-Status"))
	assert.Empty(t, resp.Header.Get("X-Freshness-Report-Id"))
}

func TestCheckEndpointVerifiesText(t *testing.T) {
	body, _ := json.Marshal(map[string]string{"text": answer})
	resp := pluginRequest(t, http.MethodPost, "/freshness/check", string(body), true)
	data := readBody(t, resp)
	require.Equal(t, http.StatusOK, resp.StatusCode, data)
	var r freshnessReport
	require.NoError(t, json.Unmarshal([]byte(data), &r))
	assertAnswerReport(t, r)
}

func TestReportsNeedToken(t *testing.T) {
	resp := pluginRequest(t, http.MethodGet, "/freshness/reports", "", false)
	readBody(t, resp)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestInternalCallsAreNotChecked(t *testing.T) {
	// Make sure at least one answer has been checked, so internal calls happened.
	resp := postChat(t, `{"model":"openai/fake-model","messages":[{"role":"user","content":"What does it cost?"}]}`, nil)
	readBody(t, resp)
	getReport(t, resp.Header.Get("X-Freshness-Report-Id"))

	for _, model := range []string{"openai/fake-extract", "openai/fake-judge"} {
		resp := pluginRequest(t, http.MethodGet, "/freshness/reports?model="+model, "", true)
		var body struct {
			Reports []json.RawMessage `json:"reports"`
		}
		require.NoError(t, json.Unmarshal([]byte(readBody(t, resp)), &body))
		assert.Empty(t, body.Reports, model)
	}
}

func TestPluginHealthEndpoint(t *testing.T) {
	resp := pluginRequest(t, http.MethodGet, "/freshness/health", "", false)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.JSONEq(t, `{"status":"ok","plugin":"freshness-guard"}`, readBody(t, resp))
}
