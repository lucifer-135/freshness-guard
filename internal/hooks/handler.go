package hooks

import (
	"log/slog"
	"strings"
	"time"

	"github.com/maximhq/bifrost/core/schemas"

	"github.com/lucifer-135/freshness-guard/internal/config"
	"github.com/lucifer-135/freshness-guard/internal/engine"
	"github.com/lucifer-135/freshness-guard/internal/report"
)

// StateKey is the Bifrost context key under which the per-request state lives.
const StateKey schemas.BifrostContextKey = "freshness-guard-state"

// State is what the hooks of one request share. The pointer is stored in the
// context once, and later hooks change the same value.
type State struct {
	ReportID   string
	Mode       Mode
	Status     string
	Prompt     string
	Model      string
	Provider   string
	VirtualKey string
	Failed     bool
}

// Submitter queues verification jobs; *engine.Engine implements it.
type Submitter interface {
	Submit(job engine.Job) (<-chan report.Report, bool)
}

// Handler holds the logic behind the plugin's Bifrost hooks.
type Handler struct {
	Config config.Config
	Jobs   Submitter
	NewID  func() string
	Now    func() time.Time
	Sample func() float64
	Logger *slog.Logger
}

// StateFrom returns the request's state, or nil when there is none.
func StateFrom(ctx *schemas.BifrostContext) *State {
	if ctx == nil {
		return nil
	}
	st, _ := ctx.Value(StateKey).(*State)
	return st
}

// MarkFailed records that a hook failed, so the response gets
// X-Freshness-Status: error.
func MarkFailed(ctx *schemas.BifrostContext) {
	if st := StateFrom(ctx); st != nil {
		st.Failed = true
	}
}

// virtualKeyPrefix marks Bifrost virtual keys sent in provider-style headers.
const virtualKeyPrefix = "sk-bf-"

// virtualKey finds the request's virtual key the way Bifrost does: x-bf-vk
// first, then a bearer token or an API-key header that starts with "sk-bf-".
func virtualKey(lookup func(string) string) string {
	if vk := strings.TrimSpace(lookup(HeaderVirtualKey)); vk != "" {
		return vk
	}
	isVirtualKey := func(v string) bool { return strings.HasPrefix(strings.ToLower(v), virtualKeyPrefix) }
	if auth := strings.TrimSpace(lookup("authorization")); len(auth) > 7 && strings.EqualFold(auth[:7], "bearer ") {
		if v := strings.TrimSpace(auth[7:]); isVirtualKey(v) {
			return v
		}
	}
	for _, header := range []string{"x-api-key", "x-goog-api-key", "api-key"} {
		if v := strings.TrimSpace(lookup(header)); isVirtualKey(v) {
			return v
		}
	}
	return ""
}

func (h *Handler) newState(ctx *schemas.BifrostContext, lookup func(string) string) *State {
	vk := virtualKey(lookup)
	st := &State{
		Mode:       SelectMode(lookup(HeaderFreshness), vk, h.Config, h.Sample()),
		VirtualKey: vk,
	}
	if st.Mode != ModeOff {
		st.ReportID = h.NewID()
	}
	ctx.SetValue(StateKey, st)
	return st
}

// TransportPre selects the mode from the request headers and stores the state.
func (h *Handler) TransportPre(ctx *schemas.BifrostContext, req *schemas.HTTPRequest) {
	if req == nil {
		req = &schemas.HTTPRequest{}
	}
	h.newState(ctx, req.CaseInsensitiveHeaderLookup)
}

// PreLLM records the prompt and model of a chat request. Streaming and other
// request types are skipped in M1.
func (h *Handler) PreLLM(ctx *schemas.BifrostContext, req *schemas.BifrostRequest) {
	st := StateFrom(ctx)
	if st == nil {
		headers, _ := ctx.Value(schemas.BifrostContextKeyRequestHeaders).(map[string]string)
		st = h.newState(ctx, func(key string) string { return headers[strings.ToLower(key)] })
	}
	if st.Mode == ModeOff {
		return
	}
	if req == nil || req.RequestType != schemas.ChatCompletionRequest || req.ChatRequest == nil {
		st.Status = StatusSkipped
		return
	}
	chat := req.ChatRequest
	st.Provider = string(chat.Provider)
	st.Model = st.Provider + "/" + chat.Model
	for i := len(chat.Input) - 1; i >= 0; i-- {
		if chat.Input[i].Role == schemas.ChatMessageRoleUser {
			st.Prompt = messageText(chat.Input[i].Content)
			break
		}
	}
}

// PostLLM submits the answer for verification. Bifrost runs the LLM hooks
// once per attempt, so an attempt without an answer (a failed primary before
// a fallback) leaves the status open; the headers hook reports "skipped" when
// no attempt produced an answer.
func (h *Handler) PostLLM(ctx *schemas.BifrostContext, resp *schemas.BifrostResponse) {
	st := StateFrom(ctx)
	if st == nil || st.Mode == ModeOff || st.Status != "" {
		return
	}
	answer := answerText(resp)
	if answer == "" {
		return
	}
	_, ok := h.Jobs.Submit(engine.Job{
		ReportID: st.ReportID, Answer: answer, Prompt: st.Prompt, Model: st.Model, Provider: st.Provider,
		VirtualKey: st.VirtualKey, Mode: string(st.Mode), At: h.Now(),
	})
	if !ok {
		st.Status = StatusSkipped
		h.Logger.Warn("The verification queue is full, so this answer was not checked.", "report", st.ReportID)
		return
	}
	st.Status = StatusPending
}

// ResponseHeaders sets the X-Freshness-* headers.
func (h *Handler) ResponseHeaders(ctx *schemas.BifrostContext, resp *schemas.HTTPResponseMetadata) {
	st := StateFrom(ctx)
	if st == nil || st.Mode == ModeOff {
		return
	}
	switch {
	case st.Failed:
		SetStatusHeader(resp, StatusError)
	case st.Status == StatusPending:
		SetHeader(resp, HeaderReportID, st.ReportID)
		SetStatusHeader(resp, StatusPending)
	default:
		SetStatusHeader(resp, StatusSkipped)
	}
}

func answerText(resp *schemas.BifrostResponse) string {
	if resp == nil || resp.ChatResponse == nil || len(resp.ChatResponse.Choices) == 0 {
		return ""
	}
	choice := resp.ChatResponse.Choices[0]
	if choice.ChatNonStreamResponseChoice == nil || choice.Message == nil {
		return ""
	}
	return messageText(choice.Message.Content)
}

func messageText(c *schemas.ChatMessageContent) string {
	if c == nil {
		return ""
	}
	if c.ContentStr != nil {
		return *c.ContentStr
	}
	var parts []string
	for _, b := range c.ContentBlocks {
		if b.Text != nil {
			parts = append(parts, *b.Text)
		}
	}
	return strings.Join(parts, "\n")
}
