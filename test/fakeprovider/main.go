// Command fakeprovider is an OpenAI-compatible test server that always returns
// the same answer, so that end-to-end tests are deterministic. It also plays
// the plugin's extraction and judge models and Tavily's search endpoint.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// Answer is the fixed reply. It contains two time-sensitive claims (a price
// and a version) that the end-to-end tests check.
const Answer = "Example Cloud Pro costs $20 per month, and its current version is 4.2."

// Model names and the Tavily key the end-to-end configuration uses.
const (
	modelID      = "fake-model"
	ExtractModel = "fake-extract"
	JudgeModel   = "fake-judge"
	HealModel    = "fake-heal"
	TavilyKey    = "fake-tavily-key"
	// RateLimitedModel always answers HTTP 429, so tests can exercise fallbacks.
	RateLimitedModel = "fake-ratelimited"
)

// extraction is the fake-extract reply: the two claims in Answer.
const extraction = `{"claims":[
 {"quote":"Example Cloud Pro costs $20 per month","entity":"Example Cloud Pro","attribute":"monthly price","value":"$20 per month",
  "category":"pricing","temporal_marker":"","search_query":"Example Cloud Pro monthly price"},
 {"quote":"its current version is 4.2","entity":"Example Cloud Pro","attribute":"current version","value":"4.2",
  "category":"version","temporal_marker":"current","search_query":"Example Cloud Pro latest version"}]}`

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string            `json:"model"`
	Stream   bool              `json:"stream"`
	Messages []message         `json:"messages"`
	Tools    []json.RawMessage `json:"tools"`
}

// toolRequests counts chat requests that arrived with tools. The e2e suite
// reads it to prove that Bifrost does not inject MCP tools into requests.
var toolRequests atomic.Int64

func newHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/chat/completions", handleChat)
	mux.HandleFunc("GET /v1/models", handleModels)
	mux.HandleFunc("POST /v1/embeddings", handleEmbeddings)
	mux.HandleFunc("POST /search", handleSearch)
	mux.HandleFunc("GET /debug/tool-requests", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"count":%d}`, toolRequests.Load())
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	return mux
}

func handleChat(w http.ResponseWriter, r *http.Request) {
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if len(req.Tools) > 0 {
		toolRequests.Add(1)
	}
	if req.Model == "" {
		req.Model = modelID
	}
	if req.Model == RateLimitedModel {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"Rate limit reached.","type":"rate_limit_exceeded","code":"rate_limit_exceeded"}}`))
		return
	}
	if req.Stream {
		streamChat(w, req.Model)
		return
	}

	content := Answer
	switch req.Model {
	case ExtractModel:
		content = extraction
	case JudgeModel:
		content = judge(lastUser(req))
	case HealModel:
		content = healAnswer(lastUser(req))
	}
	writeJSON(w, map[string]any{
		"id":      "chatcmpl-fake",
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   req.Model,
		"choices": []map[string]any{{
			"index":         0,
			"message":       map[string]any{"role": "assistant", "content": content},
			"finish_reason": "stop",
		}},
		"usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 20, "total_tokens": 30},
	})
}

func lastUser(req chatRequest) string {
	if len(req.Messages) == 0 {
		return ""
	}
	return req.Messages[len(req.Messages)-1].Content
}

// healAnswer replaces each correction's old value with its current value.
func healAnswer(user string) string {
	var in struct {
		Answer      string `json:"answer"`
		Corrections []struct {
			Value        string `json:"value"`
			CurrentValue string `json:"current_value"`
		} `json:"corrections"`
	}
	_ = json.Unmarshal([]byte(user), &in)
	text := in.Answer
	for _, c := range in.Corrections {
		text = strings.Replace(text, c.Value, c.CurrentValue, 1)
	}
	out, _ := json.Marshal(map[string]string{"text": text})
	return string(out)
}

// judge answers the plugin's judge input: price claims are STALE (now $25 per
// month), every other claim is FRESH. All of a claim's evidence supports the
// verdict.
func judge(user string) string {
	var in struct {
		Claims []struct {
			ID        string `json:"id"`
			Attribute string `json:"attribute"`
			Evidence  []struct {
				Ref string `json:"ref"`
			} `json:"evidence"`
		} `json:"claims"`
	}
	_ = json.Unmarshal([]byte(user), &in)

	results := []map[string]any{}
	for _, c := range in.Claims {
		refs := []string{}
		for _, e := range c.Evidence {
			refs = append(refs, e.Ref)
		}
		first := ""
		if len(refs) > 0 {
			first = refs[0]
		}
		result := map[string]any{
			"claim_id": c.ID, "verdict": "FRESH", "current_value": "4.2", "as_of": "2026-10-15",
			"supporting_evidence_ids": refs, "contradicting_evidence_ids": []string{}, "timeline": []any{},
			"confidence": 0.85, "rationale": "The release notes list 4.2 as the current version.",
		}
		if strings.Contains(c.Attribute, "price") {
			result["verdict"], result["current_value"], result["as_of"] = "STALE", "$25 per month", "2026-09-01"
			result["confidence"] = 0.9
			result["rationale"] = "The official pricing page lists $25 per month since September 2026."
			result["timeline"] = []map[string]string{
				{"value": "$20 per month", "valid_from": "2025-01-01", "source": first},
				{"value": "$25 per month", "valid_from": "2026-09-01", "source": first},
			}
		}
		results = append(results, result)
	}
	out, _ := json.Marshal(map[string]any{"results": results})
	return string(out)
}

type searchResult struct {
	Title         string  `json:"title"`
	URL           string  `json:"url"`
	Content       string  `json:"content"`
	Score         float64 `json:"score"`
	PublishedDate string  `json:"published_date"`
}

var pricingResults = []searchResult{
	{"Example Cloud pricing", "https://example.com/pricing", "Example Cloud Pro now costs $25 per month.", 0.9, "2026-09-01"},
	{"Example Cloud raises prices", "https://www.reuters.com/technology/example-cloud-prices",
		"Example Cloud raised the Pro price to $25 per month.", 0.9, "2026-09-02"},
}

var versionResults = []searchResult{
	{"Example Cloud releases", "https://example.com/releases", "Example Cloud Pro 4.2 is the current release.", 0.9, "2026-10-15"},
	{"Example Cloud 4.2", "https://blog.someone.dev/example-cloud-4-2", "Version 4.2 of Example Cloud Pro is out.", 0.9, "2026-10-16"},
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer "+TavilyKey {
		http.Error(w, "invalid API key", http.StatusUnauthorized)
		return
	}
	var req struct {
		Query string `json:"query"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	results := versionResults
	if strings.Contains(strings.ToLower(req.Query), "price") {
		results = pricingResults
	}
	writeJSON(w, map[string]any{"query": req.Query, "results": results, "response_time": 0.01})
}

func streamChat(w http.ResponseWriter, model string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	flusher, _ := w.(http.Flusher)

	third := len(Answer) / 3
	parts := []string{Answer[:third], Answer[third : 2*third], Answer[2*third:]}
	for i, part := range parts {
		var finish any
		if i == len(parts)-1 {
			finish = "stop"
		}
		chunk, _ := json.Marshal(map[string]any{
			"id":      "chatcmpl-fake",
			"object":  "chat.completion.chunk",
			"created": time.Now().Unix(),
			"model":   model,
			"choices": []map[string]any{{
				"index":         0,
				"delta":         map[string]any{"content": part},
				"finish_reason": finish,
			}},
		})
		fmt.Fprintf(w, "data: %s\n\n", chunk)
		if flusher != nil {
			flusher.Flush()
		}
	}
	fmt.Fprint(w, "data: [DONE]\n\n")
	if flusher != nil {
		flusher.Flush()
	}
}

func handleModels(w http.ResponseWriter, _ *http.Request) {
	data := []map[string]any{}
	for _, id := range []string{modelID, ExtractModel, JudgeModel, HealModel} {
		data = append(data, map[string]any{"id": id, "object": "model", "owned_by": "fakeprovider"})
	}
	writeJSON(w, map[string]any{"object": "list", "data": data})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func main() {
	log.Println("fakeprovider listening on :9000")
	log.Fatal(http.ListenAndServe(":9000", newHandler()))
}

// embeddingDims is the size of the fake embeddings, small enough to read in a
// test and large enough that different texts differ.
const embeddingDims = 8

// handleEmbeddings answers the OpenAI embeddings request that Bifrost's
// semantic cache sends, so the cache can be tested without a real model.
func handleEmbeddings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Model string          `json:"model"`
		Input json.RawMessage `json:"input"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	var single string
	var inputs []string
	if json.Unmarshal(req.Input, &single) == nil {
		inputs = []string{single}
	} else if err := json.Unmarshal(req.Input, &inputs); err != nil {
		http.Error(w, "input must be a string or an array of strings", http.StatusBadRequest)
		return
	}
	data := make([]map[string]any, len(inputs))
	for i, text := range inputs {
		data[i] = map[string]any{"object": "embedding", "index": i, "embedding": fakeEmbedding(text)}
	}
	writeJSON(w, map[string]any{
		"object": "list", "data": data, "model": req.Model,
		"usage": map[string]any{"prompt_tokens": len(inputs), "total_tokens": len(inputs)},
	})
}

// fakeEmbedding counts the letters of text into embeddingDims buckets and
// scales the counts to length 1. Text without letters maps to the first axis.
func fakeEmbedding(text string) []float64 {
	v := make([]float64, embeddingDims)
	for _, r := range strings.ToLower(text) {
		if r >= 'a' && r <= 'z' {
			v[int(r-'a')%embeddingDims]++
		}
	}
	var sum float64
	for _, x := range v {
		sum += x * x
	}
	if sum == 0 {
		v[0] = 1
		return v
	}
	norm := math.Sqrt(sum)
	for i := range v {
		v[i] /= norm
	}
	return v
}
