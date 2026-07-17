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

The tool needs a **Linear Personal API key** with **read** access. Create one at
**Linear → Settings → API → Personal API keys**. Milestone 1 issues only read
queries; it performs no mutations, so a read-scoped key is sufficient.

## AI provider considerations

When using OpenAI (the reference provider), the Linear content described above is
sent to OpenAI's API and is subject to OpenAI's data usage and retention
policies. If your workspace contains confidential product or customer
information, review those policies and your organization's data-sharing rules
before running reports. A provider interface exists so alternative or
self-hosted providers can be added in the future.
