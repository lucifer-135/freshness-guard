//go:build e2e

// Package e2e runs requests against a live Bifrost with the plugin loaded.
// Start the stack first: docker compose -f deploy/docker-compose.yml up -d bifrost
package e2e

import (
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func bifrostURL() string { return os.Getenv("BIFROST_URL") }

func waitReady(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(bifrostURL() + "/health")
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

func postChat(t *testing.T, body string) *http.Response {
	t.Helper()
	waitReady(t)
	resp, err := http.Post(bifrostURL()+"/v1/chat/completions", "application/json", strings.NewReader(body))
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

func TestChatHasFreshnessHeader(t *testing.T) {
	resp := postChat(t, `{"model":"openai/fake-model","messages":[{"role":"user","content":"What does Example Cloud Pro cost?"}]}`)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "skipped", resp.Header.Get("X-Freshness-Status"))
	assert.Contains(t, readBody(t, resp), "Example Cloud Pro costs $20 per month")
}

func TestStreamingChatHasFreshnessHeader(t *testing.T) {
	resp := postChat(t, `{"model":"openai/fake-model","stream":true,"messages":[{"role":"user","content":"hi"}]}`)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "skipped", resp.Header.Get("X-Freshness-Status"))
	body := readBody(t, resp)
	assert.Contains(t, body, "data: [DONE]")
}

func TestPluginHealthEndpoint(t *testing.T) {
	waitReady(t)
	resp, err := http.Get(os.Getenv("FG_URL") + "/freshness/health")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.JSONEq(t, `{"status":"ok","plugin":"freshness-guard"}`, readBody(t, resp))
}
