# linear-scout

`linear-scout` is an open-source, AI-first assistant that reviews recent Linear
activity and turns noisy project motion into evidence-backed product improvement
opportunities. It is **read-only by default** — it never writes to your Linear
workspace unless a future, explicitly-opted-in write command is run.

This repository implements **Milestone 1** (read-only reports + draft
recommendations) and **Milestone 2** (explicit, previewed Linear write
commands). Reads never mutate Linear; writes only happen behind an explicit
`--execute` flag with a dry-run preview, a confirmation prompt, and an audit log.

## Install

```bash
go install github.com/EricGrill/linear-scout/cmd/linear-scout@latest
```

Requires Go 1.22+.

## Quick start

1. **Initialize configuration:**

   ```bash
   linear-scout init
   ```

   This creates the user-local profile directory
   (`$XDG_CONFIG_HOME/linear-scout/`, or `~/.config/linear-scout/`) with a
   `profile.yaml`, and a shared `linear-scout.yaml` in the current directory.

2. **Add your secrets** to `profile.yaml` (this file, and only this file, holds
   secrets — never commit it):

   ```yaml
   linear_token: "lin_api_..."   # Linear Settings → API → Personal API keys (read scope)
   openai_key: "sk-..."          # OpenAI API key
   ```

3. **Validate credentials:**

   ```bash
   linear-scout validate
   ```

4. **Generate a report:**

   ```bash
   linear-scout report --since 7d --group-by project
   linear-scout report --since 24h --format json
   linear-scout report --since 2w --format telegram --limit 5
   ```

5. **Generate draft issue metadata** (no writes to Linear):

   ```bash
   linear-scout create-drafts --since 7d
   ```

6. **Act on recommendations** (Milestone 2 — dry-run by default):

   ```bash
   # Preview the issues that would be created (never writes):
   linear-scout preview --since 7d --team <TEAM_ID>

   # Create them for real (prompts for confirmation):
   linear-scout create-issues --since 7d --team <TEAM_ID> --execute

   # Add a comment or labels to an existing issue:
   linear-scout comment --issue ENG-123 --body "Flagged by linear-scout" --execute
   linear-scout label --issue ENG-123 --labels "needs-triage,product" --execute
   ```

   Add `--yes` to skip the confirmation prompt in automation. Every executed
   write is appended to `audit.log` in the profile directory.

## Commands

| Command | Purpose |
|---------|---------|
| `init` | Create profile dir + template config files. |
| `validate` | Check Linear and OpenAI credentials. |
| `report` | Generate an AI recommendation report (`--since`, `--group-by`, `--format`, `--limit`). |
| `create-drafts` | Produce reviewable draft issue metadata (no writes). |
| `preview` | Dry-run of the issues `create-issues` would create (never writes). |
| `create-issues` | Create Linear issues from drafts. Dry-run unless `--execute`. |
| `comment` | Add a comment to an issue. Dry-run unless `--execute`. |
| `label` | Add labels to an issue. Dry-run unless `--execute`. |
| `profile inspect\|export\|delete` | Inspect, export, or delete local learned profile state. |

## Configuration model

Settings are split into two layers:

- **Shared workspace config** (`linear-scout.yaml`, safe to commit): grouping
  preferences, report formats, and the recommendation rubric. **No secrets.**
- **User-local profile** (`$XDG_CONFIG_HOME/linear-scout/`, never committed):
  API tokens, provider credentials, and learned profile state stored as plain,
  inspectable JSON files.

See `linear-scout.example.yaml` for the shared config format.

## Safety

`linear-scout` is read-only by default. `report`, `create-drafts`, and
`preview` never mutate Linear. The write commands (`create-issues`, `comment`,
`label`) are dry-run unless `--execute` is passed, and even then print a change
summary, require confirmation (unless `--yes`), and record every change to an
append-only `audit.log`.

By design the tool can only create issues, add comments, and add labels. It
**cannot** close, reprioritize, or delete issues/comments, or broadly mutate
existing content — those operations are not implemented.

## Data sharing

Generating a report sends the selected window's Linear issue/comment text and
metadata to the configured AI provider (OpenAI by default). See
[docs/provider-data-sharing.md](docs/provider-data-sharing.md) for details.

## Development

```bash
go test ./...
go build ./cmd/linear-scout
```
