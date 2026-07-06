---
name: autofix
description: Fix actionable GitHub PR review comments or comment threads from a --comment URL, or triage and auto-fix high-severity unresolved PR feedback from a --pr number or PR link. Comment mode checks out the PR, implements one scoped fix, runs autoreview, ships, and replies to and resolves the thread. PR mode gathers unresolved feedback, classifies fixed vs unfixed findings, ranks severity, and auto-fixes P0/P1 issues with autoreview, ship, thread replies, and resolution. Also supports a deferred fix-pr orchestration mode that makes local fixes without committing, reviewing, shipping, or replying. Use when the user invokes autofix with --comment or --pr, asks to automatically address GitHub PR feedback, or wants review fixes shipped back to the same PR.
---

# Autofix

Use this workflow to turn GitHub PR review feedback into scoped fixes on the same PR. Normal standalone autofix --comment <url> behavior remains unchanged: run autoreview, ship, and reply. The deferred fix-pr orchestration mode is only for the *fix-pr* skill's aggregate review-and-ship flow.

Provide exactly one target: `--comment <github-comment-or-thread-url>` or `--pr <number-or-pull-request-url> [--repo <owner/repo>]`.

- `--comment`: fix one comment or review thread end-to-end (autoreview, ship, reply, resolve).
- `--pr`: triage all unresolved PR review threads, then auto-fix findings more severe than P2.
- `--repo`: optional when `--pr` is a number and the repo is not the current checkout.

If the input is missing, malformed, or points to a GitHub issue instead of a PR, ask for the correct target before editing.

## GitHub Access

Use `gh` for checkout, repository, PR, review-thread, GraphQL, push, and reply/resolve operations unless the user provides another GitHub integration. Pass `--repo <owner/repo>` to `gh` commands whenever the target repository is not the current checkout.

## Deferred Fix-PR Orchestration Mode

When the handoff says to run the *autofix* skill in deferred mode for fix-pr orchestration with --comment <comment-or-thread URL>, keep the scope to a local fix for that comment or thread. Resolve the comment context, prepare or confirm the target PR checkout, implement the fix with focused tests and the *tdd* skill when practical, then stop before commit, autoreview, ship, GitHub replies, or thread resolution.

The deferred mode may continue on a dirty checkout only when those dirty changes are from the same active fix-pr orchestration and target the same PR. In deferred mode, stop before editing if the worktree contains unrelated changes, changes from another PR, or an ambiguous dirty state. Deferred mode must not commit, revert, or overwrite existing changes without explicit instruction.

Deferred mode must return changed files, tests/proof run, the original comment/thread URL, dirty-worktree status, and any blockers to the *fix-pr* skill. If test-first is not practical, return why and what proof was run instead.

## PR Mode

1. Resolve the PR from `--pr` and optional `--repo`; for bare numbers infer the repo with `gh repo view`.
2. Check out the PR branch with `gh pr checkout --repo <owner/repo> <number>` or `gh pr checkout <pull-request-url>`.
3. Fetch the PR base and head, avoid working on `main`, and stop before overwriting unrelated local changes.
4. Run the bundled read-only collector:

   ```bash
   python3 <skill-dir>/scripts/gather_unresolved_pr_comments.py --repo <owner/repo> --pr <number> --format json
   ```

5. Classify each thread as `accepted`, `already fixed`, or `rejected`, citing code, tests, diff context, or requirements.
6. Rank accepted findings as P0, P1, P2, or P3. Auto-fix only P0/P1 findings by default.
7. Fix each auto-fixable finding in the current PR checkout, using the *tdd* skill for code changes when practical.
8. Run focused verification, the *autoreview* skill in local/dirty mode, and the *ship* skill once to push the aggregate fix to the existing PR.
9. After the fix is pushed, reply to and resolve only review threads that were actually fixed.

## Comment Mode

1. Parse `owner`, `repo`, PR number, and comment or thread id from the URL.
2. Read the full comment thread, linked context, changed file, and relevant PR diff.
3. Prepare the PR checkout with `gh pr checkout --repo <owner/repo> <number>` or the pull request URL. For deferred fix-pr orchestration, the worktree may contain prior deferred fixes from the same active orchestration targeting the same PR.
4. Treat the GitHub comment as the scope boundary. Fix the requested issue and directly related sibling instances; avoid drive-by refactors.
5. Run the *tdd* skill for code changes when practical: write one focused failing test or reproduction first, make it pass, then refactor.
6. In deferred fix-pr orchestration mode, stop after this local implementation and focused proof; do not continue to autoreview, ship, reply, or resolve threads.
7. For standalone comment mode, run focused verification, the *autoreview* skill, the *ship* skill, then reply to and resolve the thread only after the fix is pushed.

## Stop Conditions

- Stop before editing if the target cannot be resolved to a PR or PR comment/review thread.
- In deferred fix-pr orchestration mode, stop before commit, autoreview, ship, GitHub replies, or thread resolution.
- In deferred fix-pr orchestration mode, stop before editing if the worktree contains unrelated changes, changes from another PR, or an ambiguous dirty state.
- In PR mode, stop before editing when there are no `accepted` P0 or P1 findings.
- Stop before shipping if a fix requires product judgment, broad redesign, or unrelated cleanup.
- Stop before replying as "fixed" if tests or autoreview are failing.
- Never force-push, rewrite shared history, or dismiss a review comment unless the user explicitly asks.
- Resolve only review threads that were actually fixed in the pushed commits.

## Final Report

Tell the user which target was addressed, what changed, what tests and autoreview ran, what was pushed, where replies or resolutions happened, and which lower-priority or non-auto-fixable findings remain open. For deferred fix-pr orchestration mode, report changed files, tests/proof run, the original comment/thread URL, dirty-worktree status, and any blockers instead of autoreview, ship, or reply evidence.
