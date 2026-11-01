package tavily

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeTavily struct {
	mu       sync.Mutex
	statuses []int // status per request; the last one repeats; 0 means 200
	headers  map[string]string
	results  []map[string]any
	requests []map[string]any
	auth     []string
}

func (f *fakeTavily) start(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(data, &body)
		f.mu.Lock()
		f.requests = append(f.requests, body)
		f.auth = append(f.auth, r.Header.Get("Authorization"))
		n := len(f.requests)
		f.mu.Unlock()
		status := 0
		if len(f.statuses) > 0 {
			status = f.statuses[min(n, len(f.statuses))-1]
		}
		if status != 0 && status != http.StatusOK {
			for k, v := range f.headers {
				w.Header().Set(k, v)
			}
			w.WriteHeader(status)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"query": body["query"], "results": f.results})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newTestClient(url string, now time.Time) (*Client, *[]time.Duration) {
	c := New(url, "k")
	sleeps := &[]time.Duration{}
	c.sleep = func(_ context.Context, d time.Duration) error {
		*sleeps = append(*sleeps, d)
		return nil
	}
	c.now = func() time.Time { return now }
	return c, sleeps
}

var fixedNow = time.Date(2026, 11, 3, 15, 0, 0, 0, time.UTC)

func TestSearchSendsRequest(t *testing.T) {
	f := &fakeTavily{}
	c, _ := newTestClient(f.start(t).URL, fixedNow)
	_, err := c.Search(context.Background(), SearchRequest{Query: "q"})
	require.NoError(t, err)
	body := f.requests[0]
	assert.Equal(t, "q", body["query"])
	assert.Equal(t, "basic", body["search_depth"])
	assert.Equal(t, "general", body["topic"])
	assert.Equal(t, 5.0, body["max_results"])
	assert.Equal(t, true, body["include_published_date"])
	assert.NotContains(t, body, "max_age_hours")
	assert.Equal(t, "Bearer k", f.auth[0])

	zero := 0
	_, err = c.Search(context.Background(), SearchRequest{Query: "q", MaxAgeHours: &zero})
	require.NoError(t, err)
	assert.Equal(t, 0.0, f.requests[1]["max_age_hours"])
}

func TestSearchParsesResultsAndDates(t *testing.T) {
	f := &fakeTavily{results: []map[string]any{
		{"title": "A", "url": "https://a", "content": "a", "score": 0.9, "published_date": "2026-09-01"},
		{"title": "B", "url": "https://b", "content": "b", "published_date": "Tue, 01 Sep 2026 10:00:00 GMT"},
		{"title": "C", "url": "https://c", "content": "c", "published_date": ""},
	}}
	c, _ := newTestClient(f.start(t).URL, fixedNow)
	resp, err := c.Search(context.Background(), SearchRequest{Query: "q"})
	require.NoError(t, err)
	require.Len(t, resp.Results, 3)
	assert.Equal(t, "A", resp.Results[0].Title)
	assert.Equal(t, "https://a", resp.Results[0].URL)
	assert.Equal(t, 0.9, resp.Results[0].Score)
	require.NotNil(t, resp.Results[0].PublishedDate)
	assert.True(t, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC).Equal(*resp.Results[0].PublishedDate))
	require.NotNil(t, resp.Results[1].PublishedDate)
	assert.True(t, time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC).Equal(*resp.Results[1].PublishedDate))
	assert.Nil(t, resp.Results[2].PublishedDate)
	assert.Equal(t, 1, resp.Credits)

	resp, err = c.Search(context.Background(), SearchRequest{Query: "q", Depth: "advanced"})
	require.NoError(t, err)
	assert.Equal(t, 2, resp.Credits)
	assert.Equal(t, "advanced", f.requests[1]["search_depth"])
}

func TestSearchRetriesAfter429(t *testing.T) {
	f := &fakeTavily{statuses: []int{429, 200}, headers: map[string]string{"Retry-After": "2"}}
	c, sleeps := newTestClient(f.start(t).URL, fixedNow)
	_, err := c.Search(context.Background(), SearchRequest{Query: "q"})
	require.NoError(t, err)
	assert.Len(t, f.requests, 2)
	require.Len(t, *sleeps, 1)
	assert.GreaterOrEqual(t, (*sleeps)[0], 2*time.Second)
	assert.LessOrEqual(t, (*sleeps)[0], 2250*time.Millisecond)
}

func TestSearchGivesUpAfterThreeRetries(t *testing.T) {
	f := &fakeTavily{statuses: []int{429}}
	c, sleeps := newTestClient(f.start(t).URL, fixedNow)
	_, err := c.Search(context.Background(), SearchRequest{Query: "q"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rate limited")
	assert.Len(t, f.requests, 4)
	require.Len(t, *sleeps, 3)
	for _, d := range *sleeps {
		assert.LessOrEqual(t, d, 10*time.Second)
	}
}

func TestSearchCreditLimitOpensBreaker(t *testing.T) {
	f := &fakeTavily{statuses: []int{432, 200}}
	srv := f.start(t)
	c, _ := newTestClient(srv.URL, fixedNow)
	_, err := c.Search(context.Background(), SearchRequest{Query: "q"})
	assert.ErrorIs(t, err, ErrCreditLimit)
	_, err = c.Search(context.Background(), SearchRequest{Query: "q"})
	assert.ErrorIs(t, err, ErrCreditLimit)
	assert.Len(t, f.requests, 1)

	c.now = func() time.Time { return fixedNow.Add(10 * time.Hour) } // next UTC day
	_, err = c.Search(context.Background(), SearchRequest{Query: "q"})
	require.NoError(t, err)
	assert.Len(t, f.requests, 2)
}

func TestSearchPlanLimit433AlsoOpensBreaker(t *testing.T) {
	f := &fakeTavily{statuses: []int{433}}
	c, _ := newTestClient(f.start(t).URL, fixedNow)
	_, err := c.Search(context.Background(), SearchRequest{Query: "q"})
	assert.ErrorIs(t, err, ErrCreditLimit)
}

func TestSearchWithoutKey(t *testing.T) {
	f := &fakeTavily{}
	c := New(f.start(t).URL, "")
	_, err := c.Search(context.Background(), SearchRequest{Query: "q"})
	assert.ErrorIs(t, err, ErrNoAPIKey)
	assert.Empty(t, f.requests)
}

func TestSearchOtherStatusIsError(t *testing.T) {
	f := &fakeTavily{statuses: []int{500}}
	c, sleeps := newTestClient(f.start(t).URL, fixedNow)
	_, err := c.Search(context.Background(), SearchRequest{Query: "q"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
	assert.Empty(t, *sleeps)
}

func TestCost(t *testing.T) {
	assert.Equal(t, 1, Cost("basic"))
	assert.Equal(t, 1, Cost(""))
	assert.Equal(t, 2, Cost("advanced"))
}
