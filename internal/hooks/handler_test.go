package hooks_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lucifer-135/freshness-guard/internal/config"
	"github.com/lucifer-135/freshness-guard/internal/engine"
	"github.com/lucifer-135/freshness-guard/internal/hooks"
	"github.com/lucifer-135/freshness-guard/internal/report"
)

const testAnswer = "Example Cloud Pro costs $20 per month."

var hookNow = time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)

type fakeSubmitter struct {
	jobs []engine.Job
	full bool
}

func (f *fakeSubmitter) Submit(job engine.Job) (<-chan report.Report, bool) {
	if f.full {
		return nil, false
	}
	f.jobs = append(f.jobs, job)
	return make(chan report.Report, 1), true
}

func newHandler(jobs *fakeSubmitter) *hooks.Handler {
	return &hooks.Handler{
		Config: config.Default(), Jobs: jobs, NewID: func() string { return "r-1" },
		Now: func() time.Time { return hookNow }, Sample: func() float64 { return 0 }, Logger: slog.Default(),
	}
}

func newCtx() *schemas.BifrostContext {
	return schemas.NewBifrostContext(context.Background(), time.Now().Add(time.Minute))
}

func str(s string) *string { return &s }

func chatRequest(kind schemas.RequestType) *schemas.BifrostRequest {
	return &schemas.BifrostRequest{RequestType: kind, ChatRequest: &schemas.BifrostChatRequest{
		Provider: "groq", Model: "openai/gpt-oss-20b",
		Input: []schemas.ChatMessage{
			{Role: schemas.ChatMessageRoleSystem, Content: &schemas.ChatMessageContent{ContentStr: str("be brief")}},
			{Role: schemas.ChatMessageRoleUser, Content: &schemas.ChatMessageContent{ContentStr: str("What does X cost?")}},
		},
	}}
}

func chatResponse(content *schemas.ChatMessageContent) *schemas.BifrostResponse {
	return &schemas.BifrostResponse{ChatResponse: &schemas.BifrostChatResponse{Choices: []schemas.BifrostResponseChoice{{
		ChatNonStreamResponseChoice: &schemas.ChatNonStreamResponseChoice{
			Message: &schemas.ChatMessage{Role: schemas.ChatMessageRole("assistant"), Content: content},
		},
	}}}}
}

// runChat drives all hooks for one non-streaming chat request and returns the
// response headers.
func runChat(h *hooks.Handler, headers map[string]string, content *schemas.ChatMessageContent) *schemas.HTTPResponseMetadata {
	ctx := newCtx()
	h.TransportPre(ctx, &schemas.HTTPRequest{Headers: headers})
	h.PreLLM(ctx, chatRequest(schemas.ChatCompletionRequest))
	h.PostLLM(ctx, chatResponse(content))
	md := &schemas.HTTPResponseMetadata{}
	h.ResponseHeaders(ctx, md)
	return md
}

func TestTransportPreStoresState(t *testing.T) {
	ctx := newCtx()
	newHandler(&fakeSubmitter{}).TransportPre(ctx, &schemas.HTTPRequest{Headers: map[string]string{"X-Bf-Freshness": "async"}})
	st := hooks.StateFrom(ctx)
	require.NotNil(t, st)
	assert.Equal(t, "r-1", st.ReportID)
	assert.Equal(t, hooks.ModeAsync, st.Mode)
}

func TestTransportPreOffHasNoReportID(t *testing.T) {
	ctx := newCtx()
	newHandler(&fakeSubmitter{}).TransportPre(ctx, &schemas.HTTPRequest{Headers: map[string]string{"x-bf-freshness": "off"}})
	st := hooks.StateFrom(ctx)
	require.NotNil(t, st)
	assert.Equal(t, hooks.ModeOff, st.Mode)
	assert.Empty(t, st.ReportID)
}

func TestTransportPreReadsVirtualKey(t *testing.T) {
	ctx := newCtx()
	newHandler(&fakeSubmitter{}).TransportPre(ctx, &schemas.HTTPRequest{Headers: map[string]string{"x-bf-vk": "vk-1"}})
	assert.Equal(t, "vk-1", hooks.StateFrom(ctx).VirtualKey)
}

func TestAsyncChatSubmitsJobAndSetsHeaders(t *testing.T) {
	jobs := &fakeSubmitter{}
	md := runChat(newHandler(jobs), map[string]string{"x-bf-freshness": "async", "x-bf-vk": "vk-1"},
		&schemas.ChatMessageContent{ContentStr: str(testAnswer)})
	require.Len(t, jobs.jobs, 1)
	assert.Equal(t, engine.Job{ReportID: "r-1", Answer: testAnswer, Prompt: "What does X cost?",
		Model: "groq/openai/gpt-oss-20b", Provider: "groq", VirtualKey: "vk-1", Mode: "async", At: hookNow}, jobs.jobs[0])
	assert.Equal(t, "r-1", md.Header("X-Freshness-Report-Id"))
	assert.Equal(t, "pending", md.Header("X-Freshness-Status"))
}

func TestSyncIsHandledLikeAsyncInM1(t *testing.T) {
	jobs := &fakeSubmitter{}
	md := runChat(newHandler(jobs), map[string]string{"x-bf-freshness": "sync"}, &schemas.ChatMessageContent{ContentStr: str(testAnswer)})
	require.Len(t, jobs.jobs, 1)
	assert.Equal(t, "sync", jobs.jobs[0].Mode)
	assert.Equal(t, "pending", md.Header("X-Freshness-Status"))
}

func TestQueueFullSetsSkipped(t *testing.T) {
	md := runChat(newHandler(&fakeSubmitter{full: true}), nil, &schemas.ChatMessageContent{ContentStr: str(testAnswer)})
	assert.Equal(t, "skipped", md.Header("X-Freshness-Status"))
	assert.Empty(t, md.Header("X-Freshness-Report-Id"))
}

func TestStreamingIsSkippedInM1(t *testing.T) {
	jobs := &fakeSubmitter{}
	h := newHandler(jobs)
	ctx := newCtx()
	h.TransportPre(ctx, &schemas.HTTPRequest{})
	h.PreLLM(ctx, chatRequest(schemas.ChatCompletionStreamRequest))
	h.PostLLM(ctx, chatResponse(&schemas.ChatMessageContent{ContentStr: str("part")}))
	md := &schemas.HTTPResponseMetadata{}
	h.ResponseHeaders(ctx, md)
	assert.Empty(t, jobs.jobs)
	assert.Equal(t, "skipped", md.Header("X-Freshness-Status"))
}

func TestOtherRequestTypesAreSkipped(t *testing.T) {
	jobs := &fakeSubmitter{}
	h := newHandler(jobs)
	ctx := newCtx()
	h.TransportPre(ctx, &schemas.HTTPRequest{})
	h.PreLLM(ctx, &schemas.BifrostRequest{RequestType: schemas.RequestType("embedding")})
	h.PostLLM(ctx, &schemas.BifrostResponse{})
	md := &schemas.HTTPResponseMetadata{}
	h.ResponseHeaders(ctx, md)
	assert.Empty(t, jobs.jobs)
	assert.Equal(t, "skipped", md.Header("X-Freshness-Status"))
}

func TestOffModeSetsNoHeaders(t *testing.T) {
	jobs := &fakeSubmitter{}
	md := runChat(newHandler(jobs), map[string]string{"x-bf-freshness": "OFF"}, &schemas.ChatMessageContent{ContentStr: str(testAnswer)})
	assert.Empty(t, jobs.jobs)
	assert.Empty(t, md.Header("X-Freshness-Status"))
	assert.Empty(t, md.Header("X-Freshness-Report-Id"))
}

func TestInternalRequestsAreOff(t *testing.T) {
	// The plugin's own LLM calls send exactly this header (llm.HeaderFreshness).
	jobs := &fakeSubmitter{}
	runChat(newHandler(jobs), map[string]string{"x-bf-freshness": "off"}, &schemas.ChatMessageContent{ContentStr: str(testAnswer)})
	assert.Empty(t, jobs.jobs)
}

func TestMarkFailedSetsErrorStatus(t *testing.T) {
	h := newHandler(&fakeSubmitter{})
	ctx := newCtx()
	h.TransportPre(ctx, &schemas.HTTPRequest{})
	hooks.MarkFailed(ctx)
	md := &schemas.HTTPResponseMetadata{}
	h.ResponseHeaders(ctx, md)
	assert.Equal(t, "error", md.Header("X-Freshness-Status"))
	assert.NotPanics(t, func() { hooks.MarkFailed(newCtx()) })
}

func TestContentBlocksAreJoined(t *testing.T) {
	jobs := &fakeSubmitter{}
	runChat(newHandler(jobs), nil, &schemas.ChatMessageContent{ContentBlocks: []schemas.ChatContentBlock{
		{Type: "text", Text: str("a")}, {Type: "text", Text: str("b")},
	}})
	require.Len(t, jobs.jobs, 1)
	assert.Equal(t, "a\nb", jobs.jobs[0].Answer)
}

// Bifrost runs the LLM hooks once per attempt. A failed primary attempt must
// not stop the fallback's answer from being checked.
func TestFallbackAnswerIsSubmitted(t *testing.T) {
	jobs := &fakeSubmitter{}
	h := newHandler(jobs)
	ctx := newCtx()
	h.TransportPre(ctx, &schemas.HTTPRequest{})
	h.PreLLM(ctx, chatRequest(schemas.ChatCompletionRequest))
	h.PostLLM(ctx, nil) // primary attempt failed

	fallback := chatRequest(schemas.ChatCompletionRequest)
	fallback.ChatRequest.Provider = "gemini"
	fallback.ChatRequest.Model = "gemini-3.5-flash"
	h.PreLLM(ctx, fallback)
	h.PostLLM(ctx, chatResponse(&schemas.ChatMessageContent{ContentStr: str(testAnswer)}))

	md := &schemas.HTTPResponseMetadata{}
	h.ResponseHeaders(ctx, md)
	require.Len(t, jobs.jobs, 1)
	assert.Equal(t, "gemini/gemini-3.5-flash", jobs.jobs[0].Model)
	assert.Equal(t, "pending", md.Header("X-Freshness-Status"))
}

func TestTransportPreReadsVirtualKeyFromOtherHeaders(t *testing.T) {
	cases := map[string]struct {
		headers map[string]string
		want    string
	}{
		"bearer virtual key":     {map[string]string{"Authorization": "Bearer sk-bf-abc"}, "sk-bf-abc"},
		"lowercase bearer":       {map[string]string{"authorization": "bearer  sk-bf-abc "}, "sk-bf-abc"},
		"bearer provider key":    {map[string]string{"Authorization": "Bearer sk-openai-123"}, ""},
		"x-api-key":              {map[string]string{"x-api-key": "sk-bf-x"}, "sk-bf-x"},
		"x-goog-api-key":         {map[string]string{"x-goog-api-key": "sk-bf-g"}, "sk-bf-g"},
		"api-key":                {map[string]string{"api-key": "sk-bf-a"}, "sk-bf-a"},
		"x-api-key provider key": {map[string]string{"x-api-key": "anthropic-key"}, ""},
		"x-bf-vk wins":           {map[string]string{"x-bf-vk": "vk-1", "Authorization": "Bearer sk-bf-abc"}, "vk-1"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx := newCtx()
			newHandler(&fakeSubmitter{}).TransportPre(ctx, &schemas.HTTPRequest{Headers: tc.headers})
			assert.Equal(t, tc.want, hooks.StateFrom(ctx).VirtualKey)
		})
	}
}

func TestVirtualKeyModeAppliesToBearerKeys(t *testing.T) {
	h := newHandler(&fakeSubmitter{})
	h.Config.VirtualKeys = map[string]config.VirtualKey{"sk-bf-off": {Mode: "off", SampleRate: 1}}
	ctx := newCtx()
	h.TransportPre(ctx, &schemas.HTTPRequest{Headers: map[string]string{"Authorization": "Bearer sk-bf-off"}})
	assert.Equal(t, hooks.ModeOff, hooks.StateFrom(ctx).Mode)
}

func TestEmptyAnswerIsSkipped(t *testing.T) {
	jobs := &fakeSubmitter{}
	md := runChat(newHandler(jobs), nil, &schemas.ChatMessageContent{ContentStr: str("")})
	assert.Empty(t, jobs.jobs)
	assert.Equal(t, "skipped", md.Header("X-Freshness-Status"))
}

func TestPreLLMWithoutTransportStateUsesContextHeaders(t *testing.T) {
	ctx := newCtx()
	ctx.SetValue(schemas.BifrostContextKeyRequestHeaders, map[string]string{"x-bf-freshness": "async"})
	newHandler(&fakeSubmitter{}).PreLLM(ctx, chatRequest(schemas.ChatCompletionRequest))
	st := hooks.StateFrom(ctx)
	require.NotNil(t, st)
	assert.Equal(t, hooks.ModeAsync, st.Mode)
	assert.Equal(t, "r-1", st.ReportID)
}

func TestHooksWithoutStateDoNothing(t *testing.T) {
	jobs := &fakeSubmitter{}
	h := newHandler(jobs)
	ctx := newCtx()
	h.PostLLM(ctx, chatResponse(&schemas.ChatMessageContent{ContentStr: str(testAnswer)}))
	md := &schemas.HTTPResponseMetadata{}
	h.ResponseHeaders(ctx, md)
	assert.Empty(t, jobs.jobs)
	assert.Empty(t, md.Header("X-Freshness-Status"))
}
