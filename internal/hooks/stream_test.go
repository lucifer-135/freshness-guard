package hooks_test

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func chunk(text string) *schemas.BifrostResponse {
	return &schemas.BifrostResponse{ChatResponse: &schemas.BifrostChatResponse{Choices: []schemas.BifrostResponseChoice{{
		ChatStreamResponseChoice: &schemas.ChatStreamResponseChoice{Delta: &schemas.ChatStreamResponseChoiceDelta{Content: &text}},
	}}}}
}

func startStream(jobs *fakeSubmitter, headers map[string]string) (*schemas.BifrostContext, func(*schemas.BifrostResponse)) {
	h := newHandler(jobs)
	ctx := newCtx()
	h.TransportPre(ctx, &schemas.HTTPRequest{Headers: headers})
	h.PreLLM(ctx, chatRequest(schemas.ChatCompletionStreamRequest))
	return ctx, func(r *schemas.BifrostResponse) { h.PostLLM(ctx, r) }
}

func TestStreamingSubmitsJoinedText(t *testing.T) {
	jobs := &fakeSubmitter{}
	h := newHandler(jobs)
	ctx := newCtx()
	h.TransportPre(ctx, &schemas.HTTPRequest{Headers: map[string]string{"x-bf-freshness": "sync", "x-bf-freshness-policy": "heal"}})
	h.PreLLM(ctx, chatRequest(schemas.ChatCompletionStreamRequest))

	md := &schemas.HTTPResponseMetadata{}
	h.ResponseHeaders(ctx, md)
	assert.Equal(t, "r-1", md.Header("X-Freshness-Report-Id"))
	assert.Equal(t, "pending", md.Header("X-Freshness-Status"))

	h.PostLLM(ctx, chunk("Example "))
	h.PostLLM(ctx, chunk("Cloud"))
	assert.Empty(t, jobs.jobs, "nothing is submitted before the stream ends")
	ctx.SetValue(schemas.BifrostContextKeyStreamEndIndicator, true)
	h.PostLLM(ctx, chunk(""))

	require.Len(t, jobs.jobs, 1)
	job := jobs.jobs[0]
	assert.Equal(t, "Example Cloud", job.Answer)
	assert.Equal(t, "async", job.Mode)
	assert.Equal(t, "What does X cost?", job.Prompt)
	assert.Equal(t, "groq/openai/gpt-oss-20b", job.Model)
	assert.Equal(t, "r-1", job.ReportID)
}

func TestStreamingEmptyTextIsNotSubmitted(t *testing.T) {
	jobs := &fakeSubmitter{}
	ctx, post := startStream(jobs, nil)
	ctx.SetValue(schemas.BifrostContextKeyStreamEndIndicator, true)
	post(chunk(""))
	assert.Empty(t, jobs.jobs)
}

func TestStreamingStateIsRaceFree(t *testing.T) {
	jobs := &fakeSubmitter{}
	h := newHandler(jobs)
	ctx := newCtx()
	h.TransportPre(ctx, &schemas.HTTPRequest{})
	h.PreLLM(ctx, chatRequest(schemas.ChatCompletionStreamRequest))

	var want strings.Builder
	for i := range 100 {
		fmt.Fprintf(&want, "c%d ", i)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := range 100 {
			h.PostLLM(ctx, chunk(fmt.Sprintf("c%d ", i)))
		}
	}()
	go func() {
		defer wg.Done()
		for range 100 {
			h.ResponseHeaders(ctx, &schemas.HTTPResponseMetadata{})
		}
	}()
	wg.Wait()
	ctx.SetValue(schemas.BifrostContextKeyStreamEndIndicator, true)
	h.PostLLM(ctx, chunk(""))

	require.Len(t, jobs.jobs, 1)
	assert.Equal(t, want.String(), jobs.jobs[0].Answer)
}
