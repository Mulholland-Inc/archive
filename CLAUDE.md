# Archive

- Never push to main; rulesets block it. To ship, push a branch and open a PR.
- Don't merge the PR yourself, and never use `gh pr merge --admin`. The review workflow (`.github/workflows/review.yml`) approves it and auto-merges it once CI passes.
