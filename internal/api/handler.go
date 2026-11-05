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
	"github.com/lucifer-135/freshness-guard/internal/mcp"
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
	Gateway      Gateway
	Credits      Credits
	// UI serves the web app under /freshness/ui/; nil means no web app.
	UI       http.Handler
	Settings Settings
	// Stats feeds the dashboard routes. CachePurgeEnabled tells the dashboard
	// whether the semantic-cache purge is switched on.
	Stats             Stats
	CachePurgeEnabled bool
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
	mux.Handle("POST /freshness/chat", protect(http.HandlerFunc(h.chat)))
	mux.Handle("GET /freshness/models", protect(http.HandlerFunc(h.models)))
	mux.Handle("GET /freshness/status", protect(http.HandlerFunc(h.status)))
	if deps.Stats != nil {
		mux.Handle("GET /freshness/leaderboard", protect(http.HandlerFunc(h.leaderboard)))
		mux.Handle("GET /freshness/summary", protect(http.HandlerFunc(h.summary)))
	}
	// Every method: Streamable HTTP uses POST, GET and DELETE on one path.
	mux.Handle(mcp.Path, requireMCPToken(deps.Token)(mcp.NewHandler(func(ctx context.Context, text string) (report.Report, bool, error) {
		return h.checkText(ctx, text, "", "")
	})))
	if deps.UI != nil {
		// The web app's files hold no secrets; it asks for the token itself.
		mux.Handle("GET /freshness/ui/", deps.UI)
		mux.Handle("GET /{$}", http.RedirectHandler("/freshness/ui/", http.StatusFound))
	}
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

// checkError is a refused check: the HTTP status and the sentence shown to
// the caller, through the API or the MCP tool.
type checkError struct {
	status  int
	message string
}

func (e *checkError) Error() string { return e.message }

func (h *handler) check(w http.ResponseWriter, r *http.Request) {
	var req checkRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "The request body must be a JSON object with a text field.")
		return
	}
	rep, done, err := h.checkText(r.Context(), req.Text, req.Prompt, req.Model)
	var refused *checkError
	switch {
	case errors.As(err, &refused):
		writeError(w, refused.status, refused.message)
	case err != nil:
		// The client went away; there is nobody to answer.
	case done:
		writeJSON(w, http.StatusOK, rep)
	default:
		writeJSON(w, http.StatusAccepted, pendingReport{ID: rep.ID, Status: string(report.StatusPending)})
	}
}

// checkText validates text, submits it and waits up to CheckTimeout. done is
// false when the check is still running; the report then carries only its ID.
func (h *handler) checkText(ctx context.Context, text, prompt, model string) (report.Report, bool, error) {
	if strings.TrimSpace(text) == "" {
		return report.Report{}, false, &checkError{http.StatusBadRequest, "The text field must not be empty."}
	}
	if utf8.RuneCountInString(text) > MaxCheckTextRunes {
		return report.Report{}, false, &checkError{http.StatusBadRequest, "The text field must not be longer than 20000 characters."}
	}

	job := engine.Job{ReportID: h.NewID(), Answer: text, Prompt: prompt, Model: model, Mode: "check", At: h.Now()}
	result, ok := h.Jobs.Submit(job)
	if !ok {
		return report.Report{}, false, &checkError{http.StatusServiceUnavailable, "The verification queue is full. Try again later."}
	}

	pending := report.Report{ID: job.ReportID, Status: report.StatusPending}
	timer := time.NewTimer(h.CheckTimeout)
	defer timer.Stop()
	select {
	case rep, ok := <-result:
		if ok {
			return rep, true, nil
		}
	case <-timer.C:
	case <-ctx.Done():
		return pending, false, ctx.Err()
	}
	return pending, false, nil
}
