package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/lucifer-135/freshness-guard/internal/events"
)

// Events gives access to the verification event stream; *events.Broker
// implements it.
type Events interface {
	Subscribe() (<-chan events.Event, func())
}

// DefaultHeartbeat is how often an idle event stream sends a comment, so that
// proxies and clients keep the connection open.
const DefaultHeartbeat = 15 * time.Second

// events streams verification events as server-sent events. The optional
// query parameter "report" limits the stream to one report.
func (h *handler) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok || h.Events == nil {
		writeError(w, http.StatusServiceUnavailable, "The event stream is not available.")
		return
	}
	only := r.URL.Query().Get("report")
	ch, cancel := h.Events.Subscribe()
	defer cancel()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	heartbeat := time.NewTicker(h.Heartbeat)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case e, open := <-ch:
			if !open {
				return
			}
			if only != "" && e.ReportID != only {
				continue
			}
			data, err := json.Marshal(e)
			if err != nil {
				h.Logger.Error("An event could not be encoded.", "type", e.Type, "error", err)
				continue
			}
			fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", e.ID, e.Type, data)
			flusher.Flush()
		case <-heartbeat.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}
