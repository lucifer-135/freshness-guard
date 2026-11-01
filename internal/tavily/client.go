// Package tavily is a small client for the Tavily search API.
package tavily

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	// ErrCreditLimit is returned after Tavily reports that the plan's credit
	// limit has been reached (HTTP 432 or 433).
	ErrCreditLimit = errors.New("the Tavily credit limit has been reached")
	// ErrNoAPIKey is returned when the client has no API key.
	ErrNoAPIKey = errors.New("no Tavily API key is configured")
)

const (
	// AttemptTimeout limits each HTTP attempt.
	AttemptTimeout = 5 * time.Second
	// MaxRetries is the number of retries after HTTP 429.
	MaxRetries = 3

	maxWait = 10 * time.Second
)

// SearchRequest is one search. Empty fields use the defaults: depth "basic",
// topic "general" and 5 results. MaxAgeHours is sent only when it is set.
type SearchRequest struct {
	Query       string
	Topic       string
	Depth       string
	MaxResults  int
	MaxAgeHours *int
}

// Result is one search result.
type Result struct {
	Title         string
	URL           string
	Content       string
	Score         float64
	PublishedDate *time.Time
}

// SearchResponse holds the results and the credits the search cost.
type SearchResponse struct {
	Results []Result
	Credits int
}

// Client calls the Tavily API. It is safe for concurrent use.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
	sleep   func(context.Context, time.Duration) error
	now     func() time.Time

	mu           sync.Mutex
	breakerUntil time.Time
}

// New returns a client for the Tavily API at baseURL.
func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		http:    &http.Client{},
		sleep:   sleepContext,
		now:     time.Now,
	}
}

// Cost returns the credits a search with the given depth costs.
func Cost(depth string) int {
	if depth == "advanced" {
		return 2
	}
	return 1
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type searchBody struct {
	Query                string `json:"query"`
	SearchDepth          string `json:"search_depth"`
	Topic                string `json:"topic"`
	MaxResults           int    `json:"max_results"`
	IncludePublishedDate bool   `json:"include_published_date"`
	MaxAgeHours          *int   `json:"max_age_hours,omitempty"`
}

type searchReply struct {
	Results []struct {
		Title         string  `json:"title"`
		URL           string  `json:"url"`
		Content       string  `json:"content"`
		Score         float64 `json:"score"`
		PublishedDate string  `json:"published_date"`
	} `json:"results"`
}

// Search runs one search. It retries after HTTP 429 and stops calling Tavily
// until the next UTC midnight after HTTP 432 or 433.
func (c *Client) Search(ctx context.Context, req SearchRequest) (SearchResponse, error) {
	if c.apiKey == "" {
		return SearchResponse{}, ErrNoAPIKey
	}
	if c.breakerOpen() {
		return SearchResponse{}, ErrCreditLimit
	}

	body := searchBody{Query: req.Query, SearchDepth: req.Depth, Topic: req.Topic, MaxResults: req.MaxResults,
		IncludePublishedDate: true, MaxAgeHours: req.MaxAgeHours}
	if body.SearchDepth == "" {
		body.SearchDepth = "basic"
	}
	if body.Topic == "" {
		body.Topic = "general"
	}
	if body.MaxResults == 0 {
		body.MaxResults = 5
	}
	data, err := json.Marshal(body)
	if err != nil {
		return SearchResponse{}, err
	}

	for attempt := 0; ; attempt++ {
		status, retryAfter, payload, err := c.attempt(ctx, data)
		if err != nil {
			return SearchResponse{}, err
		}
		switch {
		case status >= 200 && status <= 299:
			return parseReply(payload, Cost(body.SearchDepth))
		case status == http.StatusTooManyRequests:
			if attempt == MaxRetries {
				return SearchResponse{}, fmt.Errorf("tavily: rate limited after %d retries", MaxRetries)
			}
			if err := c.sleep(ctx, backoff(attempt, retryAfter)); err != nil {
				return SearchResponse{}, err
			}
		case status == 432 || status == 433:
			c.openBreaker()
			return SearchResponse{}, ErrCreditLimit
		default:
			return SearchResponse{}, fmt.Errorf("tavily: search returned HTTP %d", status)
		}
	}
}

func (c *Client) attempt(ctx context.Context, data []byte) (status int, retryAfter string, payload []byte, err error) {
	ctx, cancel := context.WithTimeout(ctx, AttemptTimeout)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/search", bytes.NewReader(data))
	if err != nil {
		return 0, "", nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return 0, "", nil, fmt.Errorf("tavily: %w", err)
	}
	defer resp.Body.Close()
	payload, err = io.ReadAll(resp.Body)
	if err != nil {
		return 0, "", nil, fmt.Errorf("tavily: read reply: %w", err)
	}
	return resp.StatusCode, resp.Header.Get("Retry-After"), payload, nil
}

// backoff returns the wait before the next attempt: Retry-After seconds when
// given, otherwise 500 ms doubled per attempt, plus up to 250 ms of jitter,
// and never more than 10 seconds.
func backoff(attempt int, retryAfter string) time.Duration {
	wait := 500 * time.Millisecond << attempt
	if secs, err := strconv.Atoi(strings.TrimSpace(retryAfter)); err == nil && secs >= 0 {
		wait = time.Duration(secs) * time.Second
	}
	wait += rand.N(250 * time.Millisecond)
	return min(wait, maxWait)
}

func (c *Client) breakerOpen() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now().Before(c.breakerUntil)
}

func (c *Client) openBreaker() {
	now := c.now().UTC()
	c.mu.Lock()
	c.breakerUntil = time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
	c.mu.Unlock()
}

var dateLayouts = []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02", time.RFC1123, time.RFC1123Z}

func parseDate(s string) *time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			t = t.UTC()
			return &t
		}
	}
	return nil
}

func parseReply(payload []byte, credits int) (SearchResponse, error) {
	var reply searchReply
	if err := json.Unmarshal(payload, &reply); err != nil {
		return SearchResponse{}, fmt.Errorf("tavily: decode reply: %w", err)
	}
	resp := SearchResponse{Results: make([]Result, 0, len(reply.Results)), Credits: credits}
	for _, r := range reply.Results {
		resp.Results = append(resp.Results, Result{
			Title: r.Title, URL: r.URL, Content: r.Content, Score: r.Score, PublishedDate: parseDate(r.PublishedDate),
		})
	}
	return resp, nil
}
