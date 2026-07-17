package store

import (
	"testing"
	"time"
)

func TestMergeMappingsPreservesExisting(t *testing.T) {
	s := New(t.TempDir())
	if err := s.SaveLearned(LearnedProfile{AppMappings: map[string]string{"label:a": "AppA"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.MergeMappings(map[string]string{"label:b": "AppB"}); err != nil {
		t.Fatal(err)
	}
	lp, _ := s.LoadLearned()
	if lp.AppMappings["label:a"] != "AppA" || lp.AppMappings["label:b"] != "AppB" {
		t.Fatalf("merge lost or missed data: %+v", lp.AppMappings)
	}
}

func TestMergeMappingsOnEmptyProfile(t *testing.T) {
	s := New(t.TempDir())
	if err := s.MergeMappings(map[string]string{"label:x": "X"}); err != nil {
		t.Fatal(err)
	}
	lp, _ := s.LoadLearned()
	if lp.AppMappings["label:x"] != "X" {
		t.Fatalf("merge on empty failed: %+v", lp.AppMappings)
	}
}

func TestRecordDecisionAppends(t *testing.T) {
	s := New(t.TempDir())
	if err := s.RecordDecision(HistoryEntry{RecSummary: "Fix crash", Accepted: true, At: time.Unix(1, 0).UTC()}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordDecision(HistoryEntry{RecSummary: "Add retry", Accepted: false, At: time.Unix(2, 0).UTC()}); err != nil {
		t.Fatal(err)
	}
	lp, _ := s.LoadLearned()
	if len(lp.History) != 2 || lp.History[0].RecSummary != "Fix crash" || lp.History[1].Accepted {
		t.Fatalf("history wrong: %+v", lp.History)
	}
}
