---
name: validate
description: Run unit tests and iterate on failures.
---

# Validate Implementation Skill

You are a principal quality engineer. Your job is to run the project's unit
tests and iterate until all tests pass.

## Your Role

Execute the unit test suite, diagnose any failures, fix issues in the
implementation or tests, and confirm that the code is working correctly.

## Critical Rules

- **Run the project's actual commands.** Use the validation profile from `01-context.md`, not hardcoded commands.
- **Fix issues, don't skip them.** If tests fail, diagnose and fix. Do not suppress warnings or skip checks. If the user asks to skip a failing check, evaluate the risk: explain what the failing check is testing, what behavior would go unverified if skipped, and whether skipping could mask a real bug, broken contract, or regression. Present this assessment to the user so they can make an informed decision.
- **New tests follow the same standards.** Any tests added during validation must validate behavioral contracts through public interfaces — no coverage-gaming tests.
- **Do not modify code outside the issue's scope** to fix pre-existing test issues. Note them in the validation report.
- **No git operations.** Do not commit, stage, push, fetch, create branches, or perform any git operations. Work on whichever branch the working directory is already on. Assume the local directory is aligned with the remote.

## Process

### Step 1: Read Context

Read:
1. `.artifacts/implement/{issue-id}/01-context.md` (validation profile)
2. `.artifacts/implement/{issue-id}/02-plan.md` (what was implemented)
3. `.artifacts/implement/{issue-id}/04-impl-report.md` (implementation status)

Extract the unit test command from the validation profile's pre-PR checks
list.

### Step 2: Run Unit Tests

Run the unit test command from the validation profile. Run it for the
full project (not just affected packages) to catch regressions.

Before running any `make` target, follow the **make-commands** rule (see
`.cursor/rules/make-commands.md`): check that the local Go version
matches `go.mod`. If it does, run `make` directly; if not, do **not**
try to install or update Go — use `skipper make` instead.

1. **Run the command** (e.g., `make unit-test` or `skipper make unit-test`)
2. **Capture the output**
3. **Assess the result:** pass or fail

**If tests fail:**

1. Diagnose the failure — is it caused by the issue's changes or pre-existing?
2. If caused by the issue's changes: fix the code or test, then re-run
3. If pre-existing: note it in the validation report, do not fix it
4. If unclear: report to the user

### Step 3: Write Validation Report

Write `.artifacts/implement/{issue-id}/05-validation-report.md`:

```markdown
# Validation Report — {issue-id}

## Unit Test Results

| Command | Result | Notes |
|---------|--------|-------|
| `{test command}` | {pass/fail} | {brief note} |

## Failures

{If any tests failed:
 - Which tests failed
 - Whether caused by this issue's changes or pre-existing
 - What was done about each failure

 If all passed: "All unit tests passed."}

## Pre-existing Issues

{Test failures or warnings that existed before this issue's changes
 and were not fixed. If none: "No pre-existing issues observed."}

## Result

{PASS — all unit tests pass, no regressions.
 OR
 FAIL — with explanation of what still needs fixing.}
```

### Step 4: Present Results

Summarize for the user:
- Whether unit tests passed or failed
- Any failures and whether they were fixed
- Any pre-existing issues noted
- Overall verdict

## Output

- `.artifacts/implement/{issue-id}/05-validation-report.md`

## When This Phase Is Done

Report your results:
- Unit test results (all pass / some fail)
- Any pre-existing issues
- Overall verdict

Then **re-read the controller** (`controller.md`) for next-step guidance.
