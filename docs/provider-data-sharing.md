# Provider Data Sharing & Linear Permissions

`linear-scout` is transparent about what data leaves your machine and where it
goes. Read this before running it against a workspace that contains sensitive
information.

## What is sent, and when

- **On read (`report`, `create-drafts`):** the tool fetches issues and comments
  updated within the requested time window from Linear, then sends that content
  — issue titles/descriptions, comment bodies, labels, assignees, project/team
  identifiers, and timestamps — to the configured **AI provider** (OpenAI by
  default) to generate recommendations.
- **On `validate`:** a minimal Linear query runs to confirm the token works.
  Nothing is sent to the AI provider.
- **On `init` / `profile` commands:** nothing leaves your machine.

No data is sent to any third party unless you run a command that generates
recommendations.

## Secrets never leave the local profile

API tokens and provider credentials live only in the user-local profile
directory (`$XDG_CONFIG_HOME/linear-scout/profile.yaml`, or
`~/.config/linear-scout/profile.yaml`). They are never written to the shared
workspace config, never logged, and never transmitted anywhere except as the
authorization header to Linear and to your AI provider.

## Linear permissions

The tool needs a **Linear Personal API key**, created at **Linear → Settings →
API → Personal API keys**.

- For read-only use (`report`, `create-drafts`, `preview`), a **read**-scoped
  key is sufficient — these commands perform no mutations.
- For the Milestone 2 write commands (`create-issues`, `comment`, `label` with
  `--execute`), the key must also have **write** access. Even with a write-scoped
  key, the tool only ever creates issues, adds comments, and adds labels; it
  never closes, reprioritizes, or deletes anything.

Writes never happen implicitly: a command mutates Linear only when you pass
`--execute`, and every change is recorded in `audit.log` in the profile
directory.

## AI provider considerations

When using OpenAI (the reference provider), the Linear content described above is
sent to OpenAI's API and is subject to OpenAI's data usage and retention
policies. If your workspace contains confidential product or customer
information, review those policies and your organization's data-sharing rules
before running reports. A provider interface exists so alternative or
self-hosted providers can be added in the future.
