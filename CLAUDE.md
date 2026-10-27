# Archive

- Never push to main; rulesets block it. To ship, push a branch and open a PR.
- Don't merge the PR yourself, and never use `gh pr merge --admin`. Jev approves it (`review.yml`) and the marzy-agent app merges it as soon as CI passes (`merge.yml`), then deletes the branch.
