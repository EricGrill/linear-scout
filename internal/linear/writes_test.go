package linear

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func serverFor(t *testing.T, route func(query string, w http.ResponseWriter)) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		route(string(body), w)
	}))
	t.Cleanup(srv.Close)
	return New(srv.URL, "lin_test", srv.Client())
}

func TestCreateIssue(t *testing.T) {
	c := serverFor(t, func(q string, w http.ResponseWriter) {
		if !strings.Contains(q, "issueCreate") {
			t.Fatalf("unexpected query: %s", q)
		}
		w.Write([]byte(`{"data":{"issueCreate":{"success":true,"issue":{"id":"i9","identifier":"ENG-9","url":"https://l/ENG-9"}}}}`))
	})
	got, err := c.CreateIssue(context.Background(), CreateIssueInput{Title: "New", TeamID: "t1"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Identifier != "ENG-9" || got.URL != "https://l/ENG-9" {
		t.Fatalf("bad created issue: %+v", got)
	}
}

func TestCreateIssueRequiresTitleAndTeam(t *testing.T) {
	c := New("http://unused", "tok", nil)
	if _, err := c.CreateIssue(context.Background(), CreateIssueInput{Title: "x"}); err == nil {
		t.Fatal("expected error without teamId")
	}
}

func TestCreateComment(t *testing.T) {
	c := serverFor(t, func(q string, w http.ResponseWriter) {
		if !strings.Contains(q, "commentCreate") {
			t.Fatalf("unexpected query: %s", q)
		}
		w.Write([]byte(`{"data":{"commentCreate":{"success":true,"comment":{"id":"c1","url":"https://l/c1"}}}}`))
	})
	got, err := c.CreateComment(context.Background(), "i1", "hello")
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != "https://l/c1" {
		t.Fatalf("bad created comment: %+v", got)
	}
}

func TestAddLabelsMergesWithExisting(t *testing.T) {
	var sentIDs string
	c := serverFor(t, func(q string, w http.ResponseWriter) {
		switch {
		case strings.Contains(q, "IssueLabelContext"):
			w.Write([]byte(`{"data":{"issue":{
			  "labels":{"nodes":[{"id":"L1","name":"existing"}]},
			  "team":{"labels":{"nodes":[{"id":"L1","name":"existing"},{"id":"L2","name":"bug"}]}}
			}}}`))
		case strings.Contains(q, "issueUpdate"):
			sentIDs = q
			w.Write([]byte(`{"data":{"issueUpdate":{"success":true}}}`))
		default:
			t.Fatalf("unexpected query: %s", q)
		}
	})
	if err := c.AddLabels(context.Background(), "i1", []string{"bug"}); err != nil {
		t.Fatal(err)
	}
	// The update must include both the pre-existing L1 and the new L2.
	if !strings.Contains(sentIDs, "L1") || !strings.Contains(sentIDs, "L2") {
		t.Fatalf("update did not merge label ids: %s", sentIDs)
	}
}

func TestAddLabelsUnknownName(t *testing.T) {
	c := serverFor(t, func(q string, w http.ResponseWriter) {
		w.Write([]byte(`{"data":{"issue":{"labels":{"nodes":[]},"team":{"labels":{"nodes":[{"id":"L2","name":"bug"}]}}}}}`))
	})
	if err := c.AddLabels(context.Background(), "i1", []string{"nonexistent"}); err == nil {
		t.Fatal("expected error for unknown label name")
	}
}
