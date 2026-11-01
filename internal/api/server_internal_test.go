package api

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lucifer-135/freshness-guard/internal/events"
	"github.com/lucifer-135/freshness-guard/internal/gateway"
)

func TestServeErrorIsLogged(t *testing.T) {
	var buf bytes.Buffer
	s := NewServer("127.0.0.1:0", NewHandler(Deps{}), slog.New(slog.NewTextHandler(&buf, nil)))
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	done := make(chan struct{})
	go func() { s.serve(l); close(done) }()
	require.NoError(t, l.Close())
	<-done
	assert.Contains(t, buf.String(), "stopped unexpectedly")
}

func TestShutdownIsNotLogged(t *testing.T) {
	var buf bytes.Buffer
	s := NewServer("127.0.0.1:0", NewHandler(Deps{}), slog.New(slog.NewTextHandler(&buf, nil)))
	require.NoError(t, s.Start())
	require.NoError(t, s.Shutdown(context.Background()))
	assert.NotContains(t, buf.String(), "stopped unexpectedly")
}

func TestServerTimeouts(t *testing.T) {
	s := NewServer("127.0.0.1:0", NewHandler(Deps{}), slog.Default())
	assert.Equal(t, 30*time.Second, s.http.ReadTimeout)
	assert.Equal(t, 120*time.Second, s.http.IdleTimeout)
	assert.Equal(t, time.Duration(0), s.http.WriteTimeout, "event streams must not be cut off")
}

// slowGateway answers after 500 ms and, like the real client, stops when the
// request context ends.
type slowGateway struct{}

func (slowGateway) Chat(ctx context.Context, _ []byte, _, _ string) (*http.Response, error) {
	pr, pw := io.Pipe()
	go func() {
		select {
		case <-time.After(500 * time.Millisecond):
			_, _ = pw.Write([]byte("data: late\n\n"))
			_ = pw.Close()
		case <-ctx.Done():
			_ = pw.CloseWithError(ctx.Err())
		}
	}()
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: pr}, nil
}
func (g slowGateway) Models(context.Context) ([]gateway.Model, error) { return nil, nil }

// startShortReadTimeout serves deps on a real server whose ReadTimeout is
// 200 ms and returns its base URL.
func startShortReadTimeout(t *testing.T, deps Deps) string {
	t.Helper()
	s := NewServer("127.0.0.1:0", NewHandler(deps), slog.Default())
	s.http.ReadTimeout = 200 * time.Millisecond
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go s.serve(l)
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })
	return "http://" + l.Addr().String()
}

func readLine(t *testing.T, r *bufio.Reader, within time.Duration) string {
	t.Helper()
	got := make(chan string, 1)
	go func() {
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				got <- "error: " + err.Error()
				return
			}
			if line != "\n" { // skip the blank lines that end each event
				got <- line
				return
			}
		}
	}()
	select {
	case line := <-got:
		return line
	case <-time.After(within):
		t.Fatal("no line in time")
		return ""
	}
}

func TestEventStreamOutlivesReadTimeout(t *testing.T) {
	b := events.NewBroker()
	base := startShortReadTimeout(t, Deps{Token: "t", Events: b})
	req, _ := http.NewRequest(http.MethodGet, base+"/freshness/events", nil)
	req.Header.Set("Authorization", "Bearer t")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	r := bufio.NewReader(resp.Body)
	require.Equal(t, ": connected\n", readLine(t, r, time.Second))

	time.Sleep(500 * time.Millisecond)
	b.Publish(events.Event{Type: events.TypeReportStarted, ReportID: "r-1"})
	assert.Equal(t, "id: 1\n", readLine(t, r, 2*time.Second), "the stream must still be open after the read timeout")
}

func TestChatProxyOutlivesReadTimeout(t *testing.T) {
	base := startShortReadTimeout(t, Deps{Token: "t", Gateway: slowGateway{}})
	req, _ := http.NewRequest(http.MethodPost, base+"/freshness/chat",
		strings.NewReader(`{"model":"m","messages":[{"role":"user","content":"hi"}],"stream":true}`))
	req.Header.Set("Authorization", "Bearer t")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, "data: late\n", readLine(t, bufio.NewReader(resp.Body), 2*time.Second))
}
