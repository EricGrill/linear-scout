package cli

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/EricGrill/linear-scout/internal/model"
	"github.com/EricGrill/linear-scout/internal/store"
)

// learnSrc returns three issues all labelled "bug" in one project → a clean
// learning signal.
type learnSrc struct{}

func (learnSrc) Issues(context.Context, time.Time) ([]model.Issue, error) {
	return []model.Issue{
		{ID: "i1", ProjectID: "p1", Labels: []string{"bug"}},
		{ID: "i2", ProjectID: "p1", Labels: []string{"bug"}},
		{ID: "i3", ProjectID: "p1", Labels: []string{"bug"}},
	}, nil
}
func (learnSrc) Comments(context.Context, time.Time) ([]model.Comment, error) { return nil, nil }

func TestCorrectRecordsMapping(t *testing.T) {
	st := store.New(t.TempDir())
	testDeps = &deps{profileStore: st}
	defer func() { testDeps = nil }()

	root := NewRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetArgs([]string{"correct", "--label", "bug", "--app", "CoreApp"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	lp, _ := st.LoadLearned()
	if lp.AppMappings["label:bug"] != "CoreApp" {
		t.Fatalf("correction not recorded: %+v", lp.AppMappings)
	}
}

func TestFeedbackRecordsDecision(t *testing.T) {
	st := store.New(t.TempDir())
	testDeps = &deps{profileStore: st, now: time.Unix(10, 0).UTC()}
	defer func() { testDeps = nil }()

	root := NewRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetArgs([]string{"feedback", "--rec", "Fix crash", "--reject"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	lp, _ := st.LoadLearned()
	if len(lp.History) != 1 || lp.History[0].Accepted || lp.History[0].RecSummary != "Fix crash" {
		t.Fatalf("decision not recorded: %+v", lp.History)
	}
}

func TestFeedbackRequiresExactlyOneDecision(t *testing.T) {
	testDeps = &deps{profileStore: store.New(t.TempDir())}
	defer func() { testDeps = nil }()
	root := NewRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetArgs([]string{"feedback", "--rec", "X"}) // neither accept nor reject
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when no decision flag is set")
	}
}

func TestLearnRunDryRunThenApply(t *testing.T) {
	st := store.New(t.TempDir())
	testDeps = &deps{source: learnSrc{}, profileStore: st, now: time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC)}
	defer func() { testDeps = nil }()

	// Dry-run: proposes but does not write.
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"learn", "run", "--min-support", "2", "--min-purity", "0.75"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("label:bug")) {
		t.Fatalf("dry-run should propose label:bug:\n%s", out.String())
	}
	lp, _ := st.LoadLearned()
	if len(lp.AppMappings) != 0 {
		t.Fatalf("dry-run must not write, got %+v", lp.AppMappings)
	}

	// Apply: writes the accepted mapping.
	root2 := NewRootCmd()
	root2.SetOut(&bytes.Buffer{})
	root2.SetArgs([]string{"learn", "run", "--min-support", "2", "--min-purity", "0.75", "--apply"})
	if err := root2.Execute(); err != nil {
		t.Fatal(err)
	}
	lp, _ = st.LoadLearned()
	if lp.AppMappings["label:bug"] != "p1" {
		t.Fatalf("apply should record label:bug → p1, got %+v", lp.AppMappings)
	}
}

func TestLearnInspectShowsCounts(t *testing.T) {
	st := store.New(t.TempDir())
	st.MergeMappings(map[string]string{"label:bug": "Core"})
	st.RecordDecision(store.HistoryEntry{RecSummary: "r", Accepted: true})
	testDeps = &deps{profileStore: st}
	defer func() { testDeps = nil }()

	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"learn", "inspect"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("label:bug → Core")) || !bytes.Contains(out.Bytes(), []byte("1 accepted")) {
		t.Fatalf("inspect output wrong:\n%s", out.String())
	}
}
