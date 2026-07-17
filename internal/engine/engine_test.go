package engine

import (
	"context"
	"testing"
	"time"

	"github.com/EricGrill/linear-scout/internal/ai"
	"github.com/EricGrill/linear-scout/internal/config"
	"github.com/EricGrill/linear-scout/internal/model"
)

type fakeSrc struct{}

func (fakeSrc) Issues(context.Context, time.Time) ([]model.Issue, error) {
	return []model.Issue{{ID: "i1", Identifier: "ENG-1", URL: "https://l/ENG-1", ProjectID: "p1", Title: "Crash"}}, nil
}
func (fakeSrc) Comments(context.Context, time.Time) ([]model.Comment, error) { return nil, nil }

func TestRunProducesReport(t *testing.T) {
	prov := ai.StubProvider{Recs: []model.Recommendation{
		{Summary: "Fix crash", Confidence: 0.8, Evidence: []model.EvidenceLink{{Ref: "ENG-1"}}},
		{Summary: "weak", Confidence: 0.1, Evidence: []model.EvidenceLink{{Ref: "ENG-1"}}},
	}}
	rep, err := Run(context.Background(), fakeSrc{}, prov, Options{
		Window: "7d", GroupBy: "project", Limit: 10,
		Now:    time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC),
		Rubric: config.RubricConfig{MinConfidence: 0.5, RequireEvidence: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Recommendations) != 1 || rep.Recommendations[0].Summary != "Fix crash" {
		t.Fatalf("validator not applied: %+v", rep.Recommendations)
	}
	if len(rep.Groups) != 1 || rep.Window != "7d" {
		t.Fatalf("bad report: %+v", rep)
	}
}

func TestRunAppliesLimit(t *testing.T) {
	recs := []model.Recommendation{}
	for i := 0; i < 5; i++ {
		recs = append(recs, model.Recommendation{Summary: "r", Confidence: 0.9, Evidence: []model.EvidenceLink{{Ref: "ENG-1"}}})
	}
	rep, _ := Run(context.Background(), fakeSrc{}, ai.StubProvider{Recs: recs}, Options{
		Window: "7d", GroupBy: "project", Limit: 2,
		Now:    time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC),
		Rubric: config.RubricConfig{MinConfidence: 0.5},
	})
	if len(rep.Recommendations) != 2 {
		t.Fatalf("limit not applied: got %d", len(rep.Recommendations))
	}
}
