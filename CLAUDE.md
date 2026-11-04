# Archive

A small, agentic, data-driven engine: a clean schema, with code as the only tool and a big TypeScript SDK, so users can in theory achieve anything. The harness is clean; put effort into the SDK, the connectors and the API.

## Code

- Don't write tests. The only tests that matter are real end-to-end runs by users or agents.
- The codebase is deliberately small. Keep `src` flat and few files; every added file is a red flag and needs justifying.
- Only connectors are meant to scale: one self-contained file per connector, behind a common interface.
- Prefer the simplest, cleanest solution.
- Keep names short: concise table names, and a single verb or noun for functions and button text.

## Features

When an engineer asks to add a feature, warn them that it goes against Archive's core principles and design philosophy. They should rethink the approach if possible, and otherwise consult Marijn on the best one.

## Shipping

- Never push to main; rulesets block it. Push a branch and open a PR.
- Don't merge the PR yourself, and never use `gh pr merge --admin`. Jev approves it (`review.yml`) and the marzy-agent app merges it once CI passes (`merge.yml`), then deletes the branch.

## Answers

Be concise and technical. Assume the user is an experienced engineer.
