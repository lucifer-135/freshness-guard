package llm_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lucifer-135/freshness-guard/internal/llm"
)

type recorded struct {
	header http.Header
	body   map[string]any
}

// fakeServer replies with the given contents in order (the last one repeats).
type fakeServer struct {
	mu       sync.Mutex
	replies  []string
	status   int
	body     string
	delay    time.Duration
	requests []recorded
}

func (f *fakeServer) start(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(data, &body)
		f.mu.Lock()
		f.requests = append(f.requests, recorded{header: r.Header.Clone(), body: body})
		n := len(f.requests)
		f.mu.Unlock()
		if f.delay > 0 {
			time.Sleep(f.delay)
		}
		if f.status != 0 {
			w.WriteHeader(f.status)
			_, _ = w.Write([]byte(f.body))
			return
		}
		reply := f.replies[min(n, len(f.replies))-1]
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": reply}}},
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

type out struct {
	A int `json:"a"`
}

type validated struct {
	A int `json:"a"`
}

func (v *validated) Validate() error {
	if v.A == 0 {
		return errors.New("a must not be zero")
	}
	return nil
}

func TestCompleteSendsHeadersAndBody(t *testing.T) {
	f := &fakeServer{replies: []string{"hello"}}
	srv := f.start(t)
	got, err := llm.New(srv.URL, "vk-1").Complete(context.Background(), llm.Request{
		Model: "groq/x", Fallbacks: []string{"gemini/y"}, System: "s", User: "u",
		SchemaName: "claims", Schema: json.RawMessage(`{"type":"object"}`), MaxTokens: 2048,
	})
	require.NoError(t, err)
	assert.Equal(t, "hello", got)

	req := f.requests[0]
	assert.Equal(t, "off", req.header.Get("x-bf-freshness"))
	assert.Equal(t, "vk-1", req.header.Get("x-bf-vk"))
	assert.Equal(t, "groq/x", req.body["model"])
	assert.Equal(t, []any{"gemini/y"}, req.body["fallbacks"])
	assert.Equal(t, 0.0, req.body["temperature"])
	assert.Equal(t, 2048.0, req.body["max_completion_tokens"])
	format := req.body["response_format"].(map[string]any)
	assert.Equal(t, "json_schema", format["type"])
	assert.Equal(t, "claims", format["json_schema"].(map[string]any)["name"])
	messages := req.body["messages"].([]any)
	require.Len(t, messages, 2)
	assert.Equal(t, "system", messages[0].(map[string]any)["role"])
	assert.Equal(t, "user", messages[1].(map[string]any)["role"])
	assert.Equal(t, "u", messages[1].(map[string]any)["content"])
}

func TestCompleteOmitsEmptyOptionalFields(t *testing.T) {
	f := &fakeServer{replies: []string{"hello"}}
	srv := f.start(t)
	_, err := llm.New(srv.URL, "").Complete(context.Background(), llm.Request{Model: "m", System: "s", User: "u"})
	require.NoError(t, err)
	req := f.requests[0]
	assert.Empty(t, req.header.Get("x-bf-vk"))
	assert.NotContains(t, req.body, "fallbacks")
	assert.NotContains(t, req.body, "max_completion_tokens")
	assert.NotContains(t, req.body, "response_format")
}

func TestCompleteReturnsErrorOnHTTPStatus(t *testing.T) {
	f := &fakeServer{status: http.StatusTooManyRequests, body: "rate limited"}
	srv := f.start(t)
	_, err := llm.New(srv.URL, "").Complete(context.Background(), llm.Request{Model: "m"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "429")
	assert.Contains(t, err.Error(), "rate limited")
}

func TestCompleteJSONStripsCodeFence(t *testing.T) {
	f := &fakeServer{replies: []string{"```json\n{\"a\":1}\n```"}}
	srv := f.start(t)
	var o out
	require.NoError(t, llm.New(srv.URL, "").CompleteJSON(context.Background(), llm.Request{Model: "m"}, &o))
	assert.Equal(t, 1, o.A)
}

func TestCompleteJSONRepairsOnce(t *testing.T) {
	f := &fakeServer{replies: []string{"not json", `{"a":1}`}}
	srv := f.start(t)
	var o out
	require.NoError(t, llm.New(srv.URL, "").CompleteJSON(context.Background(), llm.Request{Model: "m", System: "s", User: "u"}, &o))
	assert.Equal(t, 1, o.A)
	require.Len(t, f.requests, 2)
	messages := f.requests[1].body["messages"].([]any)
	require.Len(t, messages, 4)
	assert.Equal(t, "assistant", messages[2].(map[string]any)["role"])
	assert.Equal(t, "not json", messages[2].(map[string]any)["content"])
	assert.Contains(t, messages[3].(map[string]any)["content"], "could not be used")
}

func TestCompleteJSONFailsAfterSecondMalformed(t *testing.T) {
	f := &fakeServer{replies: []string{"x", "x"}}
	srv := f.start(t)
	var o out
	err := llm.New(srv.URL, "").CompleteJSON(context.Background(), llm.Request{Model: "m"}, &o)
	assert.ErrorIs(t, err, llm.ErrMalformed)
	assert.Len(t, f.requests, 2)
}

func TestCompleteJSONRunsValidator(t *testing.T) {
	f := &fakeServer{replies: []string{`{"a":0}`, `{"a":2}`}}
	srv := f.start(t)
	var v validated
	require.NoError(t, llm.New(srv.URL, "").CompleteJSON(context.Background(), llm.Request{Model: "m"}, &v))
	assert.Equal(t, 2, v.A)
	assert.Len(t, f.requests, 2)
}

func TestCompleteJSONDoesNotRepairHTTPErrors(t *testing.T) {
	f := &fakeServer{status: http.StatusInternalServerError, body: "down"}
	srv := f.start(t)
	var o out
	err := llm.New(srv.URL, "").CompleteJSON(context.Background(), llm.Request{Model: "m"}, &o)
	require.Error(t, err)
	assert.NotErrorIs(t, err, llm.ErrMalformed)
	assert.Len(t, f.requests, 1)
}

func TestCompleteHonoursTimeout(t *testing.T) {
	f := &fakeServer{replies: []string{"late"}, delay: 200 * time.Millisecond}
	srv := f.start(t)
	_, err := llm.New(srv.URL, "").Complete(context.Background(), llm.Request{Model: "m", Timeout: 50 * time.Millisecond})
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestCompleteRejectsHugeReply(t *testing.T) {
	f := &fakeServer{replies: []string{strings.Repeat("a", 5<<20)}}
	srv := f.start(t)
	_, err := llm.New(srv.URL, "").Complete(context.Background(), llm.Request{Model: "m"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too large")
}

func TestInternalCallsBypassSemanticCache(t *testing.T) {
	f := &fakeServer{replies: []string{"ok"}}
	srv := f.start(t)
	client := llm.New(srv.URL, "")
	for range 2 {
		_, err := client.Complete(context.Background(), llm.Request{Model: "groq/x", User: "u"})
		require.NoError(t, err)
	}

	require.Len(t, f.requests, 2)
	keys := map[string]bool{}
	for _, req := range f.requests {
		assert.Equal(t, "true", req.header.Get("x-bf-cache-no-store"))
		// A direct lookup is a hash fetch; the default would also embed the prompt
		// with the cache's embedding model on every internal call.
		assert.Equal(t, "direct", req.header.Get("x-bf-cache-type"))
		key := req.header.Get("x-bf-cache-key")
		assert.True(t, strings.HasPrefix(key, "fg-internal-"), key)
		keys[key] = true
	}
	assert.Len(t, keys, 2, "each call needs its own cache key")
}

// modelServer answers per model: slow models wait, failing models return an HTTP error.
// seenModels records each request's model and fallbacks. The handler writes
// from the server's goroutine, so reads take the same lock: a test whose client
// gave up can read while the slow handler is still running.
type seenModels struct {
	mu   sync.Mutex
	list []string
}

func (s *seenModels) add(entry string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.list = append(s.list, entry)
}

func (s *seenModels) all() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.list...)
}

func modelServer(t *testing.T, slow map[string]time.Duration, failing map[string]int, seen *seenModels) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model     string   `json:"model"`
			Fallbacks []string `json:"fallbacks"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		seen.add(body.Model + "|" + strings.Join(body.Fallbacks, ","))
		if d := slow[body.Model]; d > 0 {
			select {
			case <-time.After(d):
			case <-r.Context().Done():
				return
			}
		}
		if code := failing[body.Model]; code != 0 {
			w.WriteHeader(code)
			_, _ = w.Write([]byte(`{"error":{"code":"json_validate_failed"}}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": `{"a":7}`}}},
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

// Bifrost only falls back on a provider error, and the plugin's deadline cancels the
// whole request, so a slow or rejected primary must hand over to the fallback here.
func TestCompleteJSONFallsBackWhenThePrimaryIsRejected(t *testing.T) {
	var seen seenModels
	srv := modelServer(t, nil, map[string]int{"groq/a": http.StatusBadRequest}, &seen)
	var o out
	err := llm.New(srv.URL, "").CompleteJSON(context.Background(), llm.Request{Model: "groq/a", Fallbacks: []string{"gemini/b", "gemini/c"}}, &o)
	require.NoError(t, err)
	assert.Equal(t, 7, o.A)
	assert.Equal(t, []string{"groq/a|gemini/b,gemini/c", "gemini/b|gemini/c"}, seen.all())
}

func TestCompleteJSONFallsBackWhenThePrimaryTimesOut(t *testing.T) {
	var seen seenModels
	srv := modelServer(t, map[string]time.Duration{"groq/a": time.Second}, nil, &seen)
	var o out
	start := time.Now()
	err := llm.New(srv.URL, "").CompleteJSON(context.Background(),
		llm.Request{Model: "groq/a", Fallbacks: []string{"gemini/b"}, Timeout: 100 * time.Millisecond}, &o)
	require.NoError(t, err)
	assert.Equal(t, 7, o.A)
	assert.Less(t, time.Since(start), 900*time.Millisecond)
}

func TestCompleteJSONDoesNotFallBackWhenTheCallerGaveUp(t *testing.T) {
	var seen seenModels
	srv := modelServer(t, map[string]time.Duration{"groq/a": time.Second}, nil, &seen)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	var o out
	err := llm.New(srv.URL, "").CompleteJSON(ctx, llm.Request{Model: "groq/a", Fallbacks: []string{"gemini/b"}}, &o)
	require.Error(t, err)
	assert.Len(t, seen.all(), 1)
}

func TestCompleteJSONReportsBothFailures(t *testing.T) {
	var seen seenModels
	srv := modelServer(t, nil, map[string]int{"groq/a": http.StatusBadRequest, "gemini/b": http.StatusTooManyRequests}, &seen)
	var o out
	err := llm.New(srv.URL, "").CompleteJSON(context.Background(), llm.Request{Model: "groq/a", Fallbacks: []string{"gemini/b"}}, &o)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "groq/a")
	assert.Contains(t, err.Error(), "gemini/b")
}
