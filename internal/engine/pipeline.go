package engine

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/lucifer-135/freshness-guard/internal/events"
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
	Policy     string
	// CacheID and CacheHit come from Bifrost's semantic cache metadata.
	CacheID  string
	CacheHit bool
	At       time.Time
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

// Healer corrects the outdated claims of an answer; *heal.Healer implements it.
type Healer interface {
	Heal(ctx context.Context, answer string, claims []report.Claim) (string, int, error)
}

// Pipeline checks one answer: extract, triage, verify, score, heal and store.
type Pipeline struct {
	Extract       Extractor
	Verify        Verifier
	Facts         FactCache
	Store         ReportStore
	Budget        *Budget
	Heal          Healer
	Events        events.Publisher // nil means no events
	Cutoffs       map[string]time.Time
	RiskThreshold float64
	// BlockConfidence is the judge confidence from which a STALE or WRONG
	// claim blocks an answer under the block policy.
	BlockConfidence float64
	Now             func() time.Time
	Logger          *slog.Logger
}

// saveTimeout limits each report save. Saves do not use the job context, so a
// job that times out or is cancelled still records its final status.
const saveTimeout = 5 * time.Second

// internalErrorReason is the report reason after a panic.
const internalErrorReason = "An internal error stopped the verification."

// budgetReason explains a risky claim the credit budget left unchecked.
const budgetReason = "This claim was not checked because today's Tavily credit budget is used up or kept for riskier claims."

const day = 24 * time.Hour

// Run checks the job's answer and returns the finished report. It never fails:
// problems, including panics, end up in the report's status and reason, and
// storage errors are logged.
func (p *Pipeline) Run(ctx context.Context, job Job) (r report.Report) {
	r = report.Report{
		ID: job.ReportID, CreatedAt: job.At, Model: job.Model, Provider: job.Provider, VirtualKey: job.VirtualKey,
		Mode: job.Mode, Policy: job.Policy, Status: report.StatusPending, Prompt: job.Prompt, AnswerText: job.Answer,
		CacheID: job.CacheID, CacheHit: job.CacheHit, Claims: []report.Claim{},
	}
	defer func() {
		if v := recover(); v != nil {
			p.Logger.Error("A verification job panicked.", "report", job.ReportID, "panic", v, "stack", string(debug.Stack()))
			r = p.fail(ctx, r, internalErrorReason)
		}
	}()
	p.save(ctx, r)
	p.publish(events.TypeReportStarted, r.ID, map[string]any{
		"model": job.Model, "provider": job.Provider, "mode": job.Mode, "policy": job.Policy,
	})
	return p.run(ctx, job, r)
}

func (p *Pipeline) run(ctx context.Context, job Job, r report.Report) report.Report {
	started := p.Now()
	claims, err := p.Extract.Extract(ctx, extract.Input{Answer: job.Answer, Prompt: job.Prompt, Model: job.Model, At: job.At})
	if err != nil {
		return p.fail(ctx, r, fmt.Sprintf("Claim extraction failed: %v", err))
	}
	if claims == nil {
		claims = []report.Claim{}
	}
	extractDone := p.Now()

	age := triage.ModelAge(job.Model, p.Cutoffs, p.Now())
	r.ModelAgeDays = int(age / day)
	for i := range claims {
		c := &claims[i]
		c.PChanged = triage.PChanged(c.Category, age, c.TemporalMarker != "")
		if hl, ok := triage.HalfLife(c.Category); ok {
			c.HalfLifeDays = int(hl / day)
		}
		if fact, ok := p.Facts.Fresh(ctx, c.Entity, c.Attribute, c.Category); ok {
			c.Checked = true
			c.CacheHit = true
			c.Evidence = []report.Evidence{facts.CachedEvidence(fact)}
			id := fact.ID
			c.FactID = &id
		}
	}
	claims = triage.Select(claims, p.RiskThreshold, p.Budget)
	for i := range claims {
		if c := &claims[i]; !c.Checked && c.PChanged >= p.RiskThreshold {
			c.Verdict = report.VerdictUnverifiable
			c.Rationale = budgetReason
		}
	}
	// A copy: subscribers marshal the data later, while the pipeline goes on
	// scoring these claims.
	p.publish(events.TypeReportClaims, r.ID, map[string]any{"claims": append([]report.Claim(nil), claims...)})

	if anyChecked(claims) {
		verified, credits, err := p.Verify.Verify(ctx, claims)
		r.CreditsUsed = credits
		r.Claims = verified
		if err != nil {
			return p.fail(ctx, r, fmt.Sprintf("Verification failed: %v", err))
		}
		claims = verified
	}
	verifyDone := p.Now()

	now := p.Now()
	for i := range claims {
		c := &claims[i]
		c.Score = score.Claim(*c, now)
		c.ScoreParts = score.Explain(*c, now)
		// A cache hit keeps the stored fact as it is; recording it would
		// refresh the fact from its own cached value forever.
		if !c.Checked || c.CacheHit {
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
	for _, c := range claims {
		if c.Checked {
			p.publish(events.TypeClaimVerdict, r.ID, map[string]any{"claim": c})
		}
	}

	// Correction runs only when the client waits for it (sync mode, heal policy).
	if job.Mode == "sync" && job.Policy == "heal" && p.Heal != nil {
		text, healed, err := p.Heal.Heal(ctx, job.Answer, claims)
		if err != nil {
			p.Logger.Error("The answer could not be corrected.", "report", r.ID, "error", err)
		} else {
			r.HealedText, r.HealedCount = text, healed
		}
	}
	healDone := p.Now()

	r.Claims = claims
	r.Score = score.Answer(claims)
	r.TotalClaims = len(claims)
	for _, c := range claims {
		if c.Checked {
			r.CheckedCount++
		}
		if c.Verdict == report.VerdictStale || c.Verdict == report.VerdictWrong {
			r.StaleCount++
			// Only a sync request can be blocked; async answers are already delivered.
			if job.Mode == "sync" && job.Policy == "block" && c.Confidence >= p.BlockConfidence {
				r.Blocked = true
			}
		}
	}
	end := p.Now()
	r.LatencyMS = end.Sub(job.At).Milliseconds()
	r.Status = report.StatusComplete
	p.save(ctx, r)
	p.publish(events.TypeReportComplete, r.ID, map[string]any{
		"status": r.Status, "score": r.Score, "stale_count": r.StaleCount, "checked_count": r.CheckedCount,
		"total_claims": r.TotalClaims, "healed_count": r.HealedCount, "blocked": r.Blocked,
		"latency_ms": r.LatencyMS, "credits_used": r.CreditsUsed,
	})
	p.Logger.Info("A freshness report finished.", "report", r.ID, "status", r.Status, "claims", r.TotalClaims,
		"checked", r.CheckedCount, "credits", r.CreditsUsed,
		"extract_ms", extractDone.Sub(started).Milliseconds(), "verify_ms", verifyDone.Sub(extractDone).Milliseconds(),
		"heal_ms", healDone.Sub(verifyDone).Milliseconds())
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
	p.publish(events.TypeReportError, r.ID, map[string]any{"reason": reason})
	return r
}

func (p *Pipeline) save(ctx context.Context, r report.Report) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), saveTimeout)
	defer cancel()
	if err := p.Store.SaveReport(ctx, r); err != nil {
		p.Logger.Error("A freshness report could not be saved.", "report", r.ID, "status", r.Status, "error", err)
	}
}

func (p *Pipeline) publish(typ, reportID string, data any) {
	if p.Events == nil {
		return
	}
	p.Events.Publish(events.Event{Type: typ, ReportID: reportID, At: p.Now(), Data: data})
}
