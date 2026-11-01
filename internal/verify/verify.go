// Package verify checks claims against Tavily search results with one LLM
// judge call per answer, and escalates once when the evidence is not enough.
package verify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/lucifer-135/freshness-guard/internal/llm"
	"github.com/lucifer-135/freshness-guard/internal/report"
	"github.com/lucifer-135/freshness-guard/internal/tavily"
)

// Limits for verification.
const (
	JudgeTimeout      = 6 * time.Second
	JudgeMaxTokens    = 4096
	MaxSnippetRunes   = 400
	SearchConcurrency = 3
	MaxResults        = 5
)

// Searcher runs Tavily searches; *tavily.Client implements it.
type Searcher interface {
	Search(ctx context.Context, req tavily.SearchRequest) (tavily.SearchResponse, error)
}

// JSONCompleter is the LLM call the judge needs; *llm.Client implements it.
type JSONCompleter interface {
	CompleteJSON(ctx context.Context, req llm.Request, out any) error
}

// Spender reserves Tavily credits for escalation searches and refunds credits
// reserved for searches that failed.
type Spender interface {
	TrySpend(n int) bool
	Refund(n int)
}

// Verifier checks claims with Tavily evidence and an LLM judge.
type Verifier struct {
	Search    Searcher
	LLM       JSONCompleter
	Model     string
	Fallbacks []string
	Budget    Spender
	Now       func() time.Time
}

const conflictPrefix = "A newer authoritative source disagrees with this claim. "

// Verify checks every claim with Checked set and returns all claims in input
// order, together with the Tavily credits its searches used. A judge failure
// is returned as an error.
func (v *Verifier) Verify(ctx context.Context, claims []report.Claim) ([]report.Claim, int, error) {
	out := make([]report.Claim, len(claims))
	for i, c := range claims {
		c.Evidence = append([]report.Evidence(nil), c.Evidence...)
		out[i] = c
	}

	searched, credits := v.searchAll(ctx, out)
	var toJudge []int
	for i := range out {
		if out[i].Checked && (out[i].CacheHit || searched[i]) {
			toJudge = append(toJudge, i)
		}
	}
	if len(toJudge) == 0 {
		return out, credits, nil
	}

	next := v.assignRefs(out, toJudge, 1)
	results, err := v.judge(ctx, out, toJudge)
	if err != nil {
		return out, credits, err
	}
	apply(out, toJudge, results)

	escalated, extra := v.escalate(ctx, out, toJudge, searched, &next)
	credits += extra
	if len(escalated) > 0 {
		if results, err := v.judge(ctx, out, escalated); err == nil {
			apply(out, escalated, results)
		}
	}

	for _, i := range toJudge {
		out[i] = ApplyConflictRule(out[i])
	}
	return out, credits, nil
}

type searchOutcome struct {
	resp tavily.SearchResponse
	err  error
}

// searchAll runs one basic search per checked claim without cached evidence.
// It returns which claims got results and the credits used. Claims whose
// search failed or found nothing are marked UNVERIFIABLE.
func (v *Verifier) searchAll(ctx context.Context, claims []report.Claim) (map[int]bool, int) {
	outcomes := make(map[int]*searchOutcome)
	for i, c := range claims {
		if c.Checked && !c.CacheHit {
			outcomes[i] = &searchOutcome{}
		}
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, SearchConcurrency)
	for i, o := range outcomes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			c := claims[i]
			o.resp, o.err = v.Search.Search(ctx, tavily.SearchRequest{
				Query: searchQuery(c), Topic: TopicFor(c.Category), Depth: "basic", MaxResults: MaxResults,
			})
		}()
	}
	wg.Wait()

	searched := make(map[int]bool)
	credits := 0
	for i, o := range outcomes {
		c := &claims[i]
		if o.err != nil {
			// Triage reserved one credit for this search; a failed search costs none.
			v.refund(1)
		}
		switch {
		case errors.Is(o.err, tavily.ErrCreditLimit):
			markUnverifiable(c, "The Tavily credit limit has been reached.")
		case errors.Is(o.err, tavily.ErrNoAPIKey):
			markUnverifiable(c, "No Tavily API key is configured.")
		case o.err != nil:
			markUnverifiable(c, "The web search failed.")
		case len(o.resp.Results) == 0:
			credits += o.resp.Credits
			markUnverifiable(c, "The web search returned no results.")
		default:
			credits += o.resp.Credits
			c.Evidence = append(c.Evidence, toEvidence(o.resp.Results, c.Entity, nil)...)
			searched[i] = true
		}
	}
	return searched, credits
}

// escalate runs one advanced search for each searched claim the judge found
// UNVERIFIABLE, when the budget allows it. It returns the claims that gained
// new evidence and the credits used.
func (v *Verifier) escalate(ctx context.Context, claims []report.Claim, judged []int, searched map[int]bool, next *int) ([]int, int) {
	if v.Budget == nil {
		return nil, 0
	}
	var escalated []int
	credits := 0
	for _, i := range judged {
		c := &claims[i]
		if c.Verdict != report.VerdictUnverifiable || !searched[i] || !v.Budget.TrySpend(2) {
			continue
		}
		resp, err := v.Search.Search(ctx, tavily.SearchRequest{
			Query: searchQuery(*c), Topic: TopicFor(c.Category), Depth: "advanced", MaxResults: MaxResults,
		})
		if err != nil {
			v.refund(2)
			continue
		}
		credits += resp.Credits
		seen := make(map[string]bool, len(c.Evidence))
		for _, e := range c.Evidence {
			seen[e.URL] = true
		}
		added := toEvidence(resp.Results, c.Entity, seen)
		if len(added) == 0 {
			continue
		}
		for k := range added {
			added[k].Ref = fmt.Sprintf("E%d", *next)
			*next++
		}
		c.Evidence = append(c.Evidence, added...)
		escalated = append(escalated, i)
	}
	return escalated, credits
}

func (v *Verifier) refund(n int) {
	if v.Budget != nil {
		v.Budget.Refund(n)
	}
}

func searchQuery(c report.Claim) string {
	if c.SearchQuery != "" {
		return c.SearchQuery
	}
	return c.Entity + " " + c.Attribute
}

func markUnverifiable(c *report.Claim, rationale string) {
	c.Verdict = report.VerdictUnverifiable
	c.Rationale = rationale
}

// toEvidence converts search results, skipping URLs already in seen.
func toEvidence(results []tavily.Result, entity string, seen map[string]bool) []report.Evidence {
	var out []report.Evidence
	for _, r := range results {
		if seen[r.URL] {
			continue
		}
		if seen != nil {
			seen[r.URL] = true
		}
		out = append(out, report.Evidence{
			URL: r.URL, Title: r.Title, Domain: hostOf(r.URL), PublishedDate: r.PublishedDate,
			Snippet: truncateRunes(r.Content, MaxSnippetRunes), Authority: Authority(r.URL, entity),
			Stance: report.StanceNeutral,
		})
	}
	return out
}

// assignRefs numbers the evidence of the given claims E<first>, E<first+1>, …
// in claim order, fills in missing authority, and returns the next number.
func (v *Verifier) assignRefs(claims []report.Claim, idx []int, first int) int {
	n := first
	for _, i := range idx {
		for k := range claims[i].Evidence {
			e := &claims[i].Evidence[k]
			e.Ref = fmt.Sprintf("E%d", n)
			n++
			if e.Authority == 0 {
				e.Authority = Authority(e.URL, claims[i].Entity)
			}
		}
	}
	return n
}

type judgeEvidence struct {
	Ref           string `json:"ref"`
	URL           string `json:"url"`
	Title         string `json:"title"`
	PublishedDate string `json:"published_date"`
	Snippet       string `json:"snippet"`
}

type judgeClaim struct {
	ID        string          `json:"id"`
	Quote     string          `json:"quote"`
	Entity    string          `json:"entity"`
	Attribute string          `json:"attribute"`
	Value     string          `json:"value"`
	Category  string          `json:"category"`
	Evidence  []judgeEvidence `json:"evidence"`
}

type judgeInput struct {
	Today  string       `json:"today"`
	Claims []judgeClaim `json:"claims"`
}

type judgeResult struct {
	ClaimID       string   `json:"claim_id"`
	Verdict       string   `json:"verdict"`
	CurrentValue  string   `json:"current_value"`
	AsOf          string   `json:"as_of"`
	Supporting    []string `json:"supporting_evidence_ids"`
	Contradicting []string `json:"contradicting_evidence_ids"`
	Timeline      []struct {
		Value     string `json:"value"`
		ValidFrom string `json:"valid_from"`
		Source    string `json:"source"`
	} `json:"timeline"`
	Confidence float64 `json:"confidence"`
	Rationale  string  `json:"rationale"`
}

type judgeReply struct {
	Results []judgeResult `json:"results"`
}

var verdicts = map[string]bool{"FRESH": true, "STALE": true, "WRONG": true, "UNVERIFIABLE": true}

// Validate rejects replies the pipeline cannot use.
func (r *judgeReply) Validate() error {
	if r.Results == nil {
		return errors.New("results is missing")
	}
	for _, res := range r.Results {
		if res.ClaimID == "" {
			return errors.New("a result has an empty claim_id")
		}
		if !verdicts[res.Verdict] {
			return fmt.Errorf("claim %s has the verdict %q, which is not FRESH, STALE, WRONG or UNVERIFIABLE", res.ClaimID, res.Verdict)
		}
	}
	return nil
}

func (v *Verifier) judge(ctx context.Context, claims []report.Claim, idx []int) ([]judgeResult, error) {
	in := judgeInput{Today: v.Now().UTC().Format("2006-01-02")}
	for _, i := range idx {
		c := claims[i]
		jc := judgeClaim{ID: c.ID, Quote: c.Quote, Entity: c.Entity, Attribute: c.Attribute, Value: c.Value,
			Category: string(c.Category), Evidence: []judgeEvidence{}}
		for _, e := range c.Evidence {
			published := ""
			if e.PublishedDate != nil {
				published = e.PublishedDate.UTC().Format("2006-01-02")
			}
			jc.Evidence = append(jc.Evidence, judgeEvidence{Ref: e.Ref, URL: e.URL, Title: e.Title,
				PublishedDate: published, Snippet: e.Snippet})
		}
		in.Claims = append(in.Claims, jc)
	}
	user, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	var reply judgeReply
	err = v.LLM.CompleteJSON(ctx, llm.Request{
		Model: v.Model, Fallbacks: v.Fallbacks, System: systemPrompt, User: string(user),
		SchemaName: "verdicts", Schema: json.RawMessage(schema), MaxTokens: JudgeMaxTokens, Timeout: JudgeTimeout,
	}, &reply)
	if err != nil {
		return nil, fmt.Errorf("judge: %w", err)
	}
	return reply.Results, nil
}

func apply(claims []report.Claim, idx []int, results []judgeResult) {
	byID := make(map[string]judgeResult, len(results))
	for _, r := range results {
		byID[r.ClaimID] = r
	}
	for _, i := range idx {
		c := &claims[i]
		for k := range c.Evidence {
			c.Evidence[k].Stance = report.StanceNeutral
		}
		r, ok := byID[c.ID]
		if !ok {
			markUnverifiable(c, "The judge returned no result for this claim.")
			continue
		}
		c.Verdict = report.Verdict(r.Verdict)
		c.CurrentValue = r.CurrentValue
		c.AsOf = r.AsOf
		c.Confidence = r.Confidence
		c.Rationale = r.Rationale

		urls := make(map[string]string, len(c.Evidence))
		stance := make(map[string]report.Stance)
		for _, ref := range r.Contradicting {
			stance[ref] = report.StanceContradicts
		}
		for _, ref := range r.Supporting {
			stance[ref] = report.StanceSupports
		}
		for k := range c.Evidence {
			e := &c.Evidence[k]
			urls[e.Ref] = e.URL
			if s, ok := stance[e.Ref]; ok {
				e.Stance = s
			}
		}
		c.Timeline = make([]report.TimelineEntry, 0, len(r.Timeline))
		for _, t := range r.Timeline {
			c.Timeline = append(c.Timeline, report.TimelineEntry{Value: t.Value, ValidFrom: t.ValidFrom, SourceURL: urls[t.Source]})
		}
	}
}

// ApplyConflictRule enforces "the newest dated authoritative source wins": a
// FRESH claim becomes UNVERIFIABLE when an authoritative source that
// contradicts it is newer than every dated supporting source.
func ApplyConflictRule(c report.Claim) report.Claim {
	if c.Verdict != report.VerdictFresh {
		return c
	}
	var newestContra, newestSupport *time.Time
	for _, e := range c.Evidence {
		if e.PublishedDate == nil {
			continue
		}
		switch {
		case e.Stance == report.StanceContradicts && e.Authority >= 1:
			if newestContra == nil || e.PublishedDate.After(*newestContra) {
				newestContra = e.PublishedDate
			}
		case e.Stance == report.StanceSupports:
			if newestSupport == nil || e.PublishedDate.After(*newestSupport) {
				newestSupport = e.PublishedDate
			}
		}
	}
	if newestContra != nil && (newestSupport == nil || newestContra.After(*newestSupport)) {
		c.Verdict = report.VerdictUnverifiable
		c.Rationale = conflictPrefix + c.Rationale
	}
	return c
}

func truncateRunes(s string, n int) string {
	count := 0
	for i := range s {
		if count == n {
			return s[:i]
		}
		count++
	}
	return s
}
