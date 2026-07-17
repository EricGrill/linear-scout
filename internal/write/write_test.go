package write

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/EricGrill/linear-scout/internal/drafts"
	"github.com/EricGrill/linear-scout/internal/linear"
	"github.com/EricGrill/linear-scout/internal/model"
	"github.com/EricGrill/linear-scout/internal/store"
)

func TestBuildIssuePlansIsPure(t *testing.T) {
	ds := []drafts.Draft{
		{Title: "Fix crash", Body: "steps", Priority: 2, Evidence: []model.EvidenceLink{{Ref: "ENG-1"}}},
	}
	plans := BuildIssuePlans(ds, "team-1")
	if len(plans) != 1 {
		t.Fatalf("want 1 plan, got %d", len(plans))
	}
	p := plans[0]
	if p.Action != ActionCreateIssue || p.Issue == nil || p.Issue.TeamID != "team-1" {
		t.Fatalf("bad plan: %+v", p)
	}
	if len(p.Evidence) != 1 || p.Evidence[0] != "ENG-1" {
		t.Fatalf("evidence not carried: %+v", p.Evidence)
	}
}

// stubWriter records calls and returns configured results.
type stubWriter struct {
	created  int
	comments int
	labels   int
	failLabels bool
}

func (s *stubWriter) CreateIssue(context.Context, linear.CreateIssueInput) (linear.CreatedIssue, error) {
	s.created++
	return linear.CreatedIssue{Identifier: "ENG-9", URL: "https://l/ENG-9"}, nil
}
func (s *stubWriter) CreateComment(context.Context, string, string) (linear.CreatedComment, error) {
	s.comments++
	return linear.CreatedComment{URL: "https://l/c1"}, nil
}
func (s *stubWriter) AddLabels(context.Context, string, []string) error {
	s.labels++
	if s.failLabels {
		return errors.New("label failure")
	}
	return nil
}

type memAudit struct{ entries []store.AuditEntry }

func (m *memAudit) AppendAudit(e store.AuditEntry) error {
	m.entries = append(m.entries, e)
	return nil
}

func TestExecuteRunsPlansAndAudits(t *testing.T) {
	w := &stubWriter{}
	audit := &memAudit{}
	now := time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC)
	plans := []Plan{
		{Action: ActionCreateIssue, Summary: "Create issue: X", Issue: &linear.CreateIssueInput{Title: "X", TeamID: "t1"}, Evidence: []string{"ENG-1"}},
		{Action: ActionComment, Target: "i1", Body: "hi"},
	}
	results, err := Execute(context.Background(), w, audit, plans, now)
	if err != nil {
		t.Fatal(err)
	}
	if w.created != 1 || w.comments != 1 {
		t.Fatalf("writer not called correctly: %+v", w)
	}
	if len(results) != 2 || !results[0].OK || !results[1].OK {
		t.Fatalf("bad results: %+v", results)
	}
	// Created-issue plan's target should be filled from the created identifier.
	if results[0].Plan.Target != "ENG-9" {
		t.Fatalf("target not set from created issue: %+v", results[0])
	}
	if len(audit.entries) != 2 || audit.entries[0].Evidence[0] != "ENG-1" {
		t.Fatalf("audit not recorded: %+v", audit.entries)
	}
}

func TestExecuteContinuesPastFailure(t *testing.T) {
	w := &stubWriter{failLabels: true}
	audit := &memAudit{}
	now := time.Now()
	plans := []Plan{
		{Action: ActionAddLabels, Target: "i1", Labels: []string{"bug"}},
		{Action: ActionComment, Target: "i1", Body: "still runs"},
	}
	results, err := Execute(context.Background(), w, audit, plans, now)
	if err != nil {
		t.Fatal(err)
	}
	if results[0].OK {
		t.Fatalf("expected first plan to fail: %+v", results[0])
	}
	if !results[1].OK {
		t.Fatalf("expected second plan to still run: %+v", results[1])
	}
	// Only the successful comment should be audited.
	if len(audit.entries) != 1 || audit.entries[0].Action != string(ActionComment) {
		t.Fatalf("audit should record only successes: %+v", audit.entries)
	}
}

func TestRenderPlansDryRun(t *testing.T) {
	out := RenderPlans([]Plan{{Action: ActionComment, Target: "ENG-1", Body: "note"}})
	if !contains(out, "dry-run") || !contains(out, "ENG-1") {
		t.Fatalf("bad preview: %s", out)
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (indexOf(s, sub) >= 0) }
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// Ensure *linear.Client satisfies Writer at compile time.
var _ Writer = (*linear.Client)(nil)
