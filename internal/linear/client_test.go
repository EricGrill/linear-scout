package linear

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const issuesFixture = `{"data":{"issues":{"nodes":[
  {"id":"i1","identifier":"ENG-1","title":"Crash on login","description":"stack trace",
   "priority":2,"url":"https://linear.app/x/issue/ENG-1","createdAt":"2026-07-10T10:00:00Z","updatedAt":"2026-07-11T10:00:00Z",
   "state":{"name":"In Progress"},"assignee":{"name":"Ana"},"project":{"id":"p1","name":"Core"},"team":{"id":"t1","name":"Eng"},
   "labels":{"nodes":[{"name":"bug"}]}}
],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}`

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

// TestIssuesPaginates verifies the client follows pageInfo across multiple pages.
func TestIssuesPaginates(t *testing.T) {
	page1 := `{"data":{"issues":{"nodes":[
	  {"id":"i1","identifier":"ENG-1"},{"id":"i2","identifier":"ENG-2"}
	],"pageInfo":{"hasNextPage":true,"endCursor":"CURSOR1"}}}}`
	page2 := `{"data":{"issues":{"nodes":[
	  {"id":"i3","identifier":"ENG-3"}
	],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}`

	var gotCursor string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Variables map[string]any `json:"variables"`
		}
		json.Unmarshal(body, &req)
		w.Header().Set("Content-Type", "application/json")
		if after, ok := req.Variables["after"].(string); ok && after == "CURSOR1" {
			gotCursor = after
			w.Write([]byte(page2))
			return
		}
		w.Write([]byte(page1))
	}))
	defer srv.Close()

	c := New(srv.URL, "lin_test", srv.Client())
	issues, err := c.Issues(context.Background(), time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 3 {
		t.Fatalf("want 3 issues across 2 pages, got %d", len(issues))
	}
	if gotCursor != "CURSOR1" {
		t.Fatalf("second page did not send endCursor, got %q", gotCursor)
	}
}

func TestProjectNamesPaginates(t *testing.T) {
	page1 := `{"data":{"projects":{"nodes":[{"id":"p1","name":"Core"}],"pageInfo":{"hasNextPage":true,"endCursor":"C1"}}}}`
	page2 := `{"data":{"projects":{"nodes":[{"id":"p2","name":"Mobile"}],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Variables map[string]any `json:"variables"`
		}
		json.Unmarshal(body, &req)
		w.Header().Set("Content-Type", "application/json")
		if after, ok := req.Variables["after"].(string); ok && after == "C1" {
			w.Write([]byte(page2))
			return
		}
		w.Write([]byte(page1))
	}))
	defer srv.Close()

	c := New(srv.URL, "lin_test", srv.Client())
	names, err := c.ProjectNames(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if names["p1"] != "Core" || names["p2"] != "Mobile" {
		t.Fatalf("bad project names across pages: %+v", names)
	}
}
