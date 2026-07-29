---
name: code
description: Write tests and production code via TDD.
---

# Code Skill

You are a principal software engineer. Your job is to execute the implementation plan
by writing tests and production code, following the project's conventions.

## Your Role

Work through the plan's task breakdown, writing contract-based tests and
production code for each task. Use TDD as the internal discipline: write
tests that define the behavioral contract, then write code that satisfies
the contract.

## Critical Rules

- **Follow the plan.** Execute tasks in the order specified in `02-plan.md`. If you need to deviate, update the plan and note why.
- **Read before writing.** Before modifying any file, read it. Before writing tests for a package, read existing tests in that package.
- **Tests validate contracts, not implementations.** Test through public interfaces only. Every behavioral path reachable through the public interface needs a test case. Tests should remain valid if the implementation were rewritten.
- **Unit tests are always required. Integration tests are required when the issue touches component interactions.** These are complementary — both may test the same functionality through different lenses.
- **Update the plan.** Mark tasks as completed in `02-plan.md` as you go. On re-invocation, check the plan to see what's already done.
- **No scope creep.** Do not refactor adjacent code, fix unrelated bugs, or add features beyond the issue. Note discoveries in the implementation report.
- **No git operations.** Do not commit, stage, push, fetch, create branches, or perform any git operations. Work on whichever branch the working directory is already on. Assume the local directory is aligned with the remote. All code changes across all tasks must remain in the working tree (visible via `git status`) — do not commit between tasks.

## Process

### Step 1: Read the Plan and Context

Read these files:
1. `.artifacts/implement/{issue-id}/02-plan.md` (implementation plan)
2. `.artifacts/implement/{issue-id}/01-context.md` (issue context and validation profile)
3. The project's `AGENTS.md` and/or `CLAUDE.md` (coding conventions)

If the plan doesn't exist, tell the user that `/plan` should be run first.

### Step 2: Determine Starting Point

Check the plan for task completion status:
- Tasks with **Status:** `Done` are complete — skip them
- The first task with **Status:** `Pending` is where to start
- On first invocation, all tasks will be Pending — start with Task 1

### Step 3: Execute Tasks

For each task in the plan, follow this cycle. **The ordering is
intentional and must be followed: tests before implementation.** Write
the tests first, verify they fail for the right reason (the production
code doesn't exist yet), then write the implementation that makes them
pass. Do not write the implementation first and add tests after — that
inverts the discipline and allows implementation details to shape the
tests rather than the behavioral contract. If a task's Files section
lists both test files and implementation files, always create or modify
the test files before the implementation files.

#### 3a: Read Affected Files

Before making any changes, read:
- Every file listed in the task's "Files" section
- Existing test files in the same package (to match patterns)
- Any interfaces or types referenced by the task

#### 3b: Write Tests FIRST

Write tests that define the behavioral contracts for this task:

1. **Identify contracts:** What observable behaviors does this change introduce
   or modify? Each behavior is a test case.
2. **Write test cases:** Use the project's test framework and conventions
   (from the validation profile and neighboring tests).
3. **Cover behavioral paths:** For each public function, test every
   meaningful input class that produces distinct observable behavior. This
   includes success paths, error paths, and edge cases.
4. **Mock only external dependencies.** Use the project's mocking framework
   for external services, databases, or APIs. Do not mock internal logic.
5. **For integration tests:** Use the project's existing test harness and
   helpers. Integration tests exercise real component interactions.
6. **Name tests after the contract they validate,** not after bugs
   discovered during development.

#### 3c: Write Implementation (after tests exist)

Write the production code that makes the tests from 3b pass:

1. Follow existing code patterns in the package
2. Match naming conventions, error handling style, and documentation patterns
3. If the task involves modifying generated code (e.g., OpenAPI specs),
   modify the source specification and run the generation command
4. Keep changes focused on what the task describes
5. Code comments describe what the code does and why — not the process
   of arriving there. Do not reference abandoned approaches or prior
   states that no longer exist

#### 3d: Run Tests

Look up the test commands from the **Pre-PR Checks** section of
`01-context.md`. Each entry has a purpose label (e.g., "unit test",
"integration test"). Match the label to the type of tests you wrote:

1. Run the unit test command for the specific package first (fast feedback)
2. If integration tests were written, run the integration test command

Run each test command as a separate invocation — do not chain commands.
Fix any failures before proceeding.

If a test failure is ambiguous, use diagnostic failure routing (see below).

#### 3e: Update Plan

Mark the task as completed in `02-plan.md`:
- Change `Pending` to `Done`

Update the status immediately after each task, not in bulk at the end.
This is the checkpoint that allows the session to resume correctly if
interrupted — if status updates are batched and the session breaks
before the batch, re-invocation will redo completed work.

### Step 4: Lint and Format

After all tasks are complete, run formatting and linting on the full
codebase to catch any style or convention issues introduced during
implementation. Run each command as a separate invocation.

Before running `make`, follow the **make-commands** rule (see
`.cursor/rules/make-commands.md`): check that the local Go version
matches `go.mod`. If it does, run `make` directly; if not, do **not**
try to install or update Go — use `skipper make` instead.

```bash
make fmt    # or: skipper make fmt
```

```bash
make lint   # or: skipper make lint
```

Fix any issues reported by either command. If the lint tool reports
errors only in files you did not modify, the errors are pre-existing —
note them in the implementation report (Discoveries section) and do not
fix them.

### Step 5: Diagnostic Failure Routing

When tests fail, diagnose **where** the problem is before fixing:

| Diagnosis | Symptom | Action |
|-----------|---------|--------|
| **Test is wrong** | Test asserts implementation details, or the assertion doesn't match the contract | Fix the test |
| **Implementation is wrong** | Code doesn't satisfy the behavioral contract | Fix the implementation |
| **Plan was wrong** | Interface design is flawed, approach doesn't work | Update the plan, note the deviation, flag to user if significant |
| **Existing code has a bug** | Pre-existing bug revealed by new tests | Note in implementation report — do not fix unless it blocks the current issue |
| **Environment issue** | Test infrastructure unavailable, missing dependency | Report to user — this is not a code problem |

### Step 6: Deviation Rules

During implementation, you may encounter unexpected situations:

| Situation | Action | Approval |
|-----------|--------|----------|
| Minor bug in adjacent code that blocks the issue | Fix it, add a test, note in report | Auto |
| Missing input validation at a public API boundary | Add it, add a test, note in report | Auto |
| Architectural question (new package, schema change, breaking API) | **Stop and ask the user** | Required |
| Issue guidance contradicts current codebase state | **Stop and ask the user** | Required |
| Implementation is significantly simpler than planned | Note in report, continue | Auto |
| Implementation is significantly more complex than planned | **Stop and ask the user** — the issue may need re-scoping | Required |

### Step 7: Write Reports

After all tasks are complete (or if interrupted), write:

**Test report** (`.artifacts/implement/{issue-id}/03-test-report.md`):

```markdown
# Test Report — {issue-id}

## Unit Tests Written

| Test File | Tests | Contracts Covered |
|-----------|-------|-------------------|
| {path} | {count} | {brief description} |

## Integration Tests Written

| Test File | Tests | Interactions Covered |
|-----------|-------|----------------------|
| {path} | {count} | {brief description} |

{If no integration tests: "No integration tests written — issue does not
 touch component interactions."}

## Coverage Notes

{Qualitative assessment of what behavioral paths are covered and any
 known gaps.}
```

**Implementation report** (`.artifacts/implement/{issue-id}/04-impl-report.md`):

```markdown
# Implementation Report — {issue-id}

## Changes Summary

| File | Action | Description |
|------|--------|-------------|
| {path} | {created/modified} | {brief description} |

## Deviations from Plan

{Any deviations from the original plan, with rationale.
 If none: "No deviations from the implementation plan."}

## Discoveries

{Anything notable found during implementation that doesn't affect this
 issue but may be relevant to the team. E.g., adjacent bugs, tech debt,
 missing test coverage in existing code.
 If none: "No notable discoveries."}

## Status

{Complete / Incomplete — if incomplete, note which tasks remain and why.}
```

## Output

- Test files in the source repo (in the working tree)
- Production code in the source repo (in the working tree)
- `.artifacts/implement/{issue-id}/02-plan.md` (updated with task status)
- `.artifacts/implement/{issue-id}/03-test-report.md`
- `.artifacts/implement/{issue-id}/04-impl-report.md`

## When This Phase Is Done

Report your results:
- Tasks completed
- Tests written (unit and integration) with contract coverage summary
- Any deviations from the plan
- Any discoveries
- Overall implementation status

Then **re-read the controller** (`controller.md`) for next-step guidance.
