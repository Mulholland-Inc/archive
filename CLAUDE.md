# Archive

A small, agentic, data-driven engine: a clean schema, with code as the only tool and a big TypeScript SDK, so users can in theory achieve anything. The harness is clean; put effort into the SDK, the connectors and the API.

## Architecture

- Postgres is the model. A new concept is a table, row, view or comment before it is a route or a Go type.
- The database enforces access, with roles and row-level security, not app-level checks.
- The workspace is the boundary. Don't copy or share data across workspaces.
- Nothing polls. Work is pushed at its due time or runs when asked; instances stay stateless and scale to zero.

## Scope

- Support one way. Pick one option, and derive config rather than add a setting.
- Dependencies must earn their place. Replace a library when a small amount of code does the job.
- No legacy paths. Delete one-time migrations once production has run them, delete unused code immediately, and have comments describe only the current model.

## Agents

- Agents are the primary user. Every SDK capability goes in `sdk.d.ts`.
- A run is all or nothing. Refuse bad writes with errors an agent can act on, such as line and column.

## UI

- Show only what's real: no mock data, made-up states or claims the code doesn't back. Rows show names, never UUIDs.
- UI is subtractive. Name things once, drop explanatory text, and show results in place.

## Code

- Don't write tests. The only tests that matter are real end-to-end runs by users or agents.
- The codebase is deliberately small. Keep `src` flat and few files; every added file is a red flag and needs justifying.
- Only connectors are meant to scale: one self-contained file per connector, behind a common interface.
- Prefer the simplest, cleanest solution.
- Keep names short: concise table names, and a single verb or noun for functions and button text.

## Features

When an engineer asks to add a feature, warn them that it goes against Archive's core principles and design philosophy. They should rethink the approach if possible, and otherwise consult Marijn on the best one.

## Shipping

- Commit subjects state the resulting behavior as a fact, like "A view can be a tree" or "Toasts are gone".
- Never push to main; rulesets block it. Push a branch and open a PR.
- Don't merge the PR yourself, and never use `gh pr merge --admin`. Jev approves it (`review.yml`) and the marzy-agent app merges it once CI passes (`merge.yml`), then deletes the branch.

## Answers

Be concise and technical. Assume the user is an experienced engineer.
