package ingest

import (
	"context"
	"testing"
	"time"

	"github.com/EricGrill/linear-scout/internal/model"
)

func TestParseWindow(t *testing.T) {
	now := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)
	cases := map[string]time.Time{
		"7d":  now.AddDate(0, 0, -7),
		"24h": now.Add(-24 * time.Hour),
		"2w":  now.AddDate(0, 0, -14),
	}
	for in, want := range cases {
		got, err := ParseWindow(in, now)
		if err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if !got.Equal(want) {
			t.Fatalf("%s: got %v want %v", in, got, want)
		}
	}
	if _, err := ParseWindow("bogus", now); err == nil {
		t.Fatal("expected error for bogus window")
	}
}

type fakeSource struct{ issues []model.Issue }

func (f fakeSource) Issues(context.Context, time.Time) ([]model.Issue, error) {
	return f.issues, nil
}
func (f fakeSource) Comments(context.Context, time.Time) ([]model.Comment, error) {
	return nil, nil
}

func TestFetchBuildsActivity(t *testing.T) {
	now := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)
	src := fakeSource{issues: []model.Issue{{ID: "i1", ProjectID: "p1", TeamID: "t1"}}}
	act, err := Fetch(context.Background(), src, "7d", now)
	if err != nil {
		t.Fatal(err)
	}
	if len(act.Issues) != 1 || act.Until != now {
		t.Fatalf("bad activity: %+v", act)
	}
}
