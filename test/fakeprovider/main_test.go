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
	var resp struct{ Choices []struct{ Message struct{ Content string } } }
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
