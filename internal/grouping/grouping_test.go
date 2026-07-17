package grouping

import (
	"testing"

	"github.com/EricGrill/linear-scout/internal/model"
)

func TestClassifyByLearnedMapping(t *testing.T) {
	act := model.Activity{Issues: []model.Issue{
		{ID: "i1", Labels: []string{"bug"}, ProjectID: "p1"},
	}}
	groups, unclassified := Classify(act, map[string]string{"label:bug": "CoreApp"}, "project")
	if unclassified != 0 {
		t.Fatalf("unclassified=%d", unclassified)
	}
	if len(groups) != 1 || groups[0].Kind != "app" || groups[0].Label != "CoreApp" {
		t.Fatalf("bad group: %+v", groups)
	}
	if groups[0].Confidence.Band() != "high" {
		t.Fatalf("want high confidence, got %s", groups[0].Confidence.Band())
	}
}

func TestClassifyUnclassified(t *testing.T) {
	act := model.Activity{Issues: []model.Issue{{ID: "i2"}}}
	groups, unclassified := Classify(act, nil, "project")
	if unclassified != 1 {
		t.Fatalf("unclassified=%d", unclassified)
	}
	if len(groups) != 1 || groups[0].Kind != "unclassified" || groups[0].Confidence.Band() != "low" {
		t.Fatalf("bad group: %+v", groups)
	}
}
