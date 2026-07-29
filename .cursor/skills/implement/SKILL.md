---
name: implement
description: >-
  Issue-to-code workflow that takes a GitHub issue, plans the implementation,
  writes contract-based tests and production code via TDD, and runs unit
  tests to validate. Code changes are left uncommitted for user review.
  Use when implementing GitHub issues.
  Activated by commands: /ingest, /plan, /revise, /code, /validate, /respond.
---
# Implement Workflow Orchestrator

## Quick Start

1. If the user invoked a specific command (e.g., `/plan`, `/code`), read
   `commands/{command}.md` and follow it.
2. Otherwise, read `skills/controller.md` to load the workflow controller:
   - If the user provided a GitHub issue URL, execute the `/ingest` phase
   - Otherwise, execute the first phase the user requests

If a step fails or produces unexpected output (e.g., `gh` CLI errors, test
failures, build errors), stop and report the error to the user. Do not
advance to the next phase. Offer to retry the failed step or escalate.

For principles, hard limits, safety, quality, and escalation rules, see `guidelines.md`.
