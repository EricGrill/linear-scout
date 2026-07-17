# linear-scout

`linear-scout` is an open-source, AI-first assistant that reviews recent Linear
activity and turns noisy project motion into evidence-backed product improvement
opportunities. It is **read-only by default** — it never writes to your Linear
workspace unless a future, explicitly-opted-in write command is run.

This repository currently implements **Milestone 1**: a CLI that reads recent
Linear activity, generates AI recommendations with source evidence links, and
emits Markdown / JSON / Telegram-friendly reports plus reviewable draft issue
metadata.

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

## Commands

| Command | Purpose |
|---------|---------|
| `init` | Create profile dir + template config files. |
| `validate` | Check Linear and OpenAI credentials. |
| `report` | Generate an AI recommendation report (`--since`, `--group-by`, `--format`, `--limit`). |
| `create-drafts` | Produce reviewable draft issue metadata (no writes). |
| `preview` | Dry-run preview of Linear writes (write actions land in Milestone 2). |
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

`linear-scout` is read-only by default. It reads Linear activity and produces
reports and draft metadata; it does not create, modify, close, reprioritize, or
delete anything in Linear. Explicit, previewed write commands are planned for
Milestone 2.

## Data sharing

Generating a report sends the selected window's Linear issue/comment text and
metadata to the configured AI provider (OpenAI by default). See
[docs/provider-data-sharing.md](docs/provider-data-sharing.md) for details.

## Development

```bash
go test ./...
go build ./cmd/linear-scout
```
