# linear-scout — Design Spec

**Date:** 2026-07-16
**Source requirements:** `Linear Scout SRS.md`
**Scope:** All four milestones (M1–M4). M1 is the detailed near-term deliverable; the architecture is designed so M2–M4 slot in without rework.

## 1. Summary

`linear-scout` is an open-source, AI-first Go tool that reads recent Linear activity and turns it into evidence-backed product improvement recommendations. It is CLI-first, read-only by default, and reuses a single surface-agnostic engine across the CLI, a GitHub Action, a scheduled daemon, and Telegram delivery.

### Foundational decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Language / runtime | **Go** | Single static binary suits CLI + daemon + easy install; Action wraps the released binary. |
| Plan scope | **All four milestones** | Phased: M1 detailed, M2–M4 lighter but architecturally reserved. |
| Local state storage | **Plain files (JSON/YAML)** | Maximally transparent; "inspect / export / delete" ≈ read / copy / remove, matching the SRS user-owned-state ethos. |
| AI provider | **Interface + OpenAI reference impl** | Required by SRS §6.3; provider swappable, stubbed in tests. |
| Linear access | **GraphQL API + personal API key** | No official Go SDK; raw GraphQL client. Key lives only in user-local profile. |

## 2. Architecture

Layered monorepo (single Go module) with a surface-agnostic **engine** as the reuse seam. Interfaces are introduced **only** where the SRS demands variation: AI provider, storage, report renderer, and ingestion source.

```
cmd/linear-scout/         main; wires CLI
internal/
  cli/          cobra commands (thin adapters over engine)
  config/       shared workspace config + user-local profile; load & merge
  model/        domain types: Activity, Issue, Comment, Recommendation,
                EvidenceBundle, Group, Confidence, Report
  linear/       Linear GraphQL client (read-only) + auth
  ingest/       fetch a time window → normalized activity dataset
  grouping/     classifier: infer app/product/project/team + confidence + unclassified
  evidence/     evidence bundle builder + ranking (auditable layer BELOW ai)
  ai/           Provider interface, OpenAI reference impl, prompt assembly,
                recommendation validator, confidence/uncertainty
  engine/       orchestrates the pipeline; surface-agnostic; returns Report
  report/       Report model → Markdown / JSON / Telegram renderers
  drafts/       draft Linear issue metadata generation
  write/        [M2] preview + explicit writes + audit log
  learn/        [M3] artifact-gated missions, evaluators, profile updates
  store/        plain-file storage for user-owned state (profile, corrections,
                history, audit)
surfaces/
  action/       [M4] GitHub Action wrapper
  daemon/       [M4] scheduler (robfig/cron)
  telegram/     [M4] delivery
```

**Reuse seam:** `engine.Run(ctx, opts) (Report, error)`. Every surface (CLI, Action, daemon, Telegram) calls this one entry point and renders the same `Report` model. This satisfies the SRS acceptance criterion that non-CLI surfaces reuse the core without duplicating recommendation logic.

**Safety invariant:** only `internal/write` may mutate Linear. It is unreachable on the default read path and is gated by an explicit command + dry-run preview + audit-log entry. Auto-close, reprioritize, delete, and broad content mutation are **not implemented** in any milestone covered here.

### Approaches considered

- **A — Layered monorepo with surface-agnostic engine (chosen).** Directly meets the reuse requirement; core is testable in isolation.
- **B — Interface-everywhere plugin architecture.** Rejected as over-abstraction (YAGNI) that would slow M1. Interfaces are used only at the four variation points above.
- **C — Script-first CLI, refactor later.** Rejected: violates the reusable-core requirement and forces M4 rework.

## 3. Data flow (read path, no writes)

1. `config` loads shared workspace config + user-local profile (merged).
2. `linear.Client` authenticates with the personal API key from the user-local profile.
3. `ingest` fetches the requested time window: issues, comments, status changes, labels, assignees, projects, teams, timestamps, and source URLs → normalized activity dataset.
4. `grouping` classifies activity into app/product/project/team buckets, using Linear metadata + local learned profile, attaching a confidence score; ambiguous items are marked low-confidence or unclassified rather than failing.
5. `evidence` builds ranked evidence bundles; every bundle carries links back to source Linear records.
6. `ai.Provider` receives the assembled prompt + evidence bundle and returns structured recommendations with confidence/uncertainty.
7. `validator` checks each recommendation against the configurable rubric, dropping or flagging weak ones.
8. `engine` assembles the `Report`; `report` renderers emit Markdown / JSON / Telegram from the same model; `drafts` emits reviewable draft issue metadata.

No Linear writes occur on this path.

## 4. Configuration & storage

**Shared workspace config** (committed, e.g. `linear-scout.yaml`) — team-shared defaults, grouping preferences, report formats, recommendation rubric defaults, safe-write policy, output templates. Contains **no secrets**.

**User-local profile** (`$XDG_CONFIG_HOME/linear-scout/`, falling back to `~/.config/linear-scout/`; plain files) — API tokens / provider credentials, learned app mappings, private corrections, accepted/rejected recommendation history, personal delivery preferences, and the write audit log. Each concern is a separate human-readable file behind a `store` interface. `inspect` / `export` / `delete` are backed by plain file read / copy / remove plus convenience CLI commands.

**Recommendation rubric** (configurable, default fields): opportunity summary; why it matters; source Linear evidence links; confidence; affected app/product/project/team; suggested owner/team when inferable; draft issue title and body; suggested labels and priority; duplicate-risk explanation.

## 5. AI layer

- `Provider` interface exists from day one; OpenAI is the reference implementation (`openai-go`).
- Components: prompt/input assembly, evidence bundle builder (auditable, sits **below** the AI layer), recommendation validator, confidence and uncertainty reporting, and a stable structured recommendation model.
- The provider is swappable with a deterministic stub for tests.

## 6. Error handling

- Typed errors throughout.
- A `validate` command checks Linear credentials and provider credentials before real runs.
- Messy Linear metadata degrades gracefully: grouping emits low-confidence/unclassified instead of erroring.
- AI failures yield a partial report with uncertainty noted rather than aborting.
- Backoff/retry on Linear and OpenAI rate limits.

## 7. Testing strategy

- Table-driven unit tests per package.
- Linear faked via an `httptest` GraphQL server backed by recorded fixtures; AI faked via the deterministic stub provider.
- Golden-file tests for the three report renderers.
- Evidence builder and rubric validator tested against fixtures.
- One end-to-end engine test exercising ingest→group→evidence→AI→report through the mock stack.

## 8. CLI surface

Canonical interface (cobra). Command areas from SRS §6.1:

- `init` — initialize configuration.
- `validate` — validate Linear + provider credentials.
- `report --since <window> --group-by <mode> --limit <n>` — generate a report.
- `create-drafts` — generate draft issue metadata (from a report).
- `preview` — preview Linear writes (dry-run). [wires to M2]
- write-execution commands — explicit, previewed writes. [M2]
- `profile inspect | export | delete` — manage local learning profile.
- `learn run | inspect` — run/inspect learning artifacts. [M3]

Example shapes:

```bash
linear-scout report --since 7d --group-by project --limit 2
linear-scout create-drafts --project BJJChat --from report.md
```

## 9. Milestone phasing

**M1 — CLI Report + Draft Recommendations.** Full read→group→evidence→AI→report→draft pipeline; `init`/`validate`/`report`/`create-drafts`/`profile` CLI; AI provider interface + OpenAI; configurable rubric; MD/JSON/Telegram renderers; plain-file profile basics; dry-run preview stubs for write candidates.

**M2 — Explicit Linear Write Commands.** `write` package: create issues from drafts, add labels/comments via explicit commands, preview + confirmation workflow, write audit log.

**M3 — Learning & Autoresearch Loops.** `learn` package: artifact-gated missions each with a mission + evaluator + output artifact; accepted updates written to local profile; correction feedback loop; recommendation acceptance/rejection tracking.

**M4 — Automation Surfaces.** GitHub Action, scheduled daemon, Telegram delivery — all calling `engine.Run` and reusing the same config, report model, AI provider interface, and safety checks as the CLI.

## 10. Acceptance criteria mapping

| SRS acceptance criterion | Where satisfied |
|--------------------------|-----------------|
| Configure Linear creds without secrets in shared config | §4 config split |
| One CLI command → useful report | §8 `report`, §3 flow |
| AI-generated, evidence-backed, auditable recs | §5 AI + §3 evidence layer below AI |
| Every rec links to Linear source evidence | §3 step 5, §4 rubric |
| Messy identity handled via AI classification + confidence + local learning | §3 step 4, grouping |
| Recommendation quality rules configurable | §4 rubric |
| MD / JSON / Telegram from one report model | §2 renderers |
| Draft metadata without writing to Linear by default | §3 drafts, safety invariant |
| Any write requires explicit command + dry-run | §2 safety invariant, M2 |
| Local profile inspect / export / delete | §4 store, §8 `profile` |
| Core engine reusable by non-CLI surfaces | §2 reuse seam, M4 |

## 11. Residual risks (from SRS §9)

- Broad full-scope ambition — mitigated by strict milestone boundaries in the plan.
- AI quality depends on prompt design, evidence selection, provider behavior, evaluator coverage — mitigated by the auditable evidence layer, validator, and M3 evaluators.
- Local learning can drift — corrections are visible (plain files) and reversible.
- Open-source users need clear docs on provider data sharing and Linear permissions — a docs deliverable accompanies M1.
