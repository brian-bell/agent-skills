---
name: work-prs
description: Work through open non-draft GitHub pull requests in chronological order, only after CI checks are complete; fix test failures and blocking code issues, commit one targeted fix per PR, and push without merging. Use when the user asks to maintain PRs, work open PRs, fix failing PRs, process PR queues, or run PR maintenance, including optional --limit N and --repo owner/repo flags.
---

# Work PRs

Work through open pull requests, fixing test failures and blocking code issues, then pushing targeted fixes. Never merge PRs.

## GitHub Access

Use `gh` for repository, PR, check, diff, checkout, and push operations unless the user provides another GitHub integration. If `--repo <owner/repo>` was provided, include that flag on every applicable `gh` command.

## Inputs

- `--limit <N>`: process at most `N` qualifying PRs.
- `--repo <owner/repo>`: target repository.

## Workflow

1. Read `AGENTS.md`, `CLAUDE.md`, `README.md`, and relevant project docs to understand test commands and coding conventions.
2. Discover candidate PRs:

   ```bash
   gh pr list --state open --json number,title,headRefName,baseRefName,url,isDraft,createdAt
   ```

3. Exclude drafts, sort by `createdAt` ascending, and apply `--limit <N>` after check-status filtering unless the user clearly asked for the first N open PRs before filtering.
4. For each candidate PR, check whether CI is terminal:

   ```bash
   gh pr checks <number> --json name,state
   ```

   A PR qualifies only if it has at least one check and every check's `state` is terminal, not `PENDING`, `QUEUED`, `IN_PROGRESS`, or `WAITING`.

5. Process qualifying PRs sequentially, oldest first:
   - Check out the PR branch with `gh pr checkout <number>`.
   - Run the project test suite from project instructions, falling back to `make test` when that is clearly supported.
   - If tests fail, read the failure output, identify the source-code root cause when the test is correct, apply the smallest targeted fix, and rerun the failing test or focused subset. Repeat for at most 3 cycles per PR.
   - Review blocking issues only with `gh pr diff <number>`.
   - Fix nil dereferences, off-by-one errors, logic errors, race conditions, security vulnerabilities, correctness problems, and resource leaks. Do not flag style or minor refactoring preferences.
   - Run formatter and lint commands when documented.
   - Stage changed files with explicit file paths, commit once for the PR, and push with regular `git push`.
   - Return to `main`, or the PR's base branch if the repo default is not `main`.

If no changes were made, log `PR #N: no issues found`.

## Report Summary

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
