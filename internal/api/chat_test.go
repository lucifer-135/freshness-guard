package api_test

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lucifer-135/freshness-guard/internal/api"
	"github.com/lucifer-135/freshness-guard/internal/gateway"
)

type fakeGateway struct {
	resp      *http.Response
	err       error
	models    []gateway.Model
	modelsErr error

	body         string
	mode, policy string
}

func (f *fakeGateway) Chat(_ context.Context, body []byte, mode, policy string) (*http.Response, error) {
	f.body, f.mode, f.policy = string(body), mode, policy
	return f.resp, f.err
}

func (f *fakeGateway) Models(context.Context) ([]gateway.Model, error) { return f.models, f.modelsErr }

type fakeCredits struct{ used, daily int }

func (f fakeCredits) Used() int  { return f.used }
func (f fakeCredits) Daily() int { return f.daily }

func jsonResponse(status int, headers map[string]string, body string) *http.Response {
	h := http.Header{"Content-Type": []string{"application/json"}}
	for k, v := range headers {
		h.Set(k, v)
	}
	return &http.Response{StatusCode: status, Header: h, Body: io.NopCloser(strings.NewReader(body))}
}

func chatHandler(g *fakeGateway) http.Handler {
	return api.NewHandler(api.Deps{Token: "t", Gateway: g, Credits: fakeCredits{used: 3, daily: 30}})
}

const chatBody = `{"model":"openai/fake-model","messages":[{"role":"user","content":"hi"}]}`

func TestChatProxyCopiesStatusHeadersAndBody(t *testing.T) {
	g := &fakeGateway{resp: jsonResponse(200, map[string]string{
		"X-Freshness-Status": "pending", "X-Freshness-Report-Id": "r-1", "X-Other": "no"}, `{"ok":true}`)}
	rec := do(chatHandler(g), http.MethodPost, "/freshness/chat", "t", chatBody)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "pending", rec.Header().Get("X-Freshness-Status"))
	assert.Equal(t, "r-1", rec.Header().Get("X-Freshness-Report-Id"))
	assert.Empty(t, rec.Header().Get("X-Other"))
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.JSONEq(t, `{"ok":true}`, rec.Body.String())
	assert.Equal(t, "async", g.mode)
	assert.Equal(t, "", g.policy)
	assert.JSONEq(t, `{"model":"openai/fake-model","messages":[{"role":"user","content":"hi"}],"stream":false}`, g.body)
}

func TestChatProxyPassesModeAndPolicy(t *testing.T) {
	g := &fakeGateway{resp: jsonResponse(200, nil, `{}`)}
	body := `{"model":"m","messages":[{"role":"user","content":"hi"}],"stream":true,"mode":"sync","policy":"heal"}`
	do(chatHandler(g), http.MethodPost, "/freshness/chat", "t", body)
	assert.Equal(t, "sync", g.mode)
	assert.Equal(t, "heal", g.policy)
	assert.Contains(t, g.body, `"stream":true`)
}

func TestChatProxyStreams(t *testing.T) {
	pr, pw := io.Pipe()
	release := make(chan struct{})
	go func() {
		_, _ = pw.Write([]byte("data: a\n\n"))
		<-release
		_ = pw.Close()
	}()
	g := &fakeGateway{resp: &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: pr}}
	srv := httptest.NewServer(chatHandler(g))
	defer srv.Close()
	defer close(release)

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/freshness/chat", strings.NewReader(chatBody))
	req.Header.Set("Authorization", "Bearer t")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	line := make(chan string, 1)
	go func() {
		l, _ := bufio.NewReader(resp.Body).ReadString('\n')
		line <- l
	}()
	select {
	case l := <-line:
		assert.Equal(t, "data: a\n", l)
	case <-time.After(2 * time.Second):
		t.Fatal("the first chunk did not arrive before the upstream stream ended")
	}
}

func TestChatProxyValidates(t *testing.T) {
	h := chatHandler(&fakeGateway{resp: jsonResponse(200, nil, `{}`)})
	for name, body := range map[string]string{
		"no model":       `{"messages":[{"role":"user","content":"hi"}]}`,
		"empty messages": `{"model":"m","messages":[]}`,
		"bad mode":       `{"model":"m","messages":[{"role":"user","content":"hi"}],"mode":"fast"}`,
		"bad policy":     `{"model":"m","messages":[{"role":"user","content":"hi"}],"policy":"fix"}`,
		"not json":       `nope`,
	} {
		assert.Equal(t, http.StatusBadRequest, do(h, http.MethodPost, "/freshness/chat", "t", body).Code, name)
	}
}

func TestChatProxyGatewayDown(t *testing.T) {
	rec := do(chatHandler(&fakeGateway{err: errors.New("connection refused")}), http.MethodPost, "/freshness/chat", "t", chatBody)
	assert.Equal(t, http.StatusBadGateway, rec.Code)
	assert.Equal(t, "Bifrost could not be reached.", errorOf(t, rec))
}

func TestModelsEndpoint(t *testing.T) {
	g := &fakeGateway{models: []gateway.Model{{ID: "gemini/a", Provider: "gemini"}}}
	rec := do(chatHandler(g), http.MethodGet, "/freshness/models", "t", "")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"models":[{"id":"gemini/a","provider":"gemini"}]}`, rec.Body.String())

	rec = do(chatHandler(&fakeGateway{modelsErr: errors.New("down")}), http.MethodGet, "/freshness/models", "t", "")
	assert.Equal(t, http.StatusBadGateway, rec.Code)
}

func TestStatusEndpoint(t *testing.T) {
	g := &fakeGateway{models: []gateway.Model{{ID: "gemini/a", Provider: "gemini"}}}
	rec := do(chatHandler(g), http.MethodGet, "/freshness/status", "t", "")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"gateway":{"reachable":true,"models":1},"credits":{"used":3,"budget":30}}`, rec.Body.String())

	rec = do(chatHandler(&fakeGateway{modelsErr: errors.New("down")}), http.MethodGet, "/freshness/status", "t", "")
	assert.JSONEq(t, `{"gateway":{"reachable":false,"models":0},"credits":{"used":3,"budget":30}}`, rec.Body.String())
}

func TestChatRoutesNeedToken(t *testing.T) {
	h := chatHandler(&fakeGateway{})
	for _, path := range []string{"/freshness/models", "/freshness/status"} {
		assert.Equal(t, http.StatusUnauthorized, do(h, http.MethodGet, path, "", "").Code, path)
	}
	assert.Equal(t, http.StatusUnauthorized, do(h, http.MethodPost, "/freshness/chat", "", chatBody).Code)
}
