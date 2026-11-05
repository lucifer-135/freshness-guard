// Package gateway calls the Bifrost instance the plugin runs in, on behalf of
// the web app: chat completions (passed through) and the model list.
package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

// MaxModelsBytes limits the size of the model list read from Bifrost.
const MaxModelsBytes = 8 << 20

// Model is one model Bifrost can route to. Provider is the part of ID before
// the first "/".
type Model struct {
	ID       string `json:"id"`
	Provider string `json:"provider"`
}

// Client calls Bifrost's OpenAI-compatible endpoints.
type Client struct {
	baseURL    string
	virtualKey string
	cacheKey   string
	http       *http.Client
}

// New returns a client for the Bifrost instance at baseURL. A non-empty
// virtualKey is sent in the x-bf-vk header. A non-empty cacheKey is sent in
// x-bf-cache-key on chat requests, which puts their answers in that semantic
// cache partition; Bifrost does not cache requests without a key.
func New(baseURL, virtualKey, cacheKey string) *Client {
	// No client timeout: chat answers may stream for a long time; callers
	// bound requests with their context.
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), virtualKey: virtualKey, cacheKey: cacheKey, http: &http.Client{}}
}

// Chat posts body to /v1/chat/completions with the freshness mode and policy
// headers. The caller closes the response body.
func (c *Client) Chat(ctx context.Context, body []byte, mode, policy string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-bf-freshness", mode)
	if policy != "" {
		req.Header.Set("x-bf-freshness-policy", policy)
	}
	if c.cacheKey != "" {
		req.Header.Set("x-bf-cache-key", c.cacheKey)
	}
	if c.virtualKey != "" {
		req.Header.Set("x-bf-vk", c.virtualKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call Bifrost: %w", err)
	}
	return resp, nil
}

// Models returns the models Bifrost lists, sorted by ID.
func (c *Client) Models(ctx context.Context) ([]Model, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v1/models", nil)
	if err != nil {
		return nil, err
	}
	if c.virtualKey != "" {
		req.Header.Set("x-bf-vk", c.virtualKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list Bifrost models: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("Bifrost model list returned HTTP %d", resp.StatusCode)
	}
	var list struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, MaxModelsBytes)).Decode(&list); err != nil {
		return nil, fmt.Errorf("decode Bifrost model list: %w", err)
	}
	models := make([]Model, 0, len(list.Data))
	for _, m := range list.Data {
		provider, _, found := strings.Cut(m.ID, "/")
		if !found {
			provider = ""
		}
		models = append(models, Model{ID: m.ID, Provider: provider})
	}
	sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
	return models, nil
}
