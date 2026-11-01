// Command fakeprovider is an OpenAI-compatible test server that always returns
// the same answer, so that end-to-end tests are deterministic.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// Answer is the fixed reply. It contains two time-sensitive claims (a price
// and a version) that later milestones use as test input.
const Answer = "Example Cloud Pro costs $20 per month, and its current version is 4.2."

const modelID = "fake-model"

type chatRequest struct {
	Model  string `json:"model"`
	Stream bool   `json:"stream"`
}

func newHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/chat/completions", handleChat)
	mux.HandleFunc("GET /v1/models", handleModels)
	return mux
}

func handleChat(w http.ResponseWriter, r *http.Request) {
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if req.Model == "" {
		req.Model = modelID
	}
	if req.Stream {
		streamChat(w, req.Model)
		return
	}
	writeJSON(w, map[string]any{
		"id":      "chatcmpl-fake",
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   req.Model,
		"choices": []map[string]any{{
			"index":         0,
			"message":       map[string]any{"role": "assistant", "content": Answer},
			"finish_reason": "stop",
		}},
		"usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 20, "total_tokens": 30},
	})
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
	writeJSON(w, map[string]any{
		"object": "list",
		"data":   []map[string]any{{"id": modelID, "object": "model", "owned_by": "fakeprovider"}},
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func main() {
	log.Println("fakeprovider listening on :9000")
	log.Fatal(http.ListenAndServe(":9000", newHandler()))
}
