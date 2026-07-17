package cli

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/EricGrill/linear-scout/internal/ai"
	"github.com/EricGrill/linear-scout/internal/ingest"
	"github.com/EricGrill/linear-scout/internal/linear"
	"github.com/EricGrill/linear-scout/internal/model"
	"github.com/EricGrill/linear-scout/internal/store"
)

type stubWriter struct{ created, comments, labels int }

func (s *stubWriter) CreateIssue(context.Context, linear.CreateIssueInput) (linear.CreatedIssue, error) {
	s.created++
	return linear.CreatedIssue{Identifier: "ENG-9", URL: "https://l/ENG-9"}, nil
}
func (s *stubWriter) CreateComment(context.Context, string, string) (linear.CreatedComment, error) {
	s.comments++
	return linear.CreatedComment{URL: "https://l/c1"}, nil
}
func (s *stubWriter) AddLabels(context.Context, string, []string) error { s.labels++; return nil }

type memAudit struct{ entries []store.AuditEntry }

func (m *memAudit) AppendAudit(e store.AuditEntry) error {
	m.entries = append(m.entries, e)
	return nil
}

type stubSrc struct{}

func (stubSrc) Issues(context.Context, time.Time) ([]model.Issue, error) {
	return []model.Issue{{ID: "i1", Identifier: "ENG-1", URL: "https://l/ENG-1", ProjectID: "p1", Title: "Crash", Labels: []string{"bug"}}}, nil
}
func (stubSrc) Comments(context.Context, time.Time) ([]model.Comment, error) { return nil, nil }

func TestReportCommandRendersMarkdown(t *testing.T) {
	root := NewRootCmd()
	// Inject deterministic source + provider.
	testDeps = &deps{
		source: stubSrc{},
		provider: ai.StubProvider{Recs: []model.Recommendation{
			{Summary: "Fix crash", Confidence: 0.9, Evidence: []model.EvidenceLink{{Ref: "ENG-1", URL: "https://l/ENG-1"}}},
		}},
		now: time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC),
	}
	defer func() { testDeps = nil }()

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"report", "--since", "7d", "--group-by", "project", "--format", "markdown"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("Fix crash")) {
		t.Fatalf("report output missing rec:\n%s", out.String())
	}
	_ = ingest.Source(stubSrc{}) // ensure interface satisfied
}

// TestReportUsesLearnedMappings proves a learned label→app mapping flows through
// the CLI seam into grouping (observable via the JSON report's Groups).
func TestReportUsesLearnedMappings(t *testing.T) {
	root := NewRootCmd()
	testDeps = &deps{
		source:   stubSrc{},
		provider: ai.StubProvider{Recs: nil},
		now:      time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC),
		mappings: map[string]string{"label:bug": "CoreApp"},
	}
	defer func() { testDeps = nil }()

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"report", "--format", "json"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("CoreApp")) {
		t.Fatalf("learned mapping did not affect grouping:\n%s", out.String())
	}
}

func writeTestDeps() (*stubWriter, *memAudit) {
	w := &stubWriter{}
	a := &memAudit{}
	testDeps = &deps{
		source: stubSrc{},
		provider: ai.StubProvider{Recs: []model.Recommendation{
			{Summary: "Fix crash", Confidence: 0.9, DraftTitle: "Fix login crash", DraftBody: "steps",
				Evidence: []model.EvidenceLink{{Ref: "ENG-1", URL: "https://l/ENG-1"}}},
		}},
		now:    time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC),
		writer: w,
		audit:  a,
	}
	return w, a
}

func TestCreateIssuesDryRunDoesNotWrite(t *testing.T) {
	w, a := writeTestDeps()
	defer func() { testDeps = nil }()

	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"create-issues", "--team", "t1"}) // no --execute
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if w.created != 0 || len(a.entries) != 0 {
		t.Fatalf("dry-run must not write: created=%d audited=%d", w.created, len(a.entries))
	}
	if !bytes.Contains(out.Bytes(), []byte("dry-run")) || !bytes.Contains(out.Bytes(), []byte("Fix login crash")) {
		t.Fatalf("dry-run output missing plan:\n%s", out.String())
	}
}

func TestCreateIssuesExecuteWrites(t *testing.T) {
	w, a := writeTestDeps()
	defer func() { testDeps = nil }()

	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"create-issues", "--team", "t1", "--execute", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if w.created != 1 {
		t.Fatalf("execute should create 1 issue, got %d", w.created)
	}
	if len(a.entries) != 1 || a.entries[0].Action != "create-issue" {
		t.Fatalf("execute should audit the write: %+v", a.entries)
	}
}

func TestCreateIssuesRequiresTeam(t *testing.T) {
	writeTestDeps()
	defer func() { testDeps = nil }()
	root := NewRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetArgs([]string{"create-issues"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error without --team")
	}
}

func TestCommentExecuteWrites(t *testing.T) {
	w, a := writeTestDeps()
	defer func() { testDeps = nil }()

	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"comment", "--issue", "ENG-1", "--body", "looks stale", "--execute", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if w.comments != 1 || len(a.entries) != 1 {
		t.Fatalf("comment not written/audited: comments=%d audited=%d", w.comments, len(a.entries))
	}
}
