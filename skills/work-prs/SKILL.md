---
name: work-prs
description: Work through open non-draft GitHub pull requests in chronological order, only after CI checks are complete; fix test failures and blocking code issues, commit one targeted fix per PR, and push without merging. Use when the user asks to maintain PRs, work open PRs, fix failing PRs, process PR queues, or run PR maintenance, including optional --limit N and --repo owner/repo flags.
---

# Work PRs

Work through open pull requests, fixing test failures and blocking code issues, then pushing targeted fixes. Never merge PRs.

## Inputs

- `--limit <N>`: process at most `N` qualifying PRs.
- `--repo <owner/repo>`: pass `--repo <owner/repo>` to all `gh` commands instead of using the current directory's repo.

## Workflow

### 1. Read Project Instructions

Read `AGENTS.md`, `CLAUDE.md`, `README.md`, and relevant project docs to understand test commands and coding conventions. Pay special attention to build, test, lint, and formatting commands.

### 2. Discover Candidate PRs

Run:

```bash
gh pr list --state open --json number,title,headRefName,baseRefName,url,isDraft,createdAt
```

If `--repo <owner/repo>` was provided, include that flag on this and every later `gh` command.

Filter and sort:

- Exclude drafts.
- Sort by `createdAt` ascending, oldest first.
- Apply `--limit <N>` after check-status filtering unless the user clearly asked for the first N open PRs before filtering.

If no non-draft PRs exist, report that and stop.

### 3. Filter By Check Status

For each candidate PR, run:

```bash
gh pr checks <number> --json name,state
```

A PR qualifies only if:

- It has at least one check.
- Every check's `state` is terminal, not `PENDING`, `QUEUED`, `IN_PROGRESS`, or `WAITING`.

Skip PRs that do not qualify and log why each was skipped.

### 4. Process Qualifying PRs

Process qualifying PRs sequentially, oldest first.

#### 4a. Check Out The PR Branch

```bash
gh pr checkout <number>
```

#### 4b. Run Tests And Fix Failures

1. Run the project test suite from project instructions, falling back to `make test` when that is clearly supported.
2. If all tests pass, continue to review.
3. If tests fail:
   - Read the failure output carefully.
   - Identify the source-code root cause when the test is correct.
   - Fix tests only when the test itself is wrong.
   - Apply the smallest targeted fix.
   - Re-run the failing test or focused test subset.
   - Repeat for at most 3 fix-and-rerun cycles per PR.

If still failing after 3 cycles, commit any clearly correct progress only when useful, note remaining failures, and move to the next PR.

#### 4c. Review Blocking Issues

Run:

```bash
gh pr diff <number>
```

Review for blocking issues only:

- Bugs: nil dereferences, off-by-one errors, logic errors, race conditions
- Security vulnerabilities: injection, auth bypass, secrets exposure
- Correctness problems: wrong return values, missing error handling that causes silent failures
- Resource leaks: unclosed connections, goroutine leaks

Do not flag style preferences, naming opinions, missing comments, formatting-only concerns, or minor refactoring opportunities.

#### 4d. Fix Blocking Issues

For each blocking issue:

1. Read the relevant source files for full context.
2. Apply the smallest targeted fix that preserves the PR author's intent.
3. Run tests to verify the fix.

#### 4e. Commit And Push

If changes were made:

1. Run the project formatter if one exists.
2. Run lint when the project documents a lint command.
3. Stage changed files with explicit file paths, not `git add -A`.
4. Commit once for the PR:

   ```bash
   git commit -m "fix: <concise description of what was fixed>"
   ```

5. Push with regular `git push`.

If no changes were made, log `PR #N: no issues found`.

#### 4f. Return To Main

```bash
git checkout main
```

If the repo's default branch is not `main`, return to the PR's base branch.

### 5. Report Summary

After processing all qualifying PRs, output a summary table:

```text
PR     Title                          Action Taken
#12    Add reflog mode                Fixed 2 test failures, fixed nil deref in handler
#15    Refactor model handlers        No issues found
#18    Add stash drop action          Fixed missing error check in drop flow
```

## Rules

- Never merge PRs.
- Never force-push.
- Never modify CI configuration, Makefiles, or test commands just to make checks pass. Fix source code or tests.
- Preserve the PR author's intent.
- Make minimal, targeted changes.
- Use one commit per PR.
- Stop early on a PR if stuck after 3 attempts; summarize the blocker and move on.
- Surface exact `gh`, git, test, and push errors.
