package ai

import (
	"context"
	"strings"
	"testing"

	"github.com/EricGrill/linear-scout/internal/config"
	"github.com/EricGrill/linear-scout/internal/model"
)

func TestAssemblePromptIncludesEvidenceRefs(t *testing.T) {
	req := Request{
		Groups: []model.Group{{Key: "project:p1", Label: "P1"}},
		Evidence: map[string]model.EvidenceBundle{
			"project:p1": {Links: []model.EvidenceLink{{Ref: "ENG-1", URL: "https://l/ENG-1"}}},
		},
	}
	p := AssemblePrompt(req)
	if !strings.Contains(p, "ENG-1") || !strings.Contains(p, "P1") {
		t.Fatalf("prompt missing content:\n%s", p)
	}
}

func TestValidateDropsWeakRecs(t *testing.T) {
	recs := []model.Recommendation{
		{Summary: "good", Confidence: 0.8, Evidence: []model.EvidenceLink{{Ref: "ENG-1"}}},
		{Summary: "low-conf", Confidence: 0.2, Evidence: []model.EvidenceLink{{Ref: "ENG-2"}}},
		{Summary: "no-evidence", Confidence: 0.9},
	}
	out := Validate(recs, config.RubricConfig{MinConfidence: 0.5, RequireEvidence: true})
	if len(out) != 1 || out[0].Summary != "good" {
		t.Fatalf("validate kept wrong recs: %+v", out)
	}
}

func TestStubProviderReturnsConfigured(t *testing.T) {
	stub := StubProvider{Recs: []model.Recommendation{{Summary: "x"}}}
	got, err := stub.Recommend(context.Background(), Request{})
	if err != nil || len(got) != 1 {
		t.Fatalf("stub failed: %v %+v", err, got)
	}
}
