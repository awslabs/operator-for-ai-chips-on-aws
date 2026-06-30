---
description: GitHub repository configuration for issue tracking and PR operations
alwaysApply: true
---

# GitHub Repository

The target GitHub repository is **not hardcoded**. It is determined at
runtime from the issue URL the user passes to `/ingest`.

During `/ingest`, the URL (e.g.,
`https://github.com/owner/repo/issues/42`) is parsed to extract
`{owner}/{repo}` (the "repo-slug"). All subsequent `gh` CLI calls in
the session must use `--repo {repo-slug}`.

Always pass `--repo {repo-slug}` when using `gh issue`, `gh pr`, or any
other `gh` subcommand that resolves a repository. Do not rely on `gh`'s
automatic repo detection — it may resolve to the upstream fork parent
instead of the intended repo.
