# linear-scout Milestone 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the read-only CLI that ingests recent Linear activity and produces AI-generated, evidence-backed product recommendations plus draft issue metadata, rendered as Markdown, JSON, and Telegram text.

**Architecture:** A single Go module with a surface-agnostic `engine` package as the reuse seam (`engine.Run(ctx, opts) → Report`). Data flows config → linear → ingest → grouping → evidence → ai → engine → report/drafts. Only `internal/write` (M2, not in this plan) may mutate Linear; the M1 path is strictly read-only. Interfaces exist only at the four variation points: AI provider, storage, report renderer, and ingestion source.

**Tech Stack:** Go 1.22+, `github.com/spf13/cobra` (CLI), `github.com/spf13/viper`-free hand-rolled config via `gopkg.in/yaml.v3`, `github.com/machinebox/graphql` (Linear GraphQL), `github.com/sashabaranov/go-openai` (AI reference provider), stdlib `net/http/httptest` for tests. No cgo.

**Scope:** Milestone 1 only. M2 (writes), M3 (learning), M4 (surfaces) are separate plans. M1 includes a `preview` command that only prints dry-run intent (no write code), reserving the seam.

## Global Constraints

- Module path: `github.com/hitsnorth/linear-scout` (adjust if the repo owner differs).
- Go version floor: `go 1.22`.
- Read-only invariant: no code in this plan may call any Linear mutation. Only `list_*`/`get_*`-style GraphQL queries are permitted.
- Secrets rule: API tokens and provider credentials live ONLY in the user-local profile dir (`$XDG_CONFIG_HOME/linear-scout/`, fallback `~/.config/linear-scout/`), NEVER in the shared workspace config file.
- Every recommendation MUST carry ≥1 evidence link back to a source Linear record.
- Every package gets table-driven tests; external systems (Linear, OpenAI) are faked, never called over the network in tests.
- Commit after every task with a `feat:`/`test:`/`chore:` prefixed message.

---

### Task 1: Project scaffold + domain model

**Files:**
- Create: `go.mod`
- Create: `internal/model/model.go`
- Test: `internal/model/model_test.go`

**Interfaces:**
- Produces: core types consumed by every later task — `Activity`, `Issue`, `Comment`, `Group`, `Confidence`, `EvidenceLink`, `EvidenceBundle`, `Recommendation`, `Report`; helper `Confidence.Band() string`.

- [ ] **Step 1: Initialize the module**

```bash
go mod init github.com/hitsnorth/linear-scout
go mod edit -go=1.22
```

- [ ] **Step 2: Write the failing test**

`internal/model/model_test.go`:
```go
package model

import "testing"

func TestConfidenceBand(t *testing.T) {
	cases := []struct {
		name string
		in   Confidence
		want string
	}{
		{"high", 0.85, "high"},
		{"medium", 0.6, "medium"},
		{"low", 0.3, "low"},
		{"floor", 0.0, "low"},
		{"ceil", 1.0, "high"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.in.Band(); got != c.want {
				t.Fatalf("Band()=%q want %q", got, c.want)
			}
		})
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/model/`
Expected: FAIL — `undefined: Confidence`.

- [ ] **Step 4: Write the model**

`internal/model/model.go`:
```go
// Package model holds the domain types shared across every linear-scout package.
package model

import "time"

// Confidence is a 0..1 score attached to classifications and recommendations.
type Confidence float64

// Band buckets a Confidence into "low" (<0.5), "medium" (<0.75), or "high".
func (c Confidence) Band() string {
	switch {
	case c >= 0.75:
		return "high"
	case c >= 0.5:
		return "medium"
	default:
		return "low"
	}
}

// Issue is a normalized Linear issue.
type Issue struct {
	ID         string
	Identifier string // e.g. "ENG-123"
	Title      string
	Body       string
	State      string
	Priority   int
	Labels     []string
	Assignee   string
	ProjectID  string
	TeamID     string
	URL        string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Comment is a normalized Linear comment.
type Comment struct {
	ID        string
	IssueID   string
	Author    string
	Body      string
	URL       string
	CreatedAt time.Time
}

// Activity is the normalized dataset for a time window.
type Activity struct {
	Since    time.Time
	Until    time.Time
	Issues   []Issue
	Comments []Comment
	Projects map[string]string // id -> name
	Teams    map[string]string // id -> name
}

// Group is an inferred app/product/project/team bucket.
type Group struct {
	Key        string // stable key, e.g. project id or learned mapping key
	Label      string // human label
	Kind       string // "app" | "product" | "project" | "team" | "unclassified"
	Confidence Confidence
	IssueIDs   []string
}

// EvidenceLink points back to a source Linear record.
type EvidenceLink struct {
	Kind  string // "issue" | "comment"
	Ref   string // identifier, e.g. "ENG-123"
	URL   string
	Quote string // short supporting excerpt
}

// EvidenceBundle is the ranked, auditable support for a recommendation.
type EvidenceBundle struct {
	GroupKey string
	Links    []EvidenceLink
	Score    float64 // ranking score, higher = stronger
}

// Recommendation is the stable structured output of the AI layer.
type Recommendation struct {
	Summary        string
	WhyItMatters   string
	Evidence       []EvidenceLink
	Confidence     Confidence
	AffectedGroup  string
	SuggestedOwner string
	DraftTitle     string
	DraftBody      string
	SuggestedLabels []string
	SuggestedPriority int
	DuplicateRisk  string
}

// Report is the surface-agnostic result every delivery surface renders.
type Report struct {
	GeneratedAt     time.Time
	Window          string // e.g. "7d"
	GroupBy         string
	Groups          []Group
	Recommendations []Recommendation
	Unclassified    int
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/model/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add go.mod internal/model/
git commit -m "feat: add module scaffold and domain model"
```

---

### Task 2: Config — shared workspace + user-local profile

**Files:**
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `type Workspace struct { GroupBy string; Formats []string; Rubric RubricConfig; ... }`
  - `type RubricConfig struct { MinConfidence float64; RequireEvidence bool }`
  - `type Profile struct { LinearToken string; OpenAIKey string; Dir string }`
  - `func LoadWorkspace(path string) (Workspace, error)`
  - `func ProfileDir() (string, error)` — resolves `$XDG_CONFIG_HOME/linear-scout` or `~/.config/linear-scout`.
  - `func LoadProfile(dir string) (Profile, error)` — reads `profile.yaml`.

- [ ] **Step 1: Add yaml dependency**

```bash
go get gopkg.in/yaml.v3
```

- [ ] **Step 2: Write the failing test**

`internal/config/config_test.go`:
```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadWorkspaceDefaults(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "linear-scout.yaml")
	os.WriteFile(p, []byte("group_by: project\nformats: [markdown, json]\nrubric:\n  min_confidence: 0.5\n  require_evidence: true\n"), 0o644)
	w, err := LoadWorkspace(p)
	if err != nil {
		t.Fatal(err)
	}
	if w.GroupBy != "project" {
		t.Fatalf("GroupBy=%q", w.GroupBy)
	}
	if !w.Rubric.RequireEvidence || w.Rubric.MinConfidence != 0.5 {
		t.Fatalf("rubric=%+v", w.Rubric)
	}
}

func TestProfileDirRespectsXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdgtest")
	got, err := ProfileDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != "/tmp/xdgtest/linear-scout" {
		t.Fatalf("ProfileDir=%q", got)
	}
}

func TestLoadProfileReadsSecrets(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "profile.yaml"),
		[]byte("linear_token: lin_abc\nopenai_key: sk_xyz\n"), 0o600)
	prof, err := LoadProfile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if prof.LinearToken != "lin_abc" || prof.OpenAIKey != "sk_xyz" {
		t.Fatalf("profile=%+v", prof)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/config/`
Expected: FAIL — undefined identifiers.

- [ ] **Step 4: Write the config package**

`internal/config/config.go`:
```go
// Package config loads the shared workspace config and the user-local profile.
// Secrets live only in the profile; the workspace file must never hold them.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type RubricConfig struct {
	MinConfidence   float64 `yaml:"min_confidence"`
	RequireEvidence bool    `yaml:"require_evidence"`
}

type Workspace struct {
	GroupBy string       `yaml:"group_by"`
	Formats []string     `yaml:"formats"`
	Rubric  RubricConfig `yaml:"rubric"`
}

type Profile struct {
	LinearToken string `yaml:"linear_token"`
	OpenAIKey   string `yaml:"openai_key"`
	Dir         string `yaml:"-"`
}

// LoadWorkspace reads and parses the shared workspace config file.
func LoadWorkspace(path string) (Workspace, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Workspace{}, fmt.Errorf("read workspace config: %w", err)
	}
	var w Workspace
	if err := yaml.Unmarshal(b, &w); err != nil {
		return Workspace{}, fmt.Errorf("parse workspace config: %w", err)
	}
	if w.GroupBy == "" {
		w.GroupBy = "project"
	}
	if len(w.Formats) == 0 {
		w.Formats = []string{"markdown"}
	}
	return w, nil
}

// ProfileDir resolves the user-local profile directory.
func ProfileDir() (string, error) {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "linear-scout"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".config", "linear-scout"), nil
}

// LoadProfile reads profile.yaml (secrets) from dir.
func LoadProfile(dir string) (Profile, error) {
	b, err := os.ReadFile(filepath.Join(dir, "profile.yaml"))
	if err != nil {
		return Profile{}, fmt.Errorf("read profile: %w", err)
	}
	var p Profile
	if err := yaml.Unmarshal(b, &p); err != nil {
		return Profile{}, fmt.Errorf("parse profile: %w", err)
	}
	p.Dir = dir
	return p, nil
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/config/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/config/ go.mod go.sum
git commit -m "feat: add workspace + profile config loading"
```

---

### Task 3: Store — plain-file user-owned state

**Files:**
- Create: `internal/store/store.go`
- Test: `internal/store/store_test.go`

**Interfaces:**
- Consumes: nothing (operates on a directory path).
- Produces:
  - `type Store struct { Dir string }`
  - `func New(dir string) *Store`
  - `type LearnedProfile struct { AppMappings map[string]string; History []HistoryEntry }`
  - `type HistoryEntry struct { RecSummary string; Accepted bool; At time.Time }`
  - `func (s *Store) LoadLearned() (LearnedProfile, error)` — returns empty profile if file absent.
  - `func (s *Store) SaveLearned(LearnedProfile) error`
  - `func (s *Store) Export(w io.Writer) error` — writes learned.json content.
  - `func (s *Store) Delete() error` — removes learned.json.

- [ ] **Step 1: Write the failing test**

`internal/store/store_test.go`:
```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/`
Expected: FAIL — undefined identifiers.

- [ ] **Step 3: Write the store**

`internal/store/store.go`:
```go
// Package store persists user-owned state as plain, inspectable JSON files.
package store

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type HistoryEntry struct {
	RecSummary string    `json:"rec_summary"`
	Accepted   bool      `json:"accepted"`
	At         time.Time `json:"at"`
}

type LearnedProfile struct {
	AppMappings map[string]string `json:"app_mappings"`
	History     []HistoryEntry    `json:"history"`
}

type Store struct{ Dir string }

func New(dir string) *Store { return &Store{Dir: dir} }

func (s *Store) path() string { return filepath.Join(s.Dir, "learned.json") }

// LoadLearned returns the learned profile, or an empty (initialized) one if absent.
func (s *Store) LoadLearned() (LearnedProfile, error) {
	b, err := os.ReadFile(s.path())
	if os.IsNotExist(err) {
		return LearnedProfile{AppMappings: map[string]string{}}, nil
	}
	if err != nil {
		return LearnedProfile{}, fmt.Errorf("read learned: %w", err)
	}
	var lp LearnedProfile
	if err := json.Unmarshal(b, &lp); err != nil {
		return LearnedProfile{}, fmt.Errorf("parse learned: %w", err)
	}
	if lp.AppMappings == nil {
		lp.AppMappings = map[string]string{}
	}
	return lp, nil
}

func (s *Store) SaveLearned(lp LearnedProfile) error {
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return fmt.Errorf("mkdir profile: %w", err)
	}
	b, err := json.MarshalIndent(lp, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal learned: %w", err)
	}
	return os.WriteFile(s.path(), b, 0o600)
}

func (s *Store) Export(w io.Writer) error {
	b, err := os.ReadFile(s.path())
	if err != nil {
		return fmt.Errorf("read learned for export: %w", err)
	}
	_, err = w.Write(b)
	return err
}

func (s *Store) Delete() error {
	err := os.Remove(s.path())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/store/
git commit -m "feat: add plain-file store for user-owned state"
```

---

### Task 4: Linear GraphQL client (read-only)

**Files:**
- Create: `internal/linear/client.go`
- Test: `internal/linear/client_test.go`

**Interfaces:**
- Consumes: `config.Profile.LinearToken`.
- Produces:
  - `type Client struct { ... }`
  - `func New(endpoint, token string, httpClient *http.Client) *Client`
  - `func (c *Client) Issues(ctx context.Context, since time.Time) ([]model.Issue, error)`
  - `func (c *Client) Comments(ctx context.Context, since time.Time) ([]model.Comment, error)`
  - Note: only queries; no mutations.

- [ ] **Step 1: Add graphql dependency**

```bash
go get github.com/machinebox/graphql
```

- [ ] **Step 2: Write the failing test (httptest GraphQL server + fixture)**

`internal/linear/client_test.go`:
```go
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
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/linear/`
Expected: FAIL — `undefined: New`.

- [ ] **Step 4: Write the client**

`internal/linear/client.go`:
```go
// Package linear is a read-only Linear GraphQL client. It issues queries only.
package linear

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/machinebox/graphql"
	"github.com/hitsnorth/linear-scout/internal/model"
)

const DefaultEndpoint = "https://api.linear.app/graphql"

type Client struct {
	gql   *graphql.Client
	token string
}

func New(endpoint, token string, httpClient *http.Client) *Client {
	opts := []graphql.ClientOption{}
	if httpClient != nil {
		opts = append(opts, graphql.WithHTTPClient(httpClient))
	}
	return &Client{gql: graphql.NewClient(endpoint, opts...), token: token}
}

type issueNode struct {
	ID          string `json:"id"`
	Identifier  string `json:"identifier"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    int    `json:"priority"`
	URL         string `json:"url"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	State       struct{ Name string `json:"name"` } `json:"state"`
	Assignee    struct{ Name string `json:"name"` } `json:"assignee"`
	Project     struct{ ID string `json:"id"` } `json:"project"`
	Team        struct{ ID string `json:"id"` } `json:"team"`
	Labels      struct {
		Nodes []struct{ Name string `json:"name"` } `json:"nodes"`
	} `json:"labels"`
}

func (c *Client) run(ctx context.Context, query string, vars map[string]any, out any) error {
	req := graphql.NewRequest(query)
	req.Header.Set("Authorization", c.token)
	for k, v := range vars {
		req.Var(k, v)
	}
	if err := c.gql.Run(ctx, req, out); err != nil {
		return fmt.Errorf("linear query: %w", err)
	}
	return nil
}

const issuesQuery = `
query Issues($since: DateTimeOrDuration) {
  issues(filter: { updatedAt: { gte: $since } }, first: 100) {
    nodes {
      id identifier title description priority url createdAt updatedAt
      state { name } assignee { name } project { id } team { id }
      labels { nodes { name } }
    }
  }
}`

func (c *Client) Issues(ctx context.Context, since time.Time) ([]model.Issue, error) {
	var resp struct {
		Issues struct{ Nodes []issueNode `json:"nodes"` } `json:"issues"`
	}
	if err := c.run(ctx, issuesQuery, map[string]any{"since": since.Format(time.RFC3339)}, &resp); err != nil {
		return nil, err
	}
	out := make([]model.Issue, 0, len(resp.Issues.Nodes))
	for _, n := range resp.Issues.Nodes {
		labels := make([]string, 0, len(n.Labels.Nodes))
		for _, l := range n.Labels.Nodes {
			labels = append(labels, l.Name)
		}
		out = append(out, model.Issue{
			ID: n.ID, Identifier: n.Identifier, Title: n.Title, Body: n.Description,
			State: n.State.Name, Priority: n.Priority, Labels: labels,
			Assignee: n.Assignee.Name, ProjectID: n.Project.ID, TeamID: n.Team.ID,
			URL: n.URL, CreatedAt: n.CreatedAt, UpdatedAt: n.UpdatedAt,
		})
	}
	return out, nil
}

const commentsQuery = `
query Comments($since: DateTimeOrDuration) {
  comments(filter: { createdAt: { gte: $since } }, first: 100) {
    nodes { id body url createdAt user { name } issue { id } }
  }
}`

func (c *Client) Comments(ctx context.Context, since time.Time) ([]model.Comment, error) {
	var resp struct {
		Comments struct {
			Nodes []struct {
				ID        string    `json:"id"`
				Body      string    `json:"body"`
				URL       string    `json:"url"`
				CreatedAt time.Time `json:"createdAt"`
				User      struct{ Name string `json:"name"` } `json:"user"`
				Issue     struct{ ID string `json:"id"` } `json:"issue"`
			} `json:"nodes"`
		} `json:"comments"`
	}
	if err := c.run(ctx, commentsQuery, map[string]any{"since": since.Format(time.RFC3339)}, &resp); err != nil {
		return nil, err
	}
	out := make([]model.Comment, 0, len(resp.Comments.Nodes))
	for _, n := range resp.Comments.Nodes {
		out = append(out, model.Comment{
			ID: n.ID, IssueID: n.Issue.ID, Author: n.User.Name,
			Body: n.Body, URL: n.URL, CreatedAt: n.CreatedAt,
		})
	}
	return out, nil
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/linear/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/linear/ go.mod go.sum
git commit -m "feat: add read-only Linear GraphQL client"
```

---

### Task 5: Ingest — window parsing + normalized dataset

**Files:**
- Create: `internal/ingest/ingest.go`
- Test: `internal/ingest/ingest_test.go`

**Interfaces:**
- Consumes: the `Client` methods from Task 4 via a narrow interface.
- Produces:
  - `type Source interface { Issues(ctx, since) ([]model.Issue, error); Comments(ctx, since) ([]model.Comment, error) }`
  - `func ParseWindow(s string, now time.Time) (since time.Time, err error)` — accepts `7d`, `24h`, `2w`.
  - `func Fetch(ctx context.Context, src Source, window string, now time.Time) (model.Activity, error)`

- [ ] **Step 1: Write the failing test**

`internal/ingest/ingest_test.go`:
```go
package ingest

import (
	"context"
	"testing"
	"time"

	"github.com/hitsnorth/linear-scout/internal/model"
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ingest/`
Expected: FAIL — undefined identifiers.

- [ ] **Step 3: Write the ingest package**

`internal/ingest/ingest.go`:
```go
// Package ingest turns a time window into a normalized activity dataset.
package ingest

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/hitsnorth/linear-scout/internal/model"
)

type Source interface {
	Issues(ctx context.Context, since time.Time) ([]model.Issue, error)
	Comments(ctx context.Context, since time.Time) ([]model.Comment, error)
}

// ParseWindow accepts forms like "7d", "24h", "2w".
func ParseWindow(s string, now time.Time) (time.Time, error) {
	if len(s) < 2 {
		return time.Time{}, fmt.Errorf("invalid window %q", s)
	}
	unit := s[len(s)-1]
	n, err := strconv.Atoi(s[:len(s)-1])
	if err != nil || n <= 0 {
		return time.Time{}, fmt.Errorf("invalid window %q", s)
	}
	switch unit {
	case 'h':
		return now.Add(-time.Duration(n) * time.Hour), nil
	case 'd':
		return now.AddDate(0, 0, -n), nil
	case 'w':
		return now.AddDate(0, 0, -7*n), nil
	default:
		return time.Time{}, fmt.Errorf("invalid window unit in %q", s)
	}
}

// Fetch pulls issues and comments for the window and normalizes them.
func Fetch(ctx context.Context, src Source, window string, now time.Time) (model.Activity, error) {
	since, err := ParseWindow(window, now)
	if err != nil {
		return model.Activity{}, err
	}
	issues, err := src.Issues(ctx, since)
	if err != nil {
		return model.Activity{}, fmt.Errorf("fetch issues: %w", err)
	}
	comments, err := src.Comments(ctx, since)
	if err != nil {
		return model.Activity{}, fmt.Errorf("fetch comments: %w", err)
	}
	return model.Activity{
		Since: since, Until: now, Issues: issues, Comments: comments,
		Projects: map[string]string{}, Teams: map[string]string{},
	}, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ingest/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ingest/
git commit -m "feat: add window parsing and activity ingestion"
```

---

### Task 6: Grouping — classifier with confidence

**Files:**
- Create: `internal/grouping/grouping.go`
- Test: `internal/grouping/grouping_test.go`

**Interfaces:**
- Consumes: `model.Activity`, `store.LearnedProfile.AppMappings`.
- Produces:
  - `func Classify(act model.Activity, mappings map[string]string, groupBy string) ([]model.Group, int)` — returns groups + count of unclassified issues.

Rules: if a label matches a learned mapping (`label:<name>` → app), group as `app` with high confidence. Else group by the `groupBy` metadata field (project/team) with medium confidence. Issues with no project/team and no mapping go to a single `unclassified` group with low confidence.

- [ ] **Step 1: Write the failing test**

`internal/grouping/grouping_test.go`:
```go
package grouping

import (
	"testing"

	"github.com/hitsnorth/linear-scout/internal/model"
)

func TestClassifyByLearnedMapping(t *testing.T) {
	act := model.Activity{Issues: []model.Issue{
		{ID: "i1", Labels: []string{"bug"}, ProjectID: "p1"},
	}}
	groups, unclassified := Classify(act, map[string]string{"label:bug": "CoreApp"}, "project")
	if unclassified != 0 {
		t.Fatalf("unclassified=%d", unclassified)
	}
	if len(groups) != 1 || groups[0].Kind != "app" || groups[0].Label != "CoreApp" {
		t.Fatalf("bad group: %+v", groups)
	}
	if groups[0].Confidence.Band() != "high" {
		t.Fatalf("want high confidence, got %s", groups[0].Confidence.Band())
	}
}

func TestClassifyUnclassified(t *testing.T) {
	act := model.Activity{Issues: []model.Issue{{ID: "i2"}}}
	groups, unclassified := Classify(act, nil, "project")
	if unclassified != 1 {
		t.Fatalf("unclassified=%d", unclassified)
	}
	if len(groups) != 1 || groups[0].Kind != "unclassified" || groups[0].Confidence.Band() != "low" {
		t.Fatalf("bad group: %+v", groups)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/grouping/`
Expected: FAIL — `undefined: Classify`.

- [ ] **Step 3: Write the grouping package**

`internal/grouping/grouping.go`:
```go
// Package grouping infers app/product/project/team buckets with confidence.
package grouping

import "github.com/hitsnorth/linear-scout/internal/model"

// Classify buckets issues, returning groups and the count of unclassified issues.
func Classify(act model.Activity, mappings map[string]string, groupBy string) ([]model.Group, int) {
	byKey := map[string]*model.Group{}
	order := []string{}
	unclassified := 0

	ensure := func(key, label, kind string, conf model.Confidence) *model.Group {
		g, ok := byKey[key]
		if !ok {
			g = &model.Group{Key: key, Label: label, Kind: kind, Confidence: conf}
			byKey[key] = g
			order = append(order, key)
		}
		return g
	}

	for _, is := range act.Issues {
		if app, key := matchMapping(is.Labels, mappings); key != "" {
			g := ensure("app:"+app, app, "app", 0.9)
			g.IssueIDs = append(g.IssueIDs, is.ID)
			continue
		}
		var metaKey string
		switch groupBy {
		case "team":
			metaKey = is.TeamID
		default:
			metaKey = is.ProjectID
		}
		if metaKey == "" {
			g := ensure("unclassified", "Unclassified", "unclassified", 0.3)
			g.IssueIDs = append(g.IssueIDs, is.ID)
			unclassified++
			continue
		}
		g := ensure(groupBy+":"+metaKey, metaKey, groupBy, 0.6)
		g.IssueIDs = append(g.IssueIDs, is.ID)
	}

	out := make([]model.Group, 0, len(order))
	for _, k := range order {
		out = append(out, *byKey[k])
	}
	return out, unclassified
}

func matchMapping(labels []string, mappings map[string]string) (app, key string) {
	for _, l := range labels {
		k := "label:" + l
		if app, ok := mappings[k]; ok {
			return app, k
		}
	}
	return "", ""
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/grouping/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/grouping/
git commit -m "feat: add grouping classifier with confidence"
```

---

### Task 7: Evidence — bundle builder + ranking

**Files:**
- Create: `internal/evidence/evidence.go`
- Test: `internal/evidence/evidence_test.go`

**Interfaces:**
- Consumes: `model.Activity`, `model.Group`.
- Produces:
  - `func Build(act model.Activity, groups []model.Group) map[string]model.EvidenceBundle` — keyed by group key. Each bundle links every issue in the group (and comments on those issues) back to source, and scores by issue+comment count.

- [ ] **Step 1: Write the failing test**

`internal/evidence/evidence_test.go`:
```go
package evidence

import (
	"testing"

	"github.com/hitsnorth/linear-scout/internal/model"
)

func TestBuildLinksAndRanks(t *testing.T) {
	act := model.Activity{
		Issues: []model.Issue{
			{ID: "i1", Identifier: "ENG-1", URL: "https://l/ENG-1", Title: "Crash"},
		},
		Comments: []model.Comment{
			{ID: "c1", IssueID: "i1", URL: "https://l/c1", Body: "still broken"},
		},
	}
	groups := []model.Group{{Key: "project:p1", IssueIDs: []string{"i1"}}}
	bundles := Build(act, groups)
	b, ok := bundles["project:p1"]
	if !ok {
		t.Fatal("missing bundle")
	}
	if len(b.Links) != 2 {
		t.Fatalf("want 2 links (issue+comment), got %d", len(b.Links))
	}
	if b.Score <= 0 {
		t.Fatalf("want positive score, got %v", b.Score)
	}
	if b.Links[0].URL == "" || b.Links[0].Ref == "" {
		t.Fatalf("evidence link missing url/ref: %+v", b.Links[0])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/evidence/`
Expected: FAIL — `undefined: Build`.

- [ ] **Step 3: Write the evidence package**

`internal/evidence/evidence.go`:
```go
// Package evidence builds ranked, auditable evidence bundles per group.
// It sits BELOW the AI layer so every recommendation can be traced to source.
package evidence

import "github.com/hitsnorth/linear-scout/internal/model"

func Build(act model.Activity, groups []model.Group) map[string]model.EvidenceBundle {
	issueByID := map[string]model.Issue{}
	for _, is := range act.Issues {
		issueByID[is.ID] = is
	}
	commentsByIssue := map[string][]model.Comment{}
	for _, c := range act.Comments {
		commentsByIssue[c.IssueID] = append(commentsByIssue[c.IssueID], c)
	}

	out := map[string]model.EvidenceBundle{}
	for _, g := range groups {
		b := model.EvidenceBundle{GroupKey: g.Key}
		for _, id := range g.IssueIDs {
			is, ok := issueByID[id]
			if !ok {
				continue
			}
			b.Links = append(b.Links, model.EvidenceLink{
				Kind: "issue", Ref: is.Identifier, URL: is.URL, Quote: is.Title,
			})
			for _, c := range commentsByIssue[id] {
				b.Links = append(b.Links, model.EvidenceLink{
					Kind: "comment", Ref: is.Identifier, URL: c.URL, Quote: excerpt(c.Body),
				})
			}
		}
		b.Score = float64(len(b.Links))
		out[g.Key] = b
	}
	return out
}

func excerpt(s string) string {
	const max = 140
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/evidence/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/evidence/
git commit -m "feat: add evidence bundle builder and ranking"
```

---

### Task 8: AI — provider interface, prompt assembly, stub, validator

**Files:**
- Create: `internal/ai/provider.go`
- Create: `internal/ai/validate.go`
- Test: `internal/ai/ai_test.go`

**Interfaces:**
- Consumes: `model.Group`, `model.EvidenceBundle`, `config.RubricConfig`.
- Produces:
  - `type Request struct { Groups []model.Group; Evidence map[string]model.EvidenceBundle }`
  - `type Provider interface { Recommend(ctx context.Context, req Request) ([]model.Recommendation, error) }`
  - `func AssemblePrompt(req Request) string`
  - `func Validate(recs []model.Recommendation, rubric config.RubricConfig) []model.Recommendation` — drops recs below `MinConfidence` or (when `RequireEvidence`) with zero evidence links.
  - `type StubProvider struct { Recs []model.Recommendation }` implementing `Provider` (deterministic; for tests and offline runs).

- [ ] **Step 1: Write the failing test**

`internal/ai/ai_test.go`:
```go
package ai

import (
	"context"
	"strings"
	"testing"

	"github.com/hitsnorth/linear-scout/internal/config"
	"github.com/hitsnorth/linear-scout/internal/model"
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ai/`
Expected: FAIL — undefined identifiers.

- [ ] **Step 3: Write provider + prompt + stub**

`internal/ai/provider.go`:
```go
// Package ai defines the provider interface, prompt assembly, and validation.
// The evidence layer below it makes every recommendation auditable.
package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/hitsnorth/linear-scout/internal/model"
)

type Request struct {
	Groups   []model.Group
	Evidence map[string]model.EvidenceBundle
}

type Provider interface {
	Recommend(ctx context.Context, req Request) ([]model.Recommendation, error)
}

// AssemblePrompt renders a deterministic prompt from groups + evidence.
func AssemblePrompt(req Request) string {
	var b strings.Builder
	b.WriteString("You are linear-scout. Propose concrete product improvement ")
	b.WriteString("opportunities. Every recommendation MUST cite evidence refs below.\n\n")
	for _, g := range req.Groups {
		fmt.Fprintf(&b, "## Group %s (%s, confidence %s)\n", g.Label, g.Kind, g.Confidence.Band())
		for _, l := range req.Evidence[g.Key].Links {
			fmt.Fprintf(&b, "- [%s] %s — %s (%s)\n", l.Kind, l.Ref, l.Quote, l.URL)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// StubProvider is a deterministic Provider for tests and offline runs.
type StubProvider struct{ Recs []model.Recommendation }

func (s StubProvider) Recommend(context.Context, Request) ([]model.Recommendation, error) {
	return s.Recs, nil
}
```

`internal/ai/validate.go`:
```go
package ai

import (
	"github.com/hitsnorth/linear-scout/internal/config"
	"github.com/hitsnorth/linear-scout/internal/model"
)

// Validate enforces the rubric: minimum confidence and (optionally) evidence.
func Validate(recs []model.Recommendation, rubric config.RubricConfig) []model.Recommendation {
	out := make([]model.Recommendation, 0, len(recs))
	for _, r := range recs {
		if float64(r.Confidence) < rubric.MinConfidence {
			continue
		}
		if rubric.RequireEvidence && len(r.Evidence) == 0 {
			continue
		}
		out = append(out, r)
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ai/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ai/
git commit -m "feat: add AI provider interface, prompt assembly, validator, stub"
```

---

### Task 9: AI — OpenAI reference provider

**Files:**
- Create: `internal/ai/openai.go`
- Test: `internal/ai/openai_test.go`

**Interfaces:**
- Consumes: `Request`, produces `[]model.Recommendation`.
- Produces:
  - `type OpenAIProvider struct { client *openai.Client; model string }`
  - `func NewOpenAI(apiKey, model string, opts ...OpenAIOption) *OpenAIProvider`
  - `func WithBaseURL(url string) OpenAIOption` — lets tests point at a mock server.
  - `OpenAIProvider` implements `Provider`, requesting JSON output and unmarshalling into recommendations.

- [ ] **Step 1: Add openai dependency**

```bash
go get github.com/sashabaranov/go-openai
```

- [ ] **Step 2: Write the failing test (mock OpenAI HTTP server)**

`internal/ai/openai_test.go`:
```go
package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const chatResp = `{"choices":[{"message":{"role":"assistant","content":
"{\"recommendations\":[{\"summary\":\"Fix login crash\",\"why_it_matters\":\"blocks users\",\"confidence\":0.8,\"evidence\":[{\"ref\":\"ENG-1\",\"url\":\"https://l/ENG-1\"}]}]}"}}]}`

func TestOpenAIProviderParsesJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(chatResp))
	}))
	defer srv.Close()

	p := NewOpenAI("sk_test", "gpt-4o-mini", WithBaseURL(srv.URL))
	recs, err := p.Recommend(context.Background(), Request{})
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].Summary != "Fix login crash" {
		t.Fatalf("bad parse: %+v", recs)
	}
	if len(recs[0].Evidence) != 1 || recs[0].Evidence[0].Ref != "ENG-1" {
		t.Fatalf("evidence not parsed: %+v", recs[0])
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/ai/ -run TestOpenAI`
Expected: FAIL — `undefined: NewOpenAI`.

- [ ] **Step 4: Write the OpenAI provider**

`internal/ai/openai.go`:
```go
package ai

import (
	"context"
	"encoding/json"
	"fmt"

	openai "github.com/sashabaranov/go-openai"
	"github.com/hitsnorth/linear-scout/internal/model"
)

type OpenAIProvider struct {
	client *openai.Client
	model  string
}

type OpenAIOption func(*openai.ClientConfig)

func WithBaseURL(url string) OpenAIOption {
	return func(c *openai.ClientConfig) { c.BaseURL = url }
}

func NewOpenAI(apiKey, modelName string, opts ...OpenAIOption) *OpenAIProvider {
	cfg := openai.DefaultConfig(apiKey)
	for _, o := range opts {
		o(&cfg)
	}
	return &OpenAIProvider{client: openai.NewClientWithConfig(cfg), model: modelName}
}

type wireRec struct {
	Summary      string `json:"summary"`
	WhyItMatters string `json:"why_it_matters"`
	Confidence   float64 `json:"confidence"`
	Evidence     []struct {
		Ref string `json:"ref"`
		URL string `json:"url"`
	} `json:"evidence"`
	DraftTitle string `json:"draft_title"`
	DraftBody  string `json:"draft_body"`
}

func (p *OpenAIProvider) Recommend(ctx context.Context, req Request) ([]model.Recommendation, error) {
	prompt := AssemblePrompt(req)
	resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: p.model,
		ResponseFormat: &openai.ChatCompletionResponseFormat{Type: openai.ChatCompletionResponseFormatTypeJSONObject},
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: "Respond with a JSON object: {\"recommendations\":[...]}."},
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("openai chat: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("openai returned no choices")
	}
	var wire struct {
		Recommendations []wireRec `json:"recommendations"`
	}
	if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &wire); err != nil {
		return nil, fmt.Errorf("parse openai json: %w", err)
	}
	out := make([]model.Recommendation, 0, len(wire.Recommendations))
	for _, w := range wire.Recommendations {
		links := make([]model.EvidenceLink, 0, len(w.Evidence))
		for _, e := range w.Evidence {
			links = append(links, model.EvidenceLink{Kind: "issue", Ref: e.Ref, URL: e.URL})
		}
		out = append(out, model.Recommendation{
			Summary: w.Summary, WhyItMatters: w.WhyItMatters,
			Confidence: model.Confidence(w.Confidence), Evidence: links,
			DraftTitle: w.DraftTitle, DraftBody: w.DraftBody,
		})
	}
	return out, nil
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/ai/`
Expected: PASS (all ai tests).

- [ ] **Step 6: Commit**

```bash
git add internal/ai/ go.mod go.sum
git commit -m "feat: add OpenAI reference provider"
```

---

### Task 10: Engine — orchestration → Report

**Files:**
- Create: `internal/engine/engine.go`
- Test: `internal/engine/engine_test.go`

**Interfaces:**
- Consumes: `ingest.Source`, `ai.Provider`, `store.LearnedProfile`, `config.Workspace`.
- Produces:
  - `type Options struct { Window, GroupBy string; Limit int; Now time.Time; Mappings map[string]string; Rubric config.RubricConfig }`
  - `func Run(ctx context.Context, src ingest.Source, prov ai.Provider, opts Options) (model.Report, error)`

- [ ] **Step 1: Write the failing end-to-end test (mock stack)**

`internal/engine/engine_test.go`:
```go
package engine

import (
	"context"
	"testing"
	"time"

	"github.com/hitsnorth/linear-scout/internal/ai"
	"github.com/hitsnorth/linear-scout/internal/config"
	"github.com/hitsnorth/linear-scout/internal/model"
)

type fakeSrc struct{}

func (fakeSrc) Issues(context.Context, time.Time) ([]model.Issue, error) {
	return []model.Issue{{ID: "i1", Identifier: "ENG-1", URL: "https://l/ENG-1", ProjectID: "p1", Title: "Crash"}}, nil
}
func (fakeSrc) Comments(context.Context, time.Time) ([]model.Comment, error) { return nil, nil }

func TestRunProducesReport(t *testing.T) {
	prov := ai.StubProvider{Recs: []model.Recommendation{
		{Summary: "Fix crash", Confidence: 0.8, Evidence: []model.EvidenceLink{{Ref: "ENG-1"}}},
		{Summary: "weak", Confidence: 0.1, Evidence: []model.EvidenceLink{{Ref: "ENG-1"}}},
	}}
	rep, err := Run(context.Background(), fakeSrc{}, prov, Options{
		Window: "7d", GroupBy: "project", Limit: 10,
		Now:    time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC),
		Rubric: config.RubricConfig{MinConfidence: 0.5, RequireEvidence: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Recommendations) != 1 || rep.Recommendations[0].Summary != "Fix crash" {
		t.Fatalf("validator not applied: %+v", rep.Recommendations)
	}
	if len(rep.Groups) != 1 || rep.Window != "7d" {
		t.Fatalf("bad report: %+v", rep)
	}
}

func TestRunAppliesLimit(t *testing.T) {
	recs := []model.Recommendation{}
	for i := 0; i < 5; i++ {
		recs = append(recs, model.Recommendation{Summary: "r", Confidence: 0.9, Evidence: []model.EvidenceLink{{Ref: "ENG-1"}}})
	}
	rep, _ := Run(context.Background(), fakeSrc{}, ai.StubProvider{Recs: recs}, Options{
		Window: "7d", GroupBy: "project", Limit: 2,
		Now: time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC),
		Rubric: config.RubricConfig{MinConfidence: 0.5},
	})
	if len(rep.Recommendations) != 2 {
		t.Fatalf("limit not applied: got %d", len(rep.Recommendations))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/engine/`
Expected: FAIL — `undefined: Run`.

- [ ] **Step 3: Write the engine**

`internal/engine/engine.go`:
```go
// Package engine orchestrates the read-only pipeline and returns a Report.
// Every delivery surface calls Run; none duplicate this logic.
package engine

import (
	"context"
	"time"

	"github.com/hitsnorth/linear-scout/internal/ai"
	"github.com/hitsnorth/linear-scout/internal/config"
	"github.com/hitsnorth/linear-scout/internal/evidence"
	"github.com/hitsnorth/linear-scout/internal/grouping"
	"github.com/hitsnorth/linear-scout/internal/ingest"
	"github.com/hitsnorth/linear-scout/internal/model"
)

type Options struct {
	Window   string
	GroupBy  string
	Limit    int
	Now      time.Time
	Mappings map[string]string
	Rubric   config.RubricConfig
}

func Run(ctx context.Context, src ingest.Source, prov ai.Provider, opts Options) (model.Report, error) {
	act, err := ingest.Fetch(ctx, src, opts.Window, opts.Now)
	if err != nil {
		return model.Report{}, err
	}
	groups, unclassified := grouping.Classify(act, opts.Mappings, opts.GroupBy)
	bundles := evidence.Build(act, groups)

	recs, err := prov.Recommend(ctx, ai.Request{Groups: groups, Evidence: bundles})
	if err != nil {
		return model.Report{}, err
	}
	recs = ai.Validate(recs, opts.Rubric)
	if opts.Limit > 0 && len(recs) > opts.Limit {
		recs = recs[:opts.Limit]
	}
	return model.Report{
		GeneratedAt: opts.Now, Window: opts.Window, GroupBy: opts.GroupBy,
		Groups: groups, Recommendations: recs, Unclassified: unclassified,
	}, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/engine/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/engine/
git commit -m "feat: add engine orchestration producing Report"
```

---

### Task 11: Report renderers — Markdown / JSON / Telegram

**Files:**
- Create: `internal/report/render.go`
- Test: `internal/report/render_test.go`

**Interfaces:**
- Consumes: `model.Report`.
- Produces:
  - `func Markdown(r model.Report) string`
  - `func JSON(r model.Report) (string, error)`
  - `func Telegram(r model.Report) string` — concise, plain text, no Markdown tables.
  - `func Render(r model.Report, format string) (string, error)` — dispatch.

- [ ] **Step 1: Write the failing test**

`internal/report/render_test.go`:
```go
package report

import (
	"strings"
	"testing"
	"time"

	"github.com/hitsnorth/linear-scout/internal/model"
)

func sampleReport() model.Report {
	return model.Report{
		GeneratedAt: time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC),
		Window:      "7d", GroupBy: "project",
		Groups: []model.Group{{Label: "CoreApp", Kind: "app", Confidence: 0.9}},
		Recommendations: []model.Recommendation{{
			Summary: "Fix login crash", WhyItMatters: "blocks users", Confidence: 0.8,
			Evidence: []model.EvidenceLink{{Ref: "ENG-1", URL: "https://l/ENG-1"}},
		}},
	}
}

func TestMarkdownIncludesEvidenceLink(t *testing.T) {
	out := Markdown(sampleReport())
	if !strings.Contains(out, "Fix login crash") || !strings.Contains(out, "https://l/ENG-1") {
		t.Fatalf("markdown missing content:\n%s", out)
	}
}

func TestJSONRoundTrips(t *testing.T) {
	out, err := JSON(sampleReport())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "\"Summary\": \"Fix login crash\"") {
		t.Fatalf("json missing content:\n%s", out)
	}
}

func TestTelegramIsConcise(t *testing.T) {
	out := Telegram(sampleReport())
	if !strings.Contains(out, "Fix login crash") || strings.Contains(out, "|") {
		t.Fatalf("telegram not concise/plain:\n%s", out)
	}
}

func TestRenderDispatch(t *testing.T) {
	for _, f := range []string{"markdown", "json", "telegram"} {
		if _, err := Render(sampleReport(), f); err != nil {
			t.Fatalf("format %s: %v", f, err)
		}
	}
	if _, err := Render(sampleReport(), "bogus"); err == nil {
		t.Fatal("expected error for bogus format")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/report/`
Expected: FAIL — undefined identifiers.

- [ ] **Step 3: Write the renderers**

`internal/report/render.go`:
```go
// Package report renders the surface-agnostic Report model into output formats.
package report

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hitsnorth/linear-scout/internal/model"
)

func Markdown(r model.Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# linear-scout report (%s, grouped by %s)\n\n", r.Window, r.GroupBy)
	if r.Unclassified > 0 {
		fmt.Fprintf(&b, "_%d unclassified issue(s)._\n\n", r.Unclassified)
	}
	for _, rec := range r.Recommendations {
		fmt.Fprintf(&b, "## %s\n\n", rec.Summary)
		fmt.Fprintf(&b, "**Why it matters:** %s\n\n", rec.WhyItMatters)
		fmt.Fprintf(&b, "**Confidence:** %s\n\n", rec.Confidence.Band())
		if len(rec.Evidence) > 0 {
			b.WriteString("**Evidence:**\n")
			for _, e := range rec.Evidence {
				fmt.Fprintf(&b, "- [%s](%s)\n", e.Ref, e.URL)
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

func JSON(r model.Report) (string, error) {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", fmt.Errorf("render json: %w", err)
	}
	return string(b), nil
}

func Telegram(r model.Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "linear-scout — %s\n", r.Window)
	for _, rec := range r.Recommendations {
		fmt.Fprintf(&b, "\n• %s (%s)\n", rec.Summary, rec.Confidence.Band())
		fmt.Fprintf(&b, "  %s\n", rec.WhyItMatters)
		for _, e := range rec.Evidence {
			fmt.Fprintf(&b, "  ↳ %s %s\n", e.Ref, e.URL)
		}
	}
	return b.String()
}

func Render(r model.Report, format string) (string, error) {
	switch format {
	case "markdown", "md":
		return Markdown(r), nil
	case "json":
		return JSON(r)
	case "telegram":
		return Telegram(r), nil
	default:
		return "", fmt.Errorf("unknown format %q", format)
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/report/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/report/
git commit -m "feat: add Markdown/JSON/Telegram report renderers"
```

---

### Task 12: Drafts — draft issue metadata generation

**Files:**
- Create: `internal/drafts/drafts.go`
- Test: `internal/drafts/drafts_test.go`

**Interfaces:**
- Consumes: `model.Report`.
- Produces:
  - `type Draft struct { Title, Body string; Labels []string; Priority int; Evidence []model.EvidenceLink }`
  - `func FromReport(r model.Report) []Draft` — builds one draft per recommendation, falling back to the summary as title when `DraftTitle` is empty. Never writes to Linear.

- [ ] **Step 1: Write the failing test**

`internal/drafts/drafts_test.go`:
```go
package drafts

import (
	"testing"

	"github.com/hitsnorth/linear-scout/internal/model"
)

func TestFromReportBuildsDrafts(t *testing.T) {
	r := model.Report{Recommendations: []model.Recommendation{
		{Summary: "Fix crash", DraftBody: "steps", Evidence: []model.EvidenceLink{{Ref: "ENG-1"}}},
		{Summary: "Add retry", DraftTitle: "Add retry to uploader", DraftBody: "why"},
	}}
	ds := FromReport(r)
	if len(ds) != 2 {
		t.Fatalf("want 2 drafts, got %d", len(ds))
	}
	if ds[0].Title != "Fix crash" { // falls back to summary
		t.Fatalf("draft0 title=%q", ds[0].Title)
	}
	if ds[1].Title != "Add retry to uploader" {
		t.Fatalf("draft1 title=%q", ds[1].Title)
	}
	if len(ds[0].Evidence) != 1 {
		t.Fatalf("draft0 evidence lost")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/drafts/`
Expected: FAIL — `undefined: FromReport`.

- [ ] **Step 3: Write the drafts package**

`internal/drafts/drafts.go`:
```go
// Package drafts turns recommendations into reviewable Linear issue metadata.
// It NEVER writes to Linear; it only produces draft structs.
package drafts

import "github.com/hitsnorth/linear-scout/internal/model"

type Draft struct {
	Title    string
	Body     string
	Labels   []string
	Priority int
	Evidence []model.EvidenceLink
}

func FromReport(r model.Report) []Draft {
	out := make([]Draft, 0, len(r.Recommendations))
	for _, rec := range r.Recommendations {
		title := rec.DraftTitle
		if title == "" {
			title = rec.Summary
		}
		out = append(out, Draft{
			Title: title, Body: rec.DraftBody, Labels: rec.SuggestedLabels,
			Priority: rec.SuggestedPriority, Evidence: rec.Evidence,
		})
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/drafts/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/drafts/
git commit -m "feat: add draft issue metadata generation"
```

---

### Task 13: CLI — commands and wiring

**Files:**
- Create: `internal/cli/root.go`
- Create: `internal/cli/report.go`
- Create: `internal/cli/config_cmds.go`
- Create: `internal/cli/profile.go`
- Create: `cmd/linear-scout/main.go`
- Test: `internal/cli/cli_test.go`

**Interfaces:**
- Consumes: every package above.
- Produces:
  - `func NewRootCmd() *cobra.Command` wiring subcommands: `init`, `validate`, `report`, `create-drafts`, `preview`, `profile inspect|export|delete`.
  - `func Execute() error` for `main`.

Commands construct real providers from config, EXCEPT tests inject a stub via an unexported `providerFactory` seam.

- [ ] **Step 1: Add cobra dependency**

```bash
go get github.com/spf13/cobra
```

- [ ] **Step 2: Write the failing test (report command with stub provider)**

`internal/cli/cli_test.go`:
```go
package cli

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/hitsnorth/linear-scout/internal/ai"
	"github.com/hitsnorth/linear-scout/internal/ingest"
	"github.com/hitsnorth/linear-scout/internal/model"
)

type stubSrc struct{}

func (stubSrc) Issues(context.Context, time.Time) ([]model.Issue, error) {
	return []model.Issue{{ID: "i1", Identifier: "ENG-1", URL: "https://l/ENG-1", ProjectID: "p1", Title: "Crash"}}, nil
}
func (stubSrc) Comments(context.Context, time.Time) ([]model.Comment, error) { return nil, nil }

func TestReportCommandRendersMarkdown(t *testing.T) {
	root := NewRootCmd()
	// Inject deterministic source + provider.
	testDeps = &deps{
		source: stubSrc{},
		provider: ai.StubProvider{Recs: []model.Recommendation{
			{Summary: "Fix crash", Confidence: 0.9, Evidence: []model.EvidenceLink{{Ref: "ENG-1", URL: "https://l/ENG-1"}}},
		}},
		now: time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC),
	}
	defer func() { testDeps = nil }()

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"report", "--since", "7d", "--group-by", "project", "--format", "markdown"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("Fix crash")) {
		t.Fatalf("report output missing rec:\n%s", out.String())
	}
	_ = ingest.Source(stubSrc{}) // ensure interface satisfied
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/cli/`
Expected: FAIL — undefined identifiers.

- [ ] **Step 4: Write the root command and dependency seam**

`internal/cli/root.go`:
```go
// Package cli wires cobra commands as thin adapters over the engine.
package cli

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/hitsnorth/linear-scout/internal/ai"
	"github.com/hitsnorth/linear-scout/internal/ingest"
)

// deps lets tests inject a deterministic source/provider/clock.
type deps struct {
	source   ingest.Source
	provider ai.Provider
	now      time.Time
}

// testDeps, when non-nil, overrides real construction (tests only).
var testDeps *deps

func nowFn(d *deps) time.Time {
	if d != nil && !d.now.IsZero() {
		return d.now
	}
	return time.Now()
}

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "linear-scout",
		Short: "AI-first assistant that turns Linear activity into product opportunities",
	}
	root.AddCommand(newReportCmd())
	root.AddCommand(newInitCmd())
	root.AddCommand(newValidateCmd())
	root.AddCommand(newCreateDraftsCmd())
	root.AddCommand(newPreviewCmd())
	root.AddCommand(newProfileCmd())
	return root
}

func Execute() error {
	return NewRootCmd().Execute()
}
```

- [ ] **Step 5: Write the report + create-drafts + preview commands**

`internal/cli/report.go`:
```go
package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hitsnorth/linear-scout/internal/config"
	"github.com/hitsnorth/linear-scout/internal/drafts"
	"github.com/hitsnorth/linear-scout/internal/engine"
	"github.com/hitsnorth/linear-scout/internal/report"
)

func buildDeps(cmd *cobra.Command) (*deps, config.RubricConfig, error) {
	if testDeps != nil {
		return testDeps, config.RubricConfig{MinConfidence: 0.5, RequireEvidence: true}, nil
	}
	// Real construction: load profile, build Linear client + OpenAI provider.
	dir, err := config.ProfileDir()
	if err != nil {
		return nil, config.RubricConfig{}, err
	}
	prof, err := config.LoadProfile(dir)
	if err != nil {
		return nil, config.RubricConfig{}, fmt.Errorf("load profile (run `linear-scout init`): %w", err)
	}
	return realDeps(prof), config.RubricConfig{MinConfidence: 0.5, RequireEvidence: true}, nil
}

func newReportCmd() *cobra.Command {
	var since, groupBy, format string
	var limit int
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate an AI recommendation report for a time window",
		RunE: func(cmd *cobra.Command, _ []string) error {
			d, rubric, err := buildDeps(cmd)
			if err != nil {
				return err
			}
			rep, err := engine.Run(context.Background(), d.source, d.provider, engine.Options{
				Window: since, GroupBy: groupBy, Limit: limit, Now: nowFn(d), Rubric: rubric,
			})
			if err != nil {
				return err
			}
			out, err := report.Render(rep, format)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), out)
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "7d", "time window, e.g. 7d, 24h, 2w")
	cmd.Flags().StringVar(&groupBy, "group-by", "project", "grouping mode: project|team")
	cmd.Flags().StringVar(&format, "format", "markdown", "output format: markdown|json|telegram")
	cmd.Flags().IntVar(&limit, "limit", 0, "max recommendations (0 = unlimited)")
	return cmd
}

func newCreateDraftsCmd() *cobra.Command {
	var since, groupBy string
	cmd := &cobra.Command{
		Use:   "create-drafts",
		Short: "Generate reviewable draft issue metadata (no writes)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			d, rubric, err := buildDeps(cmd)
			if err != nil {
				return err
			}
			rep, err := engine.Run(context.Background(), d.source, d.provider, engine.Options{
				Window: since, GroupBy: groupBy, Now: nowFn(d), Rubric: rubric,
			})
			if err != nil {
				return err
			}
			for i, dr := range drafts.FromReport(rep) {
				fmt.Fprintf(cmd.OutOrStdout(), "Draft %d: %s\n  %s\n", i+1, dr.Title, dr.Body)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "7d", "time window")
	cmd.Flags().StringVar(&groupBy, "group-by", "project", "grouping mode")
	return cmd
}

func newPreviewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "preview",
		Short: "Preview Linear writes (dry-run). Writes are implemented in Milestone 2.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "[dry-run] No write actions available yet (Milestone 2). Nothing will change in Linear.")
			return nil
		},
	}
}
```

- [ ] **Step 6: Write init/validate + real dependency construction**

`internal/cli/config_cmds.go`:
```go
package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/hitsnorth/linear-scout/internal/ai"
	"github.com/hitsnorth/linear-scout/internal/config"
	"github.com/hitsnorth/linear-scout/internal/linear"
)

func realDeps(prof config.Profile) *deps {
	src := linear.New(linear.DefaultEndpoint, prof.LinearToken, http.DefaultClient)
	prov := ai.NewOpenAI(prof.OpenAIKey, "gpt-4o-mini")
	return &deps{source: src, provider: prov}
}

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize configuration (creates profile dir + template files)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir, err := config.ProfileDir()
			if err != nil {
				return err
			}
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return err
			}
			pp := filepath.Join(dir, "profile.yaml")
			if _, err := os.Stat(pp); os.IsNotExist(err) {
				os.WriteFile(pp, []byte("linear_token: \"\"\nopenai_key: \"\"\n"), 0o600)
			}
			if _, err := os.Stat("linear-scout.yaml"); os.IsNotExist(err) {
				os.WriteFile("linear-scout.yaml",
					[]byte("group_by: project\nformats: [markdown, json, telegram]\nrubric:\n  min_confidence: 0.5\n  require_evidence: true\n"), 0o644)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Initialized. Edit secrets in %s and defaults in ./linear-scout.yaml\n", pp)
			return nil
		},
	}
}

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate Linear and provider credentials",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir, err := config.ProfileDir()
			if err != nil {
				return err
			}
			prof, err := config.LoadProfile(dir)
			if err != nil {
				return fmt.Errorf("load profile: %w", err)
			}
			if prof.LinearToken == "" {
				return fmt.Errorf("linear_token is empty in %s", dir)
			}
			c := linear.New(linear.DefaultEndpoint, prof.LinearToken, http.DefaultClient)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if _, err := c.Issues(ctx, time.Now().Add(-time.Hour)); err != nil {
				return fmt.Errorf("linear credential check failed: %w", err)
			}
			if prof.OpenAIKey == "" {
				return fmt.Errorf("openai_key is empty in %s", dir)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Credentials OK.")
			return nil
		},
	}
}
```

- [ ] **Step 7: Write profile commands**

`internal/cli/profile.go`:
```go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hitsnorth/linear-scout/internal/config"
	"github.com/hitsnorth/linear-scout/internal/store"
)

func newProfileCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "profile", Short: "Inspect, export, or delete the local learning profile"}

	inspect := &cobra.Command{
		Use:   "inspect",
		Short: "Show learned profile summary",
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := profileStore()
			if err != nil {
				return err
			}
			lp, err := s.LoadLearned()
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "App mappings: %d\nHistory entries: %d\n", len(lp.AppMappings), len(lp.History))
			return nil
		},
	}
	export := &cobra.Command{
		Use:   "export",
		Short: "Export learned profile JSON to stdout",
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := profileStore()
			if err != nil {
				return err
			}
			return s.Export(cmd.OutOrStdout())
		},
	}
	del := &cobra.Command{
		Use:   "delete",
		Short: "Delete the learned profile",
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := profileStore()
			if err != nil {
				return err
			}
			if err := s.Delete(); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Deleted learned profile.")
			return nil
		},
	}
	cmd.AddCommand(inspect, export, del)
	return cmd
}

func profileStore() (*store.Store, error) {
	dir, err := config.ProfileDir()
	if err != nil {
		return nil, err
	}
	return store.New(dir), nil
}
```

- [ ] **Step 8: Write main**

`cmd/linear-scout/main.go`:
```go
package main

import (
	"fmt"
	"os"

	"github.com/hitsnorth/linear-scout/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 9: Run tests + build**

Run: `go test ./... && go build ./cmd/linear-scout`
Expected: PASS; binary builds.

- [ ] **Step 10: Commit**

```bash
git add internal/cli/ cmd/ go.mod go.sum
git commit -m "feat: add CLI commands and wiring"
```

---

### Task 14: Docs + example config

**Files:**
- Create: `README.md`
- Create: `docs/provider-data-sharing.md`
- Create: `linear-scout.example.yaml`

**Interfaces:** none (documentation).

- [ ] **Step 1: Write README**

`README.md` must cover: what linear-scout does; install (`go install github.com/hitsnorth/linear-scout/cmd/linear-scout@latest`); `init` → edit `profile.yaml` (Linear personal API key from Linear Settings → API, OpenAI key); `validate`; `report --since 7d --group-by project`; the read-only-by-default safety model; and a pointer to `docs/provider-data-sharing.md`.

- [ ] **Step 2: Write provider data-sharing doc**

`docs/provider-data-sharing.md` must state: Linear issue/comment text and metadata for the selected window are sent to the configured AI provider (OpenAI by default); no data is sent anywhere on read unless a report is generated; secrets never leave the local profile dir; and which Linear permission scope the personal API key needs (read).

- [ ] **Step 3: Write example workspace config**

`linear-scout.example.yaml`:
```yaml
group_by: project
formats: [markdown, json, telegram]
rubric:
  min_confidence: 0.5
  require_evidence: true
```

- [ ] **Step 4: Verify build + full test suite once more**

Run: `go vet ./... && go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add README.md docs/provider-data-sharing.md linear-scout.example.yaml
git commit -m "docs: add README, provider data-sharing notes, example config"
```

---

## Self-Review

**Spec coverage (SRS → task):**
- §6.1 CLI (init/validate/report/drafts/preview/profile) → Task 13; learn commands deferred to M3 (out of scope, noted).
- §6.2 Linear ingestion (issues/comments/status/labels/assignees/projects/teams/timestamps/links) → Tasks 4–5. *Status changes* are not separately fetched in M1; issue state + updatedAt approximate recent motion. Noted as an M1 limitation; full status-change history can be added when needed.
- §6.3 AI-first engine (provider iface, OpenAI, prompt assembly, evidence builder, validator, confidence, structured model) → Tasks 7–10.
- §6.4 Grouping with confidence + unclassified + learned profile + corrections → Task 6 (corrections recording is M3; classification consumes existing mappings now).
- §6.5 Learning/autoresearch → deferred to M3 (store scaffolding for it is in Task 3).
- §6.6 Settings split (shared vs user-local) → Task 2.
- §6.7 Rubric fields → model + validator (Tasks 1, 8); full field population depends on provider output.
- §6.8 Outputs (MD/JSON/Telegram) → Task 11.
- Acceptance criteria → covered across Tasks 2 (secrets), 13 (one command), 7–10 (auditable recs), 6 (messy identity), 8 (configurable rubric), 11 (three formats), 12 (drafts, no writes), 13 (preview dry-run), 3+13 (inspect/export/delete), 10 (reusable engine).

**Placeholder scan:** No TBD/TODO in task steps. The `preview` command intentionally prints a dry-run notice (M2 reserves real writes) — this is a real deliverable, not a placeholder.

**Type consistency:** `ingest.Source` (Tasks 5,10,13), `ai.Provider`/`ai.Request`/`ai.StubProvider`/`ai.Validate` (Tasks 8,9,10), `engine.Options`/`engine.Run` (Tasks 10,13), `config.RubricConfig` (Tasks 2,8,10,13), `model.*` (all) are used with consistent names and signatures across tasks.

**Known M1 limitations (intentional, documented):** status-change history is approximated; user-correction recording and learning loops are M3; report `Projects`/`Teams` name maps are left empty (ids used as labels) until a metadata fetch is added.
