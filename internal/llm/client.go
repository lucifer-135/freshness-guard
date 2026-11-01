// Package llm calls Bifrost's own OpenAI-compatible chat endpoint for the
// plugin's internal model calls (claim extraction and judging).
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// HeaderFreshness is the request header that selects the plugin mode. Internal
// calls send "off" so that the plugin never checks its own requests.
const HeaderFreshness = "x-bf-freshness"

// ErrMalformed is returned when the model reply is still unusable after one
// repair attempt.
var ErrMalformed = errors.New("the model reply is not valid JSON for the schema")

// MaxResponseBytes limits how much of a reply is read.
const MaxResponseBytes = 4 << 20

// Request is one chat completion with a system and a user message.
type Request struct {
	Model      string
	Fallbacks  []string
	System     string
	User       string
	SchemaName string
	Schema     json.RawMessage
	MaxTokens  int
	Timeout    time.Duration
}

// Validator is implemented by reply types that check their own content.
type Validator interface {
	Validate() error
}

// Client sends chat completions to Bifrost.
type Client struct {
	baseURL    string
	virtualKey string
	http       *http.Client
}

// New returns a client for the Bifrost instance at baseURL. A non-empty
// virtualKey is sent in the x-bf-vk header.
func New(baseURL, virtualKey string) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), virtualKey: virtualKey, http: &http.Client{}}
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type jsonSchema struct {
	Name   string          `json:"name"`
	Schema json.RawMessage `json:"schema"`
}

type responseFormat struct {
	Type       string     `json:"type"`
	JSONSchema jsonSchema `json:"json_schema"`
}

type chatBody struct {
	Model          string          `json:"model"`
	Messages       []message       `json:"messages"`
	Temperature    float64         `json:"temperature"`
	MaxTokens      int             `json:"max_completion_tokens,omitempty"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
	Fallbacks      []string        `json:"fallbacks,omitempty"`
}

type chatReply struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// Complete sends req and returns the content of the first choice.
func (c *Client) Complete(ctx context.Context, req Request) (string, error) {
	return c.send(ctx, req, initialMessages(req))
}

// CompleteJSON sends req, decodes the reply into out and validates it. An
// unusable reply gets one repair request; a second failure returns an error
// wrapping ErrMalformed. HTTP errors are returned without a repair attempt.
func (c *Client) CompleteJSON(ctx context.Context, req Request, out any) error {
	messages := initialMessages(req)
	content, err := c.send(ctx, req, messages)
	if err != nil {
		return err
	}
	decodeErr := decode(content, out)
	if decodeErr == nil {
		return nil
	}

	messages = append(messages,
		message{Role: "assistant", Content: content},
		message{Role: "user", Content: fmt.Sprintf(
			"Your previous reply could not be used: %v. Reply again with only a JSON object that matches the schema.", decodeErr)},
	)
	content, err = c.send(ctx, req, messages)
	if err != nil {
		return err
	}
	if err := decode(content, out); err != nil {
		return fmt.Errorf("%w: %v", ErrMalformed, err)
	}
	return nil
}

func initialMessages(req Request) []message {
	return []message{{Role: "system", Content: req.System}, {Role: "user", Content: req.User}}
}

func decode(content string, out any) error {
	if err := json.Unmarshal([]byte(stripFence(content)), out); err != nil {
		return err
	}
	if v, ok := out.(Validator); ok {
		return v.Validate()
	}
	return nil
}

// stripFence removes a surrounding Markdown code fence such as ```json ... ```.
func stripFence(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	s = strings.TrimPrefix(s, "```")
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[i+1:]
	}
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "```"))
}

func (c *Client) send(ctx context.Context, req Request, messages []message) (string, error) {
	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}

	body := chatBody{Model: req.Model, Messages: messages, MaxTokens: req.MaxTokens, Fallbacks: req.Fallbacks}
	if len(req.Schema) > 0 {
		body.ResponseFormat = &responseFormat{Type: "json_schema", JSONSchema: jsonSchema{Name: req.SchemaName, Schema: req.Schema}}
	}
	data, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat/completions", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set(HeaderFreshness, "off")
	if c.virtualKey != "" {
		httpReq.Header.Set("x-bf-vk", c.virtualKey)
	}

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("call %s: %w", req.Model, err)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, MaxResponseBytes+1))
	if err != nil {
		return "", fmt.Errorf("read reply from %s: %w", req.Model, err)
	}
	if len(payload) > MaxResponseBytes {
		return "", fmt.Errorf("the reply from %s is too large (more than %d bytes)", req.Model, MaxResponseBytes)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		excerpt := payload
		if len(excerpt) > 300 {
			excerpt = excerpt[:300]
		}
		return "", fmt.Errorf("model %s returned HTTP %d: %s", req.Model, resp.StatusCode, excerpt)
	}

	var reply chatReply
	if err := json.Unmarshal(payload, &reply); err != nil {
		return "", fmt.Errorf("decode reply from %s: %w", req.Model, err)
	}
	if len(reply.Choices) == 0 {
		return "", fmt.Errorf("model %s returned no choices", req.Model)
	}
	return reply.Choices[0].Message.Content, nil
}
