package gateway_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lucifer-135/freshness-guard/internal/gateway"
)

func TestChatSendsFreshnessHeaders(t *testing.T) {
	var got *http.Request
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Clone(context.Background())
		body, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	resp, err := gateway.New(srv.URL, "vk", "").Chat(context.Background(), []byte(`{"model":"m"}`), "sync", "heal")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "/v1/chat/completions", got.URL.Path)
	assert.Equal(t, "sync", got.Header.Get("x-bf-freshness"))
	assert.Equal(t, "heal", got.Header.Get("x-bf-freshness-policy"))
	assert.Equal(t, "vk", got.Header.Get("x-bf-vk"))
	assert.Equal(t, "application/json", got.Header.Get("Content-Type"))
	assert.JSONEq(t, `{"model":"m"}`, string(body))
}

func TestChatOmitsEmptyPolicyAndKey(t *testing.T) {
	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { got = r.Header.Clone() }))
	defer srv.Close()
	resp, err := gateway.New(srv.URL, "", "").Chat(context.Background(), []byte(`{}`), "async", "")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Empty(t, got.Get("x-bf-freshness-policy"))
	assert.Empty(t, got.Get("x-bf-vk"))
	assert.Equal(t, "async", got.Get("x-bf-freshness"))
}

func TestModelsParsesAndSorts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/models", r.URL.Path)
		_, _ = w.Write([]byte(`{"data":[{"id":"groq/b"},{"id":"gemini/a"},{"id":"plain"}]}`))
	}))
	defer srv.Close()
	models, err := gateway.New(srv.URL, "", "").Models(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []gateway.Model{{ID: "gemini/a", Provider: "gemini"}, {ID: "groq/b", Provider: "groq"}, {ID: "plain", Provider: ""}}, models)
}

func TestModelsReportsHTTPErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(500) }))
	defer srv.Close()
	_, err := gateway.New(srv.URL, "", "").Models(context.Background())
	assert.Error(t, err)
}

func TestChatSendsCacheKey(t *testing.T) {
	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { got = r.Header.Clone() }))
	defer srv.Close()

	resp, err := gateway.New(srv.URL, "", "web-chat").Chat(context.Background(), []byte(`{}`), "async", "")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, "web-chat", got.Get("x-bf-cache-key"))

	resp, err = gateway.New(srv.URL, "", "").Chat(context.Background(), []byte(`{}`), "async", "")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Empty(t, got.Get("x-bf-cache-key"))
}
