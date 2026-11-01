package engine

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/lucifer-135/freshness-guard/internal/extract"
	"github.com/lucifer-135/freshness-guard/internal/facts"
	"github.com/lucifer-135/freshness-guard/internal/report"
	"github.com/lucifer-135/freshness-guard/internal/score"
	"github.com/lucifer-135/freshness-guard/internal/triage"
)

// Job is one answer to check. Model is "provider/model".
type Job struct {
	ReportID   string
	Answer     string
	Prompt     string
	Model      string
	Provider   string
	VirtualKey string
	Mode       string
	At         time.Time
}

// Extractor finds the claims in an answer.
type Extractor interface {
	Extract(ctx context.Context, in extract.Input) ([]report.Claim, error)
}

// Verifier checks the claims marked as checked.
type Verifier interface {
	Verify(ctx context.Context, claims []report.Claim) ([]report.Claim, int, error)
}

// FactCache is the verdict cache.
type FactCache interface {
	Fresh(ctx context.Context, entity, attribute string, cat report.Category) (report.Fact, bool)
	Record(ctx context.Context, cl report.Claim) (int64, bool, error)
}

// ReportStore saves reports.
type ReportStore interface {
	SaveReport(ctx context.Context, r report.Report) error
}

// Pipeline checks one answer: extract, triage, verify, score and store.
type Pipeline struct {
	Extract       Extractor
	Verify        Verifier
	Facts         FactCache
	Store         ReportStore
	Budget        *Budget
	Cutoffs       map[string]time.Time
	RiskThreshold float64
	Now           func() time.Time
	Logger        *slog.Logger
}

// Run checks the job's answer and returns the finished report. It never fails:
// problems end up in the report's status and reason, and storage errors are
// logged.
func (p *Pipeline) Run(ctx context.Context, job Job) report.Report {
	r := report.Report{
		ID: job.ReportID, CreatedAt: job.At, Model: job.Model, Provider: job.Provider, VirtualKey: job.VirtualKey,
		Mode: job.Mode, Status: report.StatusPending, Prompt: job.Prompt, AnswerText: job.Answer,
		Claims: []report.Claim{},
	}
	p.save(ctx, r)

	claims, err := p.Extract.Extract(ctx, extract.Input{Answer: job.Answer, Prompt: job.Prompt, Model: job.Model, At: job.At})
	if err != nil {
		return p.fail(ctx, r, fmt.Sprintf("Claim extraction failed: %v", err))
	}
	if claims == nil {
		claims = []report.Claim{}
	}

	age := triage.ModelAge(job.Model, p.Cutoffs, p.Now())
	for i := range claims {
		c := &claims[i]
		c.PChanged = triage.PChanged(c.Category, age, c.TemporalMarker != "")
		if fact, ok := p.Facts.Fresh(ctx, c.Entity, c.Attribute, c.Category); ok {
			c.Checked = true
			c.CacheHit = true
			c.Evidence = []report.Evidence{facts.CachedEvidence(fact)}
		}
	}
	claims = triage.Select(claims, p.RiskThreshold, p.Budget)

	if anyChecked(claims) {
		verified, credits, err := p.Verify.Verify(ctx, claims)
		r.CreditsUsed = credits
		r.Claims = verified
		if err != nil {
			return p.fail(ctx, r, fmt.Sprintf("Verification failed: %v", err))
		}
		claims = verified
	}

	now := p.Now()
	for i := range claims {
		c := &claims[i]
		c.Score = score.Claim(*c, now)
		if !c.Checked {
			continue
		}
		id, recorded, err := p.Facts.Record(ctx, *c)
		if err != nil {
			p.Logger.Error("A verified fact could not be stored.", "report", r.ID, "claim", c.ID, "error", err)
		}
		if recorded {
			c.FactID = &id
		}
	}

	r.Claims = claims
	r.Score = score.Answer(claims)
	r.TotalClaims = len(claims)
	for _, c := range claims {
		if c.Checked {
			r.CheckedCount++
		}
		if c.Verdict == report.VerdictStale || c.Verdict == report.VerdictWrong {
			r.StaleCount++
		}
	}
	r.LatencyMS = now.Sub(job.At).Milliseconds()
	r.Status = report.StatusComplete
	p.save(ctx, r)
	return r
}

func anyChecked(claims []report.Claim) bool {
	for _, c := range claims {
		if c.Checked {
			return true
		}
	}
	return false
}

func (p *Pipeline) fail(ctx context.Context, r report.Report, reason string) report.Report {
	r.Status = report.StatusError
	r.Reason = reason
	r.TotalClaims = len(r.Claims)
	r.LatencyMS = p.Now().Sub(r.CreatedAt).Milliseconds()
	p.save(ctx, r)
	return r
}

func (p *Pipeline) save(ctx context.Context, r report.Report) {
	if err := p.Store.SaveReport(ctx, r); err != nil {
		p.Logger.Error("A freshness report could not be saved.", "report", r.ID, "status", r.Status, "error", err)
	}
}
