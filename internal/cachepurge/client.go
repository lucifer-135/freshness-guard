// Package cachepurge removes outdated answers from Bifrost's semantic cache.
package cachepurge

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// requestTimeout bounds one call to Bifrost's management API.
const requestTimeout = 5 * time.Second

// maxErrorBody limits how much of an error reply is kept.
const maxErrorBody = 512

// ClearError is returned when Bifrost refuses to clear a cache entry.
type ClearError struct {
	Status int
	Body   string
}

func (e *ClearError) Error() string {
	return fmt.Sprintf("Bifrost answered HTTP %d when clearing a cache entry: %s", e.Status, e.Body)
}

// Client calls Bifrost's cache management API. While dashboard authentication
// is off, Bifrost accepts management calls only with its setup token.
type Client struct {
	BaseURL    string
	SetupToken string
	HTTP       *http.Client // nil means http.DefaultClient
}

// Clear removes the semantic cache entry with the given ID.
func (c *Client) Clear(ctx context.Context, cacheID string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	target := strings.TrimRight(c.BaseURL, "/") + "/api/cache/clear/" + url.PathEscape(cacheID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, target, nil)
	if err != nil {
		return err
	}
	if c.SetupToken != "" {
		req.Header.Set("X-Bifrost-Setup-Token", c.SetupToken)
	}
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("call Bifrost to clear a cache entry: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return &ClearError{Status: resp.StatusCode, Body: strings.TrimSpace(string(body))}
	}
	return nil
}
