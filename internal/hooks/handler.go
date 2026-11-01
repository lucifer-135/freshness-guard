package hooks

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/maximhq/bifrost/core/schemas"

	"github.com/lucifer-135/freshness-guard/internal/config"
	"github.com/lucifer-135/freshness-guard/internal/engine"
	"github.com/lucifer-135/freshness-guard/internal/report"
)

// StateKey is the Bifrost context key under which the per-request state lives.
const StateKey schemas.BifrostContextKey = "freshness-guard-state"

// State is what the hooks of one request share. The pointer is stored in the
// context once, and later hooks change the same value under mu, because for
// streamed answers the headers hook runs while chunks are still processed.
type State struct {
	mu sync.Mutex

	ReportID   string
	Mode       Mode
	Policy     Policy
	Status     string
	Prompt     string
	Model      string
	Provider   string
	VirtualKey string
	Failed     bool
	// Report is the finished report when sync mode received it in time.
	Report *report.Report

	// streaming requests collect their chunks in buf until the last one.
	streaming bool
	buf       strings.Builder
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

// Messages of a blocked answer; %s is the report ID.
const (
	blockedOutdated   = "The answer was blocked because it contains outdated or incorrect claims. Report ID: %s."
	blockedUnverified = "The answer was blocked because it could not be verified in time. Report ID: %s."
)

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
		st.mu.Lock()
		defer st.mu.Unlock()
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
		Policy:     SelectPolicy(lookup(HeaderPolicy), vk, h.Config),
		VirtualKey: vk,
	}
	if st.Mode != ModeOff {
		st.ReportID = h.NewID()
	}
	ctx.SetValue(StateKey, st)
	return st
}

// TransportPre selects the mode and policy from the request headers and
// stores the state.
func (h *Handler) TransportPre(ctx *schemas.BifrostContext, req *schemas.HTTPRequest) {
	if req == nil {
		req = &schemas.HTTPRequest{}
	}
	h.newState(ctx, req.CaseInsensitiveHeaderLookup)
}

// PreLLM records the prompt and model of a chat request, streamed or not.
// Other request types are skipped.
func (h *Handler) PreLLM(ctx *schemas.BifrostContext, req *schemas.BifrostRequest) {
	st := StateFrom(ctx)
	if st == nil {
		headers, _ := ctx.Value(schemas.BifrostContextKeyRequestHeaders).(map[string]string)
		st = h.newState(ctx, func(key string) string { return headers[strings.ToLower(key)] })
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.Mode == ModeOff {
		return
	}
	if req == nil || req.ChatRequest == nil ||
		(req.RequestType != schemas.ChatCompletionRequest && req.RequestType != schemas.ChatCompletionStreamRequest) {
		st.Status = StatusSkipped
		return
	}
	st.streaming = req.RequestType == schemas.ChatCompletionStreamRequest
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

// PostLLM submits the answer for verification. In sync mode it waits for the
// report, at most Config.SyncTimeout, and applies the policy: heal replaces
// the answer with the corrected text, block returns an error instead of the
// answer. Bifrost runs the LLM hooks once per attempt, so an attempt without
// an answer (a failed primary before a fallback) leaves the status open.
func (h *Handler) PostLLM(ctx *schemas.BifrostContext, resp *schemas.BifrostResponse) (*schemas.BifrostResponse, *schemas.BifrostError) {
	st := StateFrom(ctx)
	if st == nil {
		return resp, nil
	}
	plan := h.plan(ctx, st, resp)
	if !plan.submit {
		return resp, nil
	}

	result, ok := h.Jobs.Submit(plan.job)
	if !ok {
		h.setStatus(st, StatusSkipped)
		h.Logger.Warn("The verification queue is full, so this answer was not checked.", "report", plan.job.ReportID)
		if plan.streaming {
			return resp, nil
		}
		return h.unverified(resp, plan.mode, plan.policy, plan.job.ReportID)
	}
	if plan.streaming {
		return resp, nil
	}
	if plan.mode != ModeSync {
		h.setStatus(st, StatusPending)
		return resp, nil
	}

	// Wait without holding the state lock.
	timer := time.NewTimer(h.Config.SyncTimeout)
	defer timer.Stop()
	var rep *report.Report
	select {
	case r, ok := <-result:
		if ok {
			rep = &r
		}
	case <-timer.C:
	}
	h.record(st, rep)

	id := plan.job.ReportID
	if rep == nil || rep.Status != report.StatusComplete {
		return h.unverified(resp, plan.mode, plan.policy, id)
	}
	switch {
	case plan.policy == PolicyBlock && rep.Blocked:
		return nil, blockError(id, blockedOutdated)
	case plan.policy == PolicyHeal && rep.HealedText != "":
		setAnswer(resp, rep.HealedText)
	}
	return resp, nil
}

// postPlan is what PostLLM decided while holding the state lock.
type postPlan struct {
	submit    bool
	streaming bool
	job       engine.Job
	mode      Mode
	policy    Policy
}

// plan reads and updates the state under its lock. Every locked section
// unlocks with defer, so a panic cannot leave the state locked for the hooks
// that still run afterwards. For streams it adds the chunk's text to the
// buffer and plans a submission only on the last chunk; streams are always
// checked asynchronously, because their headers are sent before the answer
// is complete (spec 8.5).
func (h *Handler) plan(ctx *schemas.BifrostContext, st *State, resp *schemas.BifrostResponse) postPlan {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.Mode == ModeOff || st.Status != "" {
		return postPlan{}
	}
	if st.streaming {
		st.buf.WriteString(deltaText(resp))
		if end, _ := ctx.Value(schemas.BifrostContextKeyStreamEndIndicator).(bool); !end {
			return postPlan{}
		}
		answer := st.buf.String()
		st.buf.Reset()
		if answer == "" {
			st.Status = StatusSkipped
			return postPlan{}
		}
		st.Status = StatusPending
		return postPlan{submit: true, streaming: true, job: engine.Job{
			ReportID: st.ReportID, Answer: answer, Prompt: st.Prompt, Model: st.Model, Provider: st.Provider,
			VirtualKey: st.VirtualKey, Mode: string(ModeAsync), At: h.Now(),
		}}
	}
	answer := answerText(resp)
	if answer == "" {
		return postPlan{}
	}
	return postPlan{submit: true, mode: st.Mode, policy: st.Policy, job: engine.Job{
		ReportID: st.ReportID, Answer: answer, Prompt: st.Prompt, Model: st.Model, Provider: st.Provider,
		VirtualKey: st.VirtualKey, Mode: string(st.Mode), Policy: string(st.Policy), At: h.Now(),
	}}
}

// record stores the outcome of a sync wait: nil means it timed out.
func (h *Handler) record(st *State, rep *report.Report) {
	st.mu.Lock()
	defer st.mu.Unlock()
	switch {
	case rep == nil:
		st.Status = StatusPending
	case rep.Status == report.StatusComplete:
		st.Report, st.Status = rep, StatusComplete
	default:
		st.Report, st.Status = rep, StatusError
	}
}

func deltaText(resp *schemas.BifrostResponse) string {
	if resp == nil || resp.ChatResponse == nil || len(resp.ChatResponse.Choices) == 0 {
		return ""
	}
	choice := resp.ChatResponse.Choices[0]
	if choice.ChatStreamResponseChoice == nil || choice.Delta == nil || choice.Delta.Content == nil {
		return ""
	}
	return *choice.Delta.Content
}

// unverified handles a sync request without a complete report: the answer
// passes, unless the block policy is configured to fail closed.
func (h *Handler) unverified(resp *schemas.BifrostResponse, mode Mode, policy Policy, id string) (*schemas.BifrostResponse, *schemas.BifrostError) {
	if mode == ModeSync && policy == PolicyBlock && h.Config.FailClosed {
		return nil, blockError(id, blockedUnverified)
	}
	return resp, nil
}

func (h *Handler) setStatus(st *State, status string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.Status = status
}

// blockError is final: Bifrost must not retry a blocked answer on a fallback.
func blockError(id, message string) *schemas.BifrostError {
	status, kind, allowFallbacks := 422, "freshness_blocked", false
	return &schemas.BifrostError{
		StatusCode:     &status,
		Type:           &kind,
		Error:          &schemas.ErrorField{Type: &kind, Message: fmt.Sprintf(message, id)},
		AllowFallbacks: &allowFallbacks,
	}
}

func setAnswer(resp *schemas.BifrostResponse, text string) {
	if resp == nil || resp.ChatResponse == nil || len(resp.ChatResponse.Choices) == 0 {
		return
	}
	choice := resp.ChatResponse.Choices[0]
	if choice.ChatNonStreamResponseChoice == nil || choice.Message == nil {
		return
	}
	choice.Message.Content = &schemas.ChatMessageContent{ContentStr: &text}
}

// ResponseHeaders sets the X-Freshness-* headers.
func (h *Handler) ResponseHeaders(ctx *schemas.BifrostContext, resp *schemas.HTTPResponseMetadata) {
	st := StateFrom(ctx)
	if st == nil {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.Mode == ModeOff {
		return
	}
	switch {
	case st.Failed:
		SetStatusHeader(resp, StatusError)
	case st.Status == StatusPending, st.streaming && st.Status == "":
		// Stream headers are sent before the answer ends; the report is announced.
		SetHeader(resp, HeaderReportID, st.ReportID)
		SetStatusHeader(resp, StatusPending)
	case st.Status == StatusComplete && st.Report != nil:
		r := st.Report
		SetHeader(resp, HeaderReportID, st.ReportID)
		SetStatusHeader(resp, StatusComplete)
		if r.Score != nil {
			SetHeader(resp, HeaderScore, strconv.Itoa(*r.Score))
		}
		SetHeader(resp, HeaderStale, strconv.Itoa(r.StaleCount))
		SetHeader(resp, HeaderChecked, fmt.Sprintf("%d/%d", r.CheckedCount, r.TotalClaims))
		if st.Policy == PolicyHeal {
			SetHeader(resp, HeaderHealed, strconv.Itoa(r.HealedCount))
		}
	case st.Status == StatusError:
		SetHeader(resp, HeaderReportID, st.ReportID)
		SetStatusHeader(resp, StatusError)
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
