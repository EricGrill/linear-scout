package learn

import (
	"testing"

	"github.com/EricGrill/linear-scout/internal/model"
	"github.com/EricGrill/linear-scout/internal/store"
)

func TestLabelMappingProposesClusteredLabel(t *testing.T) {
	// "bug" appears on 3 issues, all in the Core group → high purity.
	act := model.Activity{Issues: []model.Issue{
		{ID: "i1", Labels: []string{"bug"}},
		{ID: "i2", Labels: []string{"bug"}},
		{ID: "i3", Labels: []string{"bug"}},
	}}
	groups := []model.Group{{Label: "Core", Kind: "project", IssueIDs: []string{"i1", "i2", "i3"}}}

	art, accepted, _ := RunPass(LabelMappingMission{}, PurityEvaluator{MinSupport: 2, MinPurity: 0.75},
		Input{Activity: act, Groups: groups})
	if len(art.Candidates) != 1 {
		t.Fatalf("want 1 candidate, got %+v", art.Candidates)
	}
	if len(accepted) != 1 || accepted[0].Key != "label:bug" || accepted[0].App != "Core" {
		t.Fatalf("bad acceptance: %+v", accepted)
	}
	if accepted[0].Purity != 1.0 || accepted[0].Support != 3 {
		t.Fatalf("bad support/purity: %+v", accepted[0])
	}
}

func TestLabelMappingRejectsSpreadLabel(t *testing.T) {
	// "ui" is split 1/1 across two groups → purity 0.5, below the bar.
	act := model.Activity{Issues: []model.Issue{
		{ID: "i1", Labels: []string{"ui"}},
		{ID: "i2", Labels: []string{"ui"}},
	}}
	groups := []model.Group{
		{Label: "Core", Kind: "project", IssueIDs: []string{"i1"}},
		{Label: "Web", Kind: "project", IssueIDs: []string{"i2"}},
	}
	_, accepted, _ := RunPass(LabelMappingMission{}, PurityEvaluator{MinSupport: 1, MinPurity: 0.75},
		Input{Activity: act, Groups: groups})
	if len(accepted) != 0 {
		t.Fatalf("spread label should be rejected, got %+v", accepted)
	}
}

func TestLabelMappingRejectsBelowSupport(t *testing.T) {
	act := model.Activity{Issues: []model.Issue{{ID: "i1", Labels: []string{"rare"}}}}
	groups := []model.Group{{Label: "Core", Kind: "project", IssueIDs: []string{"i1"}}}
	_, accepted, _ := RunPass(LabelMappingMission{}, PurityEvaluator{MinSupport: 3, MinPurity: 0.5},
		Input{Activity: act, Groups: groups})
	if len(accepted) != 0 {
		t.Fatalf("below-support label should be rejected, got %+v", accepted)
	}
}

func TestLabelMappingSkipsKnownAndUnclassified(t *testing.T) {
	act := model.Activity{Issues: []model.Issue{
		{ID: "i1", Labels: []string{"bug"}},          // already known
		{ID: "i2", Labels: []string{"orphan"}},       // only in unclassified
	}}
	groups := []model.Group{
		{Label: "Core", Kind: "project", IssueIDs: []string{"i1"}},
		{Label: "Unclassified", Kind: "unclassified", IssueIDs: []string{"i2"}},
	}
	existing := store.LearnedProfile{AppMappings: map[string]string{"label:bug": "Core"}}
	art := LabelMappingMission{}.Run(Input{Activity: act, Groups: groups, Existing: existing})
	if len(art.Candidates) != 0 {
		t.Fatalf("should skip known + unclassified, got %+v", art.Candidates)
	}
}
