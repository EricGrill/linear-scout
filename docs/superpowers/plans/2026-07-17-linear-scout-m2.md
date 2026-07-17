# linear-scout Milestone 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Make recommendations actionable through explicit, previewed Linear write commands while preserving the read-only-by-default safety model.

**Architecture:** A `write` package mirrors the read `engine` seam: build a `Plan` (no side effects) → preview → confirm → `Execute` behind a `Writer` interface, recording every change to an append-only audit log. Mutations live in a dedicated `internal/linear/writes.go` so the read surface stays unambiguous. CLI commands are thin adapters that funnel through one `runPlans` path.

**Tech Stack:** Go 1.22+, cobra, Linear GraphQL mutations (`issueCreate`, `commentCreate`, `issueUpdate`).

## Global Constraints

- Read-only is the default: no command writes without an explicit `--execute` flag.
- Every write path: dry-run preview → confirmation (`--yes` bypass) → change summary + evidence → audit-log entry.
- Never implemented: auto-close, reprioritize existing issues, delete issues/comments, broad content mutation.
- Secrets remain only in the user-local profile.
- Every package gets table-driven tests; Linear is faked, never called over the network in tests.

---

### Task 1: Linear write client (CHA-2421)

**Files:** Create `internal/linear/writes.go`, `internal/linear/writes_test.go`.

**Interfaces:**
- `CreateIssue(ctx, CreateIssueInput) (CreatedIssue, error)` → `issueCreate`.
- `CreateComment(ctx, issueID, body) (CreatedComment, error)` → `commentCreate`.
- `AddLabels(ctx, issueID, labelNames) error` → resolve names against the issue's team labels, merge with existing label ids, `issueUpdate`.

- [ ] Write httptest-backed tests asserting each mutation's request shape and response parsing (including label-merge and unknown-label error).
- [ ] Implement the three methods; return an error when Linear reports `success:false`.
- [ ] Verify no close/reprioritize/delete method exists.
- [ ] `go test ./internal/linear/`; commit.

### Task 2: Append-only write audit log (CHA-2422)

**Files:** Create `internal/store/audit.go`, `internal/store/audit_test.go`.

**Interfaces:**
- `AuditEntry{At, Action, Target, Summary, Evidence}`.
- `(*Store) AppendAudit(AuditEntry) error` — appends one JSON line to `audit.log`.
- `(*Store) ReadAudit() ([]AuditEntry, error)` — empty slice when absent.

- [ ] Round-trip test (append two, read two, verify order + fields).
- [ ] Implement with `O_APPEND|O_CREATE`, mode 0600.
- [ ] `go test ./internal/store/`; commit.

### Task 3: Write engine — plan + executor (CHA-2423)

**Files:** Create `internal/write/write.go`, `internal/write/write_test.go`.

**Interfaces:**
- `Plan{Action, Target, Summary, Evidence, Issue, Body, Labels}`; `Action ∈ {create-issue, comment, add-labels}`.
- `Writer` interface abstracting the three Linear mutations (`*linear.Client` satisfies it — assert at compile time).
- `AuditSink` interface (`*store.Store` satisfies it).
- `BuildIssuePlans(drafts, teamID) []Plan` — pure.
- `RenderPlans([]Plan) string` — dry-run preview text.
- `Execute(ctx, Writer, AuditSink, []Plan, now) ([]Result, error)` — performs each plan, audits successes, continues past per-plan failures.

- [ ] Tests: pure plan building; execute drives a stub writer + in-memory audit; failure of one plan does not stop the rest and is not audited.
- [ ] Implement; commit.

### Task 4: CLI write commands + real preview (CHA-2424)

**Files:** Create `internal/cli/write_cmds.go`; modify `internal/cli/root.go` (register commands, add `writer`/`audit` to `deps`), `internal/cli/config_cmds.go` (`realDeps` wires the client as writer + store as audit), remove the M1 `preview` stub from `report.go`.

**Commands:**
- `create-issues --team <id> [--since --group-by] [--execute] [--yes]`.
- `comment --issue <id> --body <text> [--execute] [--yes]`.
- `label --issue <id> --labels a,b [--execute] [--yes]`.
- `preview` — dry-run of `create-issues`, never writes.

- [ ] `confirm()` helper (prompt unless `--yes`); `runPlans()` single write path.
- [ ] CLI tests via injected stub writer: dry-run writes nothing; `--execute --yes` writes and audits; `create-issues` errors without `--team`; comment executes.
- [ ] `go test ./...`, `go vet ./...`, build; commit.

### Task 5: Plan doc + docs (CHA-2425)

**Files:** This document; update `README.md` and `docs/provider-data-sharing.md`.

- [ ] README: write commands, dry-run/confirm/audit model, the Linear key now needs write scope, never-allowed actions.
- [ ] provider-data-sharing: write scope note.
- [ ] Commit.

---

## Self-Review

- **Spec coverage (SRS §5, §6.1 write areas, M2):** create issues from drafts (Task 1,3,4), labels/comments (Task 1,4), preview + confirmation (Task 4), write audit log (Task 2), never-allowed actions omitted by construction (Task 1).
- **Placeholder scan:** none.
- **Type consistency:** `Writer`/`AuditSink`/`Plan`/`Result` names are consistent across write, cli, and linear; compile-time `var _ Writer = (*linear.Client)(nil)` enforces the client contract.

## Known M2 limitations (intentional)

- `create-issues` sets title/description/priority/team; suggested labels from drafts are added via the separate `label` command rather than on create (keeps create-input simple; label resolution needs team context).
- `--team` is a required flag; a workspace-config default team can be added later.
