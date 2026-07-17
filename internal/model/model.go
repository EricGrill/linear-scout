// Package model holds the domain types shared across every linear-scout package.
package model

import "time"

// Confidence is a 0..1 score attached to classifications and recommendations.
type Confidence float64

// Band buckets a Confidence into "low" (<0.5), "medium" (<0.75), or "high".
func (c Confidence) Band() string {
	switch {
	case c >= 0.75:
		return "high"
	case c >= 0.5:
		return "medium"
	default:
		return "low"
	}
}

// Issue is a normalized Linear issue.
type Issue struct {
	ID         string
	Identifier string // e.g. "ENG-123"
	Title      string
	Body       string
	State      string
	Priority   int
	Labels     []string
	Assignee   string
	ProjectID  string
	TeamID     string
	URL        string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Comment is a normalized Linear comment.
type Comment struct {
	ID        string
	IssueID   string
	Author    string
	Body      string
	URL       string
	CreatedAt time.Time
}

// Activity is the normalized dataset for a time window.
type Activity struct {
	Since    time.Time
	Until    time.Time
	Issues   []Issue
	Comments []Comment
	Projects map[string]string // id -> name
	Teams    map[string]string // id -> name
}

// Group is an inferred app/product/project/team bucket.
type Group struct {
	Key        string // stable key, e.g. project id or learned mapping key
	Label      string // human label
	Kind       string // "app" | "product" | "project" | "team" | "unclassified"
	Confidence Confidence
	IssueIDs   []string
}

// EvidenceLink points back to a source Linear record.
type EvidenceLink struct {
	Kind  string // "issue" | "comment"
	Ref   string // identifier, e.g. "ENG-123"
	URL   string
	Quote string // short supporting excerpt
}

// EvidenceBundle is the ranked, auditable support for a recommendation.
type EvidenceBundle struct {
	GroupKey string
	Links    []EvidenceLink
	Score    float64 // ranking score, higher = stronger
}

// Recommendation is the stable structured output of the AI layer.
type Recommendation struct {
	Summary           string
	WhyItMatters      string
	Evidence          []EvidenceLink
	Confidence        Confidence
	AffectedGroup     string
	SuggestedOwner    string
	DraftTitle        string
	DraftBody         string
	SuggestedLabels   []string
	SuggestedPriority int
	DuplicateRisk     string
}

// Report is the surface-agnostic result every delivery surface renders.
type Report struct {
	GeneratedAt     time.Time
	Window          string // e.g. "7d"
	GroupBy         string
	Groups          []Group
	Recommendations []Recommendation
	Unclassified    int
}
