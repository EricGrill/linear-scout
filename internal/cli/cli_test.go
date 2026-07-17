package cli

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/EricGrill/linear-scout/internal/ai"
	"github.com/EricGrill/linear-scout/internal/ingest"
	"github.com/EricGrill/linear-scout/internal/model"
)

type stubSrc struct{}

func (stubSrc) Issues(context.Context, time.Time) ([]model.Issue, error) {
	return []model.Issue{{ID: "i1", Identifier: "ENG-1", URL: "https://l/ENG-1", ProjectID: "p1", Title: "Crash"}}, nil
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
