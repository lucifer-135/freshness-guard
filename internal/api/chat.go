package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/lucifer-135/freshness-guard/internal/gateway"
)

// Gateway reaches Bifrost for the web app; *gateway.Client implements it.
type Gateway interface {
	Chat(ctx context.Context, body []byte, mode, policy string) (*http.Response, error)
	Models(ctx context.Context) ([]gateway.Model, error)
}

// Credits reports today's Tavily credit use; *engine.Budget implements it.
type Credits interface {
	Used() int
	Daily() int
}

// Settings are the configured defaults that GET /freshness/status reports, so
// the web app can start from them and name the real sync time limit.
type Settings struct {
	DefaultMode   string `json:"default_mode"`
	DefaultPolicy string `json:"default_policy"`
	SyncTimeoutMS int    `json:"sync_timeout_ms"`
}

const statusModelsTimeout = 2 * time.Second

type chatRequest struct {
	Model    string          `json:"model"`
	Messages json.RawMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Mode     string          `json:"mode"`
	Policy   string          `json:"policy"`
}

// chat proxies a chat completion through Bifrost, so the web app needs only
// the plugin API's origin and token. The answer goes through the gateway and
// is checked like any other request.
func (h *handler) chat(w http.ResponseWriter, r *http.Request) {
	if h.Gateway == nil {
		writeError(w, http.StatusServiceUnavailable, "The chat proxy is not available.")
		return
	}
	var req chatRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "The request body must be a JSON object with model and messages.")
		return
	}
	var messages []json.RawMessage
	if strings.TrimSpace(req.Model) == "" || json.Unmarshal(req.Messages, &messages) != nil || len(messages) == 0 {
		writeError(w, http.StatusBadRequest, "The request needs a model and at least one message.")
		return
	}
	if req.Mode == "" {
		req.Mode = "async"
	}
	if req.Mode != "async" && req.Mode != "sync" && req.Mode != "off" {
		writeError(w, http.StatusBadRequest, "The mode must be async, sync or off.")
		return
	}
	if req.Policy != "" && req.Policy != "annotate" && req.Policy != "heal" && req.Policy != "block" {
		writeError(w, http.StatusBadRequest, "The policy must be annotate, heal or block.")
		return
	}

	body, err := json.Marshal(map[string]any{"model": req.Model, "messages": req.Messages, "stream": req.Stream})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "The request could not be prepared.")
		return
	}
	resp, err := h.Gateway.Chat(r.Context(), body, req.Mode, req.Policy)
	if err != nil {
		h.Logger.Error("The chat proxy could not reach Bifrost.", "error", err)
		writeError(w, http.StatusBadGateway, "Bifrost could not be reached.")
		return
	}
	defer resp.Body.Close()

	for name, values := range resp.Header {
		if name == "Content-Type" || name == "Cache-Control" || strings.HasPrefix(name, "X-Freshness-") {
			w.Header()[name] = values
		}
	}
	w.WriteHeader(resp.StatusCode)
	flusher, _ := w.(http.Flusher)
	buf := make([]byte, 4096)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := w.Write(buf[:n]); err != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if readErr != nil {
			if readErr != io.EOF {
				h.Logger.Warn("The chat stream from Bifrost ended early.", "error", readErr)
			}
			return
		}
	}
}

func (h *handler) models(w http.ResponseWriter, r *http.Request) {
	if h.Gateway == nil {
		writeError(w, http.StatusServiceUnavailable, "The model list is not available.")
		return
	}
	models, err := h.Gateway.Models(r.Context())
	if err != nil {
		h.Logger.Error("The model list could not be loaded from Bifrost.", "error", err)
		writeError(w, http.StatusBadGateway, "Bifrost could not be reached.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"models": models})
}

func (h *handler) status(w http.ResponseWriter, r *http.Request) {
	gw := map[string]any{"reachable": false, "models": 0}
	if h.Gateway != nil {
		ctx, cancel := context.WithTimeout(r.Context(), statusModelsTimeout)
		defer cancel()
		if models, err := h.Gateway.Models(ctx); err == nil {
			gw["reachable"], gw["models"] = true, len(models)
		}
	}
	credits := map[string]int{"used": 0, "budget": 0}
	if h.Credits != nil {
		credits["used"], credits["budget"] = h.Credits.Used(), h.Credits.Daily()
	}
	writeJSON(w, http.StatusOK, map[string]any{"gateway": gw, "credits": credits, "settings": h.Settings})
}
