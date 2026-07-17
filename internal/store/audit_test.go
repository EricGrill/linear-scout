package store

import (
	"testing"
	"time"
)

func TestAppendAndReadAudit(t *testing.T) {
	s := New(t.TempDir())
	e1 := AuditEntry{At: time.Unix(1000, 0).UTC(), Action: "create-issue", Target: "ENG-9", Summary: "Created", Evidence: []string{"ENG-1"}}
	e2 := AuditEntry{At: time.Unix(2000, 0).UTC(), Action: "comment", Target: "ENG-1", Summary: "Commented"}
	if err := s.AppendAudit(e1); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendAudit(e2); err != nil {
		t.Fatal(err)
	}
	got, err := s.ReadAudit()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 entries, got %d", len(got))
	}
	if got[0].Target != "ENG-9" || got[0].Evidence[0] != "ENG-1" {
		t.Fatalf("bad entry 0: %+v", got[0])
	}
	if got[1].Action != "comment" {
		t.Fatalf("bad entry 1: %+v", got[1])
	}
}

func TestReadAuditMissingReturnsEmpty(t *testing.T) {
	s := New(t.TempDir())
	got, err := s.ReadAudit()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("want empty, got %d", len(got))
	}
}
