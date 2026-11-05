// Package report defines the freshness report, its claims and evidence. The JSON
// field names are the contract of the plugin API.
package report

import "time"

// Category is the kind of fact a claim states.
type Category string

// Claim categories.
const (
	CategoryPricing    Category = "pricing"
	CategoryVersion    Category = "version"
	CategoryLeadership Category = "leadership"
	CategoryLaw        Category = "law"
	CategoryStatistic  Category = "statistic"
	CategoryEvent      Category = "event"
	CategoryFeature    Category = "feature"
	CategoryStatic     Category = "static"
)

// Verdict is the result of checking a claim.
type Verdict string

// Claim verdicts.
const (
	VerdictFresh        Verdict = "FRESH"
	VerdictStale        Verdict = "STALE"
	VerdictWrong        Verdict = "WRONG"
	VerdictUnverifiable Verdict = "UNVERIFIABLE"
	VerdictNotChecked   Verdict = "NOT_CHECKED"
)

// Status is the processing state of a report.
type Status string

// Report statuses.
const (
	StatusPending  Status = "pending"
	StatusComplete Status = "complete"
	StatusError    Status = "error"
)

// Stance says how an evidence item relates to a claim.
type Stance string

// Evidence stances.
const (
	StanceSupports    Stance = "supports"
	StanceContradicts Stance = "contradicts"
	StanceNeutral     Stance = "neutral"
)

// Report is the freshness check of one answer.
type Report struct {
	ID           string    `json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	Model        string    `json:"model"`
	Provider     string    `json:"provider"`
	VirtualKey   string    `json:"-"`
	Mode         string    `json:"mode"`
	Status       Status    `json:"status"`
	Reason       string    `json:"reason,omitempty"`
	Prompt       string    `json:"prompt"`
	AnswerText   string    `json:"answer_text"`
	Score        *int      `json:"score"`
	StaleCount   int       `json:"stale_count"`
	CheckedCount int       `json:"checked_count"`
	TotalClaims  int       `json:"total_claims"`
	LatencyMS    int64     `json:"latency_ms"`
	CreditsUsed  int       `json:"credits_used"`
	Policy       string    `json:"policy"`
	HealedText   string    `json:"healed_text"`
	HealedCount  int       `json:"healed_count"`
	Blocked      bool      `json:"blocked"`
	ModelAgeDays int       `json:"model_age_days"`
	// CacheID is the semantic cache entry that holds this answer; empty when
	// the answer was not cached. CacheHit says Bifrost served it from the cache.
	CacheID  string  `json:"cache_id"`
	CacheHit bool    `json:"cache_hit"`
	Claims   []Claim `json:"claims"`
}

// ScoreParts are the factors of a FRESH claim's score: evidence recency,
// source authority and agreement between sources.
type ScoreParts struct {
	Recency   float64 `json:"recency"`
	Authority float64 `json:"authority"`
	Agreement float64 `json:"agreement"`
}

// Claim is one factual statement found in an answer. Spans are UTF-16
// code-unit offsets into the answer, or -1 when the quote was not found.
type Claim struct {
	ID             string          `json:"id"`
	Quote          string          `json:"quote"`
	SpanStart      int             `json:"span_start"`
	SpanEnd        int             `json:"span_end"`
	Entity         string          `json:"entity"`
	Attribute      string          `json:"attribute"`
	Value          string          `json:"value"`
	Category       Category        `json:"category"`
	TemporalMarker string          `json:"temporal_marker"`
	SearchQuery    string          `json:"search_query"`
	PChanged       float64         `json:"p_changed"`
	Checked        bool            `json:"checked"`
	CacheHit       bool            `json:"cache_hit"`
	Verdict        Verdict         `json:"verdict"`
	CurrentValue   string          `json:"current_value"`
	AsOf           string          `json:"as_of"`
	Score          *int            `json:"score"`
	ScoreParts     *ScoreParts     `json:"score_parts"`
	HalfLifeDays   int             `json:"half_life_days"` // 0 when the fact never changes
	Confidence     float64         `json:"confidence"`
	Rationale      string          `json:"rationale"`
	FactID         *int64          `json:"fact_id"`
	Evidence       []Evidence      `json:"evidence"`
	Timeline       []TimelineEntry `json:"timeline"`
}

// Evidence is one source used to check a claim.
type Evidence struct {
	Ref           string     `json:"ref"`
	URL           string     `json:"url"`
	Title         string     `json:"title"`
	Domain        string     `json:"domain"`
	PublishedDate *time.Time `json:"published_date"`
	Snippet       string     `json:"snippet"`
	Authority     float64    `json:"authority"`
	Stance        Stance     `json:"stance"`
}

// TimelineEntry is one known value of a fact and the date it took effect.
type TimelineEntry struct {
	Value     string `json:"value"`
	ValidFrom string `json:"valid_from"`
	SourceURL string `json:"source_url"`
}

// Fact is the latest verified value of an (entity, attribute) pair. It also
// serves as the verdict cache.
type Fact struct {
	ID             int64
	Key            string
	Entity         string
	Attribute      string
	Category       Category
	CurrentValue   string
	AsOf           string
	SourceURL      string
	LastVerifiedAt time.Time
}
