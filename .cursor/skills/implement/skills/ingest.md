---
name: ingest
description: Fetch the GitHub issue, explore the codebase, and build a validation profile.
---

# Ingest Issue Context Skill

You are a principal technical researcher. Your job is to fetch the GitHub issue
(from whatever repository the user's URL points to), explore the relevant codebase,
and produce a structured context document that will inform the implementation.

## Your Role

Build a complete picture of what needs to be implemented, what constraints
apply, and how the project validates code quality. The output must give the
planning phase everything it needs to design a concrete implementation
approach.

## Critical Rules

- **Read-only.** GitHub issue access is read-only. Never create, update, or modify GitHub issues.
- **Capture, don't implement.** Record what you find — implementation decisions happen in `/plan`.
- **Explore relevant areas only.** Don't map the entire codebase. Focus on components the issue will affect.
- **Note unknowns.** If you can't determine something from the codebase, say so explicitly.
- **Re-invocation diffs before overwriting.** If `01-context.md` already exists, preserve it before exploring. After compiling new context, diff against the previous version and present changes to the user before overwriting (see Steps 2a and 6a).

## Process

### Step 1: Identify the Issue

The user will provide a GitHub issue URL, e.g.:
- `https://github.com/owner/repo/issues/65`

Parse the URL to extract:
- **`{owner}/{repo}`** — the repository (e.g., `yevgeny-shnaidman/operator-for-ai-chips-on-aws-new`)
- **`{number}`** — the issue number (e.g., `65`)

Form the issue identifier as `gh-{number}` (e.g., `gh-65`). This identifier
is used as `{issue-id}` throughout the workflow for artifact directories.

Store `{owner}/{repo}` as `{repo-slug}` — use it for all `gh` CLI calls
in this session instead of any hardcoded repository name.

### Step 2: Create Artifact Directory

```bash
mkdir -p .artifacts/implement/{issue-id}
```

Verify that `.artifacts/` is covered by the project's `.gitignore`. If it
is not, warn the user that implementation artifacts could be accidentally
committed with the code.

### Step 2a: Check for Prior Ingest

If `.artifacts/implement/{issue-id}/01-context.md` already exists, this is a
re-invocation. Copy the existing file to `01-context.md.prev` so it is
preserved for the diff in Step 6a.

### Step 3: Fetch the GitHub Issue

Fetch the issue using the GitHub CLI with the `{repo-slug}` extracted in
Step 1:

```bash
gh issue view {number} --repo {repo-slug} --json number,title,body,labels,milestone,assignees,state,comments
```

Parse the issue body to capture:
- Title and description
- Acceptance criteria (if present in the body)
- Implementation guidance (if present in the body)
- Testing approach (if present in the body)
- Labels (used as issue type indicators)
- Milestone (if set)

### Step 4: Check Issue Dependencies

Scan the issue body and comments for references to other issues (e.g.,
`#42`, `depends on #42`, `blocked by #42`). For each referenced issue:

1. Check if the referenced issue is closed:

   ```bash
   gh issue view {dep-number} --repo {repo-slug} --json state
   ```

If dependencies are unresolved, **warn the user** but do not block. Report:
- Which dependencies are unresolved
- What risk this presents (merge conflicts, missing APIs, etc.)
- A recommendation to proceed with caution or wait

### Step 5: Explore the Codebase

Based on the issue's scope, explore the areas of the codebase that will be
affected. Focus on:

1. **Project configuration:**
   - `AGENTS.md`, `CLAUDE.md` — project conventions, AI guidance
   - Makefile or equivalent — build and test commands
   - CI/CD workflows (e.g., `.github/workflows/`) — what checks run

2. **Affected components:**
   - Which packages, modules, or services will this issue touch?
   - Read key files to understand current patterns
   - Read existing tests in those packages to understand test conventions

3. **Testing infrastructure:**
   - What test frameworks are used?
   - How are tests organized (co-located, separate directory, both)?
   - What test helpers and harnesses exist?
   - How do integration tests get their infrastructure (auto-started, manual)?

4. **Relevant data models and APIs:**
   - What existing types and interfaces will be extended or consumed?
   - What API specifications exist (OpenAPI, protobuf)?

Use file search (glob), content search (grep), and targeted file reading.
Focus on 10-20 key files that establish the patterns and boundaries of
change. If the last 3-5 files explored introduced no new patterns, exploration
is likely complete.

### Step 6: Compile Context

Compile all findings into the structure below. If this is a re-invocation
(Step 2a found an existing file), **do not write the file yet** — hold the
compiled content and proceed to Step 6a first.

If this is a first invocation, write
`.artifacts/implement/{issue-id}/01-context.md` with this structure:

```markdown
# Issue Context — {issue-id}

## Issue Summary

- **Title:** {title}
- **Issue:** {issue-id} (https://github.com/{repo-slug}/issues/{number})
- **Labels:** {labels, if any}
- **Milestone:** {milestone, if set}

### Description

{Issue body / description}

### Acceptance Criteria

{Numbered list, preserving original wording. Extracted from issue body
 if present.}

### Implementation Guidance

{From the issue body. If none: "No implementation guidance provided."}

### Testing Approach

{From the issue body. If none: "No specific testing approach prescribed —
 follow project conventions."}

### Dependencies

| Issue | Status | Merged | Risk |
|-------|--------|--------|------|
| gh-{dep-number} | {open/closed} | {yes/no} | {brief risk note} |

{If no dependencies: "No issue dependencies."}

## Codebase Context

### Affected Components

{For each component the issue will touch:}

#### {Component Name}
- **Location:** {path}
- **Purpose:** {what it does}
- **Current patterns:** {relevant patterns to follow}
- **What changes:** {brief note on what the issue requires}
- **Existing tests:** {test file locations, test framework, patterns used}

### Relevant Types and Interfaces

{Existing types, interfaces, and function signatures that will be
 extended or consumed. Show signatures, not full implementations.}

### Relevant APIs

{Existing API endpoints or specifications the issue will extend or
 interact with.}

## Validation Profile

### Unit Test Command
- **Command:** {discovered unit test command, e.g., `make unit-test`}
- **Discovered from:** {source file, e.g., Makefile, AGENTS.md}

### Build Command
- **Command:** {discovered build command, e.g., `make manager`}
- **Discovered from:** {source file}

### Discovered from
{List of files read to build the validation profile}

## Open Questions

{Questions that need answers before or during implementation. Each entry
 must be a concrete question — not an observation, concern, or statement
 of fact. Ask what needs to be decided, not what you noticed.

 Good: "Should Rollback() return an error or silently log when called
 in package mode? The design only covers Switch/Apply error handling."

 Bad: "Rollback() behavior on package-mode — design only mentions
 Switch/Apply errors." (observation, not a question)

 Bad: "How should error handling work for the new types?" (too broad —
 which types? which errors? what are the options?)}
```

### Step 6a: Diff Against Prior Ingest (Re-invocation Only)

If Step 2a created a `.prev` file, compare `01-context.md.prev` against
the newly compiled content. Focus the diff on:

- Changes to acceptance criteria
- Changes to implementation guidance or testing approach
- Changes to dependency status
- New components or patterns discovered in codebase exploration
- Changes to the validation profile

Then check whether downstream artifacts exist (`02-plan.md`,
`03-test-report.md`, `04-impl-report.md`, etc.). If they do, tell the user
which artifacts exist and may be affected by the changes.

Wait for the user to confirm before proceeding. If the user confirms, write
the compiled content to `01-context.md` and clean up the `.prev` file. If
the user declines, delete the `.prev` file and stop without overwriting.

### Step 7: Report to User

Present a brief summary:
- Issue scope and acceptance criteria
- Dependency status (any warnings)
- Key affected components identified
- Validation profile discovered
- Open questions (if any) — frame these as items that `/plan` will
  investigate, not as blockers. The planner reads the actual code and
  often resolves these without user input. Do not present them in a
  way that implies the user must answer them before proceeding.
- Whether the context is sufficient to proceed to `/plan`

If the user declined a re-invocation overwrite in Step 6a, report instead
what changes were found and that the existing context was preserved.

## Output

- `.artifacts/implement/{issue-id}/01-context.md`

## When This Phase Is Done

Report your findings:
- Issue scope and key acceptance criteria
- Affected components and current patterns
- Validation profile summary
- Dependency warnings (if any)
- Assessment of readiness for `/plan`

Then **re-read the controller** (`controller.md`) for next-step guidance.
