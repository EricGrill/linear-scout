# Software Requirements Specification: linear-scout

## 1. Overview

`linear-scout` is an open-source, AI-first assistant that reviews recent Linear activity and turns noisy project motion into evidence-backed product improvement opportunities.

The product must be safe for other teams to install and run against their own Linear workspaces. It should start as a CLI-first product, then reuse the same core engine for GitHub Action, scheduled daemon, and Telegram delivery surfaces.

## 2. Product Goal

The first real win is a tool that:

- Reads recent Linear activity.
- Understands how a team uses Linear over time.
- Groups work into apps, products, projects, teams, or configurable buckets.
- Uses AI to recommend concrete product improvements.
- Shows source evidence for every recommendation.
- Produces useful reports and draft issue metadata.
- Writes to Linear only through explicit, previewed commands.

## 3. Users

- Founder-engineers managing many products at once.
- Product-minded engineering teams using Linear as their main execution system.
- Agencies and consultants working across multiple client products.
- AI-heavy teams that want better product follow-up from engineering activity.

## 4. Scope

Everything useful can remain part of the long-term product vision. The SRS uses milestones rather than permanent feature exclusions.

The first milestone should ship a CLI report-and-draft workflow:

- Read Linear.
- Generate AI recommendations.
- Include evidence links.
- Produce Markdown, JSON, and Telegram-friendly outputs.
- Generate reviewable draft issue metadata.
- Avoid Linear writes by default.

## 5. Safety Principles

Default behavior is read-only.

Allowed only after explicit opt-in:

- Create new Linear issues from draft metadata.
- Add labels through explicit commands.
- Add comments through explicit commands.

Every write path must include:

- Dry-run preview.
- Clear command name.
- Summary of what will change.
- Source evidence links.

Not allowed until a future approval model exists:

- Auto-closing issues.
- Reprioritizing issues.
- Deleting issues or comments.
- Broadly mutating existing issue content.

## 6. Core Requirements

### 6.1 CLI

The CLI is the canonical interface.

Required command areas:

- Initialize configuration.
- Validate Linear credentials.
- Generate reports for a time window.
- Generate draft issue metadata.
- Preview Linear writes.
- Execute explicit write actions.
- Inspect, export, and delete local learning profile.
- Run or inspect learning/autoresearch artifacts.

Example command shape:

```bash
linear-scout report --since 7d --group-by project --limit 2
linear-scout create-drafts --project BJJChat --from report.md
```

### 6.2 Linear Ingestion

The system must fetch enough Linear data to explain recommendations:

- Issues.
- Comments.
- Status changes.
- Labels.
- Assignees.
- Projects.
- Teams.
- Timestamps.
- Links back to source Linear records.

### 6.3 AI-First Recommendation Engine

AI is required from the first milestone.

The architecture must include:

- AI provider interface from day one.
- OpenAI as the reference provider.
- Prompt/input assembly layer.
- Evidence bundle builder.
- Recommendation validator.
- Confidence and uncertainty reporting.
- Stable structured recommendation model.

The AI layer must sit above an auditable evidence/ranking layer so users can see why a recommendation exists.

### 6.4 Product/App Grouping

When Linear projects are messy, the AI should make its best classification using available evidence.

The classifier must:

- Infer app/product/project/team grouping.
- Use Linear metadata when useful.
- Use local learned profile data when available.
- Include confidence scores.
- Mark uncertain items as low-confidence or unclassified.
- Record user corrections for future runs.

### 6.5 Learning and Autoresearch

`linear-scout` should improve over time by learning how a developer or team uses Linear.

Learning must be artifact-gated:

- Each learning pass has a mission.
- Each learning pass has an evaluator.
- Each learning pass produces an output artifact.
- Accepted learning updates are stored in local user-owned profile state.

Learning examples:

- Infer label-to-product mappings.
- Learn which teams own which apps.
- Detect duplicate recommendation patterns.
- Track accepted and rejected recommendations.

### 6.6 Settings

Settings must be split into two layers.

Shared workspace config:

- Team-shared defaults.
- Grouping preferences.
- Report formats.
- Recommendation rubric defaults.
- Safe write policy settings.
- Output templates.

User-local profile/settings:

- Secrets and API tokens.
- Learned app mappings.
- Private corrections.
- Accepted/rejected recommendation history.
- Provider credentials.
- Personal delivery preferences.

### 6.7 Recommendation Rubric

The quality bar for a good recommendation must be configurable in settings.

Default recommendation fields:

- Opportunity summary.
- Why it matters.
- Source Linear evidence links.
- Confidence.
- Affected app/product/project/team.
- Suggested owner/team when inferable.
- Draft issue title and body.
- Suggested labels and priority.
- Duplicate-risk explanation.

### 6.8 Outputs

Reports must support:

- Markdown.
- JSON.
- Telegram-friendly text.

Reports should be concise by default and grouped by the configured grouping mode.

## 7. Milestones

### Milestone 1: CLI Report + Draft Recommendations

Goal: prove the product value safely.

Includes:

- CLI setup.
- Linear read ingestion.
- AI provider interface with OpenAI implementation.
- Evidence bundle builder.
- AI recommendation generation.
- Grouping/classification with confidence.
- Markdown, JSON, and Telegram-friendly outputs.
- Draft Linear issue metadata.
- Configurable recommendation rubric.
- Local learning profile basics.
- Dry-run previews for write candidates.

### Milestone 2: Explicit Linear Write Commands

Goal: make recommendations actionable while preserving safety.

Includes:

- Create issues from drafts.
- Add labels/comments through explicit commands.
- Preview and confirmation workflow.
- Write audit log.

### Milestone 3: Learning and Autoresearch Loops

Goal: improve grouping and recommendation quality over time.

Includes:

- Artifact-gated learning missions.
- Evaluator-backed learning outputs.
- Local profile updates.
- Correction feedback loop.
- Recommendation acceptance/rejection tracking.

### Milestone 4: Automation Surfaces

Goal: reuse the core engine beyond the CLI.

Includes:

- GitHub Action.
- Scheduled daemon.
- Telegram delivery.
- Same config, report model, AI provider interface, and safety checks as CLI.

## 8. Acceptance Criteria

- A new user can configure Linear credentials without storing secrets in shared config.
- A user can run one CLI command and receive a useful report for recent Linear activity.
- Recommendations are AI-generated, evidence-backed, and auditable.
- Every recommendation links back to Linear source evidence.
- Messy project/app identity is handled through AI classification with confidence and local learning.
- Recommendation quality rules are configurable.
- Reports can be emitted as Markdown, JSON, and Telegram-friendly text from the same report model.
- Draft issue metadata is generated without writing to Linear by default.
- Any Linear write requires an explicit command and dry-run preview.
- Local learned profile data can be inspected, exported, and deleted.
- The core engine can be reused by non-CLI automation surfaces without duplicating recommendation logic.

## 9. Residual Risks

- The full-scope ambition is broad, so milestone boundaries must stay strict.
- AI quality depends on prompt design, evidence selection, provider behavior, and evaluator coverage.
- Local learning can become wrong if corrections are not visible and reversible.
- Open-source users need clear documentation around provider data sharing and Linear permissions.
