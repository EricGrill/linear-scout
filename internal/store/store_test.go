package store

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadLearnedMissingReturnsEmpty(t *testing.T) {
	s := New(t.TempDir())
	lp, err := s.LoadLearned()
	if err != nil {
		t.Fatal(err)
	}
	if len(lp.AppMappings) != 0 || len(lp.History) != 0 {
		t.Fatalf("expected empty, got %+v", lp)
	}
}

func TestSaveThenLoadRoundTrip(t *testing.T) {
	s := New(t.TempDir())
	in := LearnedProfile{AppMappings: map[string]string{"label:bug": "CoreApp"}}
	if err := s.SaveLearned(in); err != nil {
		t.Fatal(err)
	}
	out, err := s.LoadLearned()
	if err != nil {
		t.Fatal(err)
	}
	if out.AppMappings["label:bug"] != "CoreApp" {
		t.Fatalf("round trip lost data: %+v", out)
	}
}

func TestExportAndDelete(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	s.SaveLearned(LearnedProfile{AppMappings: map[string]string{"a": "b"}})
	var buf bytes.Buffer
	if err := s.Export(&buf); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"a": "b"`)) {
		t.Fatalf("export missing data: %s", buf.String())
	}
	if err := s.Delete(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "learned.json")); !os.IsNotExist(err) {
		t.Fatalf("delete did not remove file")
	}
}
