package api_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lucifer-135/freshness-guard/internal/api"
	"github.com/lucifer-135/freshness-guard/internal/events"
)

// recordingEvents wraps a broker and signals when a subscription is cancelled.
type recordingEvents struct {
	b         *events.Broker
	subscribe chan struct{}
	cancelled chan struct{}
}

func (r *recordingEvents) Subscribe() (<-chan events.Event, func()) {
	ch, cancel := r.b.Subscribe()
	close(r.subscribe)
	return ch, func() {
		cancel()
		select {
		case <-r.cancelled:
		default:
			close(r.cancelled)
		}
	}
}

func newEventsServer(t *testing.T, ev api.Events, heartbeat time.Duration) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(api.NewHandler(api.Deps{Token: "t", Events: ev, Heartbeat: heartbeat}))
	t.Cleanup(srv.Close)
	return srv
}

// openStream connects and returns a line reader positioned after ": connected".
func openStream(t *testing.T, ctx context.Context, url string) *bufio.Reader {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer t")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
	r := bufio.NewReader(resp.Body)
	line, err := r.ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, ": connected\n", line)
	return r
}

// readUntil returns the first line with the given prefix, failing after 2 s.
func readUntil(t *testing.T, r *bufio.Reader, prefix string) string {
	t.Helper()
	found := make(chan string, 1)
	go func() {
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				close(found)
				return
			}
			if strings.HasPrefix(line, prefix) {
				found <- strings.TrimSpace(line)
				return
			}
		}
	}()
	select {
	case line, ok := <-found:
		require.True(t, ok, "stream ended before %q", prefix)
		return line
	case <-time.After(2 * time.Second):
		t.Fatalf("no line starting with %q within 2 seconds", prefix)
		return ""
	}
}

func TestEventsStreamsPublishedEvents(t *testing.T) {
	b := events.NewBroker()
	ev := &recordingEvents{b: b, subscribe: make(chan struct{}), cancelled: make(chan struct{})}
	srv := newEventsServer(t, ev, time.Minute)
	r := openStream(t, context.Background(), srv.URL+"/freshness/events")
	<-ev.subscribe
	b.Publish(events.Event{Type: events.TypeReportStarted, ReportID: "r-1"})
	assert.Equal(t, "event: report.started", readUntil(t, r, "event: "))
	data := readUntil(t, r, "data: ")
	var e events.Event
	require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(data, "data: ")), &e))
	assert.Equal(t, "r-1", e.ReportID)
	assert.Equal(t, events.TypeReportStarted, e.Type)
}

func TestEventsFilterByReport(t *testing.T) {
	b := events.NewBroker()
	ev := &recordingEvents{b: b, subscribe: make(chan struct{}), cancelled: make(chan struct{})}
	srv := newEventsServer(t, ev, time.Minute)
	r := openStream(t, context.Background(), srv.URL+"/freshness/events?report=r-2")
	<-ev.subscribe
	b.Publish(events.Event{Type: events.TypeReportStarted, ReportID: "r-1"})
	b.Publish(events.Event{Type: events.TypeReportComplete, ReportID: "r-2"})
	assert.Equal(t, "event: report.complete", readUntil(t, r, "event: "))
}

func TestEventsNeedToken(t *testing.T) {
	srv := newEventsServer(t, events.NewBroker(), time.Minute)
	resp, err := http.Get(srv.URL + "/freshness/events")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestEventsHeartbeat(t *testing.T) {
	srv := newEventsServer(t, events.NewBroker(), 20*time.Millisecond)
	r := openStream(t, context.Background(), srv.URL+"/freshness/events")
	assert.Equal(t, ": ping", readUntil(t, r, ": ping"))
}

func TestEventsUnsubscribeOnDisconnect(t *testing.T) {
	ev := &recordingEvents{b: events.NewBroker(), subscribe: make(chan struct{}), cancelled: make(chan struct{})}
	srv := newEventsServer(t, ev, time.Minute)
	ctx, cancel := context.WithCancel(context.Background())
	openStream(t, ctx, srv.URL+"/freshness/events")
	<-ev.subscribe
	cancel()
	select {
	case <-ev.cancelled:
	case <-time.After(time.Second):
		t.Fatal("the subscription was not cancelled after the client left")
	}
}
