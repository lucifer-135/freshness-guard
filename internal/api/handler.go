package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lucifer-135/freshness-guard/internal/engine"
	"github.com/lucifer-135/freshness-guard/internal/report"
	"github.com/lucifer-135/freshness-guard/internal/store"
)

// Limits for POST /freshness/check.
const (
	MaxCheckTextRunes   = 20000
	DefaultCheckTimeout = 30 * time.Second
	maxBodyBytes        = 1 << 20
)

// Reports reads stored reports; *store.SQLite implements it.
type Reports interface {
	GetReport(ctx context.Context, id string) (report.Report, error)
	ListReports(ctx context.Context, f store.ListFilter) ([]report.Report, error)
}

// Jobs submits verification jobs; *engine.Engine implements it.
type Jobs interface {
	Submit(job engine.Job) (<-chan report.Report, bool)
	IsQueued(id string) bool
}

// Deps holds what the API handlers need.
type Deps struct {
	Token        string
	Reports      Reports
	Jobs         Jobs
	NewID        func() string
	Now          func() time.Time
	CheckTimeout time.Duration
	Logger       *slog.Logger
	Events       Events
	Heartbeat    time.Duration
}

type handler struct {
	Deps
}

// NewHandler returns the plugin API's request handler. Every route except
// GET /freshness/health needs the bearer token.
func NewHandler(deps Deps) http.Handler {
	if deps.CheckTimeout <= 0 {
		deps.CheckTimeout = DefaultCheckTimeout
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.Heartbeat <= 0 {
		deps.Heartbeat = DefaultHeartbeat
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	h := &handler{deps}
	protect := requireToken(deps.Token)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /freshness/health", handleHealth)
	mux.Handle("GET /freshness/reports/{id}", protect(http.HandlerFunc(h.getReport)))
	mux.Handle("GET /freshness/reports", protect(http.HandlerFunc(h.listReports)))
	mux.Handle("POST /freshness/check", protect(http.HandlerFunc(h.check)))
	mux.Handle("GET /freshness/events", protect(http.HandlerFunc(h.events)))
	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

type pendingReport struct {
	ID     string      `json:"id"`
	Status string      `json:"status"`
	Claims *[]struct{} `json:"claims,omitempty"`
}

func (h *handler) getReport(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rep, err := h.Reports.GetReport(r.Context(), id)
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, rep)
	case errors.Is(err, store.ErrNotFound) && h.Jobs.IsQueued(id):
		writeJSON(w, http.StatusOK, pendingReport{ID: id, Status: string(report.StatusPending), Claims: &[]struct{}{}})
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "The report was not found.")
	default:
		h.Logger.Error("A report could not be loaded.", "report", id, "error", err)
		writeError(w, http.StatusInternalServerError, "The report could not be loaded.")
	}
}

func (h *handler) listReports(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := store.ListFilter{Model: q.Get("model")}
	if s := q.Get("since"); s != "" {
		since, err := parseSince(s)
		if err != nil {
			writeError(w, http.StatusBadRequest, "The since parameter must be an RFC 3339 time or a YYYY-MM-DD date.")
			return
		}
		filter.Since = since
	}
	limit, _ := strconv.Atoi(q.Get("limit"))
	filter.Limit = store.EffectiveLimit(limit)

	reports, err := h.Reports.ListReports(r.Context(), filter)
	if err != nil {
		h.Logger.Error("Reports could not be listed.", "error", err)
		writeError(w, http.StatusInternalServerError, "The reports could not be loaded.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reports": reports})
}

func parseSince(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}
	return time.Parse("2006-01-02", s)
}

type checkRequest struct {
	Text   string `json:"text"`
	Prompt string `json:"prompt"`
	Model  string `json:"model"`
}

func (h *handler) check(w http.ResponseWriter, r *http.Request) {
	var req checkRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "The request body must be a JSON object with a text field.")
		return
	}
	if strings.TrimSpace(req.Text) == "" {
		writeError(w, http.StatusBadRequest, "The text field must not be empty.")
		return
	}
	if utf8.RuneCountInString(req.Text) > MaxCheckTextRunes {
		writeError(w, http.StatusBadRequest, "The text field must not be longer than 20000 characters.")
		return
	}

	job := engine.Job{ReportID: h.NewID(), Answer: req.Text, Prompt: req.Prompt, Model: req.Model, Mode: "check", At: h.Now()}
	result, ok := h.Jobs.Submit(job)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "The verification queue is full. Try again later.")
		return
	}

	timer := time.NewTimer(h.CheckTimeout)
	defer timer.Stop()
	select {
	case rep, ok := <-result:
		if ok {
			writeJSON(w, http.StatusOK, rep)
			return
		}
	case <-timer.C:
	case <-r.Context().Done():
		return
	}
	writeJSON(w, http.StatusAccepted, pendingReport{ID: job.ReportID, Status: string(report.StatusPending)})
}
