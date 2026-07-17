package learn

import "testing"

type fakeMission struct{ cands []Candidate }

func (fakeMission) Name() string { return "fake" }
func (f fakeMission) Run(Input) Artifact {
	return Artifact{Mission: "fake", Candidates: f.cands}
}

type thresholdEval struct{ minSupport int }

func (e thresholdEval) Evaluate(a Artifact) ([]Candidate, string) {
	var out []Candidate
	for _, c := range a.Candidates {
		if c.Support >= e.minSupport {
			out = append(out, c)
		}
	}
	return out, "kept candidates meeting support"
}

func TestRunPassGatesCandidates(t *testing.T) {
	m := fakeMission{cands: []Candidate{
		{Key: "label:bug", App: "Core", Support: 5, Purity: 1.0},
		{Key: "label:ui", App: "Web", Support: 1, Purity: 1.0},
	}}
	art, accepted, reason := RunPass(m, thresholdEval{minSupport: 2}, Input{})
	if art.Mission != "fake" || len(art.Candidates) != 2 {
		t.Fatalf("artifact wrong: %+v", art)
	}
	if len(accepted) != 1 || accepted[0].Key != "label:bug" {
		t.Fatalf("gate wrong: %+v", accepted)
	}
	if reason == "" {
		t.Fatal("expected a reason")
	}
}

func TestMappingsFromAccepted(t *testing.T) {
	got := Mappings([]Candidate{{Key: "label:bug", App: "Core"}})
	if got["label:bug"] != "Core" || len(got) != 1 {
		t.Fatalf("bad mappings: %+v", got)
	}
}
