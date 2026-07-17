package drafts

import (
	"testing"

	"github.com/EricGrill/linear-scout/internal/model"
)

func TestFromReportBuildsDrafts(t *testing.T) {
	r := model.Report{Recommendations: []model.Recommendation{
		{Summary: "Fix crash", DraftBody: "steps", Evidence: []model.EvidenceLink{{Ref: "ENG-1"}}},
		{Summary: "Add retry", DraftTitle: "Add retry to uploader", DraftBody: "why"},
	}}
	ds := FromReport(r)
	if len(ds) != 2 {
		t.Fatalf("want 2 drafts, got %d", len(ds))
	}
	if ds[0].Title != "Fix crash" { // falls back to summary
		t.Fatalf("draft0 title=%q", ds[0].Title)
	}
	if ds[1].Title != "Add retry to uploader" {
		t.Fatalf("draft1 title=%q", ds[1].Title)
	}
	if len(ds[0].Evidence) != 1 {
		t.Fatalf("draft0 evidence lost")
	}
}
