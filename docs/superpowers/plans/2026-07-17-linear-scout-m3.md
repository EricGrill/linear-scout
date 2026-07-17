# linear-scout Milestone 3 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Let linear-scout improve over time by learning how a team uses Linear — inferring label→app mappings from activity, and capturing user corrections and recommendation accept/reject feedback — all in user-owned, reversible local state.

**Architecture:** An artifact-gated `learn` package (SRS §6.5): each pass = a `Mission` (proposes `Candidate` profile updates) + an `Evaluator` (gates them). Passes are pure; applying accepted candidates to the profile is an explicit, separate step. Accepted learning, corrections, and decisions all persist through the plain-file `store`. Learned mappings flow back into grouping on the next run (via the mapping wiring added in M1 hardening, CHA-2401).

**Tech Stack:** Go 1.22+, cobra. No AI dependency — the reference mission is a deterministic heuristic, which keeps learning auditable and testable.

## Global Constraints

- Learning is artifact-gated: mission → evaluator → output artifact; only accepted candidates are applied.
- `learn run` is dry-run by default; `--apply` is required to write.
- All learned state is plain JSON in the profile dir — inspectable and reversible (`profile inspect/export/delete`).
- Local, non-Linear operations (`correct`, `feedback`, `learn inspect`) never require provider credentials.
- Table-driven tests; no network.

---

### Task 1: learn seam (CHA-2426)

**Files:** `internal/learn/learn.go`, `internal/learn/learn_test.go`.

- `Candidate{Key, App, Support, Purity}`, `Artifact{Mission, Candidates, Rationale}`, `Input{Activity, Groups, Existing}`.
- `Mission{Name(); Run(Input) Artifact}`, `Evaluator{Evaluate(Artifact) ([]Candidate, string)}`.
- `RunPass(m, e, in) (Artifact, []Candidate, string)` — pure.
- `Mappings([]Candidate) map[string]string` — accepted → mappings delta.
- [ ] Tests with a fake mission + threshold evaluator. Commit.

### Task 2: LabelMappingMission + PurityEvaluator (CHA-2427)

**Files:** `internal/learn/label_mapping.go`, `internal/learn/label_mapping_test.go`.

- Mission builds issue→group (skipping unclassified), tallies each label's group distribution, proposes `label:<name> → <top group>` with support = top count and purity = topCount/total; skips already-known mappings.
- `PurityEvaluator{MinSupport, MinPurity}` accepts candidates clearing both bars.
- [ ] Tests: clustered label accepted; spread label rejected (low purity); below-support rejected; known/unclassified skipped. Commit.

### Task 3: store learned-ops (CHA-2428)

**Files:** `internal/store/learned_ops.go`, `internal/store/learned_ops_test.go`.

- `MergeMappings(delta) error` — load, merge (preserving existing), save.
- `RecordDecision(HistoryEntry) error` — append to history, save.
- [ ] Round-trip tests including merge-on-empty. Commit.

### Task 4: CLI (CHA-2429)

**Files:** `internal/cli/learn_cmds.go`, `internal/cli/learn_cmds_test.go`; modify `root.go` (register commands, add `profileStore` to `deps`).

- `learn run [--since --group-by --min-support --min-purity] [--apply]` — ingest → classify → RunPass → print artifact; `--apply` merges accepted mappings.
- `learn inspect` — mappings + accepted/rejected counts.
- `correct --label --app` — record a label→app correction.
- `feedback --rec --accept|--reject` — record a decision (exactly one flag required).
- `storeFor()` returns the injected store under test, else one at the profile dir — local commands need no credentials.
- [ ] CLI tests via injected deps: correct writes a mapping; feedback records a decision and requires exactly one flag; `learn run` dry-run proposes without writing, `--apply` writes; `learn inspect` shows counts. Commit.

### Task 5: docs (CHA-2430)

**Files:** this doc; `README.md`.

- [ ] README: learning commands, correction/feedback loop, and how learned mappings feed grouping. Commit.

---

## Self-Review

- **Spec coverage (SRS §6.5, M3):** artifact-gated missions (Tasks 1–2), evaluator-backed outputs (Task 2), local profile updates (Task 3), correction feedback loop (Task 4 `correct`), accept/reject tracking (Task 4 `feedback` + Task 3 history). "Infer label-to-product mappings" is the reference mission; "learn which teams own which apps" reuses the same mission with `--group-by team`.
- **Placeholder scan:** none.
- **Type consistency:** `learn.Candidate/Artifact/Input/Mission/Evaluator/RunPass/Mappings`, `store.MergeMappings/RecordDecision/HistoryEntry` used consistently across learn, store, and cli; compile-time asserts for `Mission`/`Evaluator`.

## Known M3 limitations (intentional, documented)

- "Detect duplicate recommendation patterns": decisions are recorded but not yet used to suppress/flag duplicate recommendations at generation time — a natural next step (set `Recommendation.DuplicateRisk` from rejected history in the engine).
- The reference mission is heuristic; an AI-backed mission can be added behind the same `Mission` interface.
- Learned mappings label groups by app name; group keys remain stable IDs.
