package evidence

import (
	"testing"

	"github.com/EricGrill/linear-scout/internal/model"
)

func TestBuildLinksAndRanks(t *testing.T) {
	act := model.Activity{
		Issues: []model.Issue{
			{ID: "i1", Identifier: "ENG-1", URL: "https://l/ENG-1", Title: "Crash"},
		},
		Comments: []model.Comment{
			{ID: "c1", IssueID: "i1", URL: "https://l/c1", Body: "still broken"},
		},
	}
	groups := []model.Group{{Key: "project:p1", IssueIDs: []string{"i1"}}}
	bundles := Build(act, groups)
	b, ok := bundles["project:p1"]
	if !ok {
		t.Fatal("missing bundle")
	}
	if len(b.Links) != 2 {
		t.Fatalf("want 2 links (issue+comment), got %d", len(b.Links))
	}
	if b.Score <= 0 {
		t.Fatalf("want positive score, got %v", b.Score)
	}
	if b.Links[0].URL == "" || b.Links[0].Ref == "" {
		t.Fatalf("evidence link missing url/ref: %+v", b.Links[0])
	}
}
