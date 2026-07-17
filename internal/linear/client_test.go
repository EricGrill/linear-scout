package linear

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const issuesFixture = `{"data":{"issues":{"nodes":[
  {"id":"i1","identifier":"ENG-1","title":"Crash on login","description":"stack trace",
   "priority":2,"url":"https://linear.app/x/issue/ENG-1","createdAt":"2026-07-10T10:00:00Z","updatedAt":"2026-07-11T10:00:00Z",
   "state":{"name":"In Progress"},"assignee":{"name":"Ana"},"project":{"id":"p1"},"team":{"id":"t1"},
   "labels":{"nodes":[{"name":"bug"}]}}
]}}}`

func TestIssuesParsesNodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(issuesFixture))
	}))
	defer srv.Close()

	c := New(srv.URL, "lin_test", srv.Client())
	issues, err := c.Issues(context.Background(), time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 1 {
		t.Fatalf("want 1 issue, got %d", len(issues))
	}
	got := issues[0]
	if got.Identifier != "ENG-1" || got.State != "In Progress" || got.Assignee != "Ana" {
		t.Fatalf("bad mapping: %+v", got)
	}
	if len(got.Labels) != 1 || got.Labels[0] != "bug" || !strings.HasPrefix(got.URL, "https://") {
		t.Fatalf("bad labels/url: %+v", got)
	}
}
