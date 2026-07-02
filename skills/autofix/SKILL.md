---
name: autofix
description: Fix actionable GitHub PR review comments or comment threads from a --comment URL by checking out the PR, implementing the requested change, running focused tests and autoreview, shipping the update to the PR, and replying to the original GitHub comment or thread. Also supports a deferred fix-pr orchestration mode that makes local fixes without committing, reviewing, shipping, or replying. Use when the user invokes autofix with --comment, asks to automatically address a GitHub comment, or wants a comment-linked PR fix shipped back to the same PR.
---

# Autofix

Use this workflow to turn one GitHub PR comment or review thread into a scoped fix on the same PR. Normal standalone autofix --comment <url> behavior remains unchanged: run autoreview, ship, and reply. The deferred fix-pr orchestration mode is only for the *fix-pr* skill's aggregate review-and-ship flow.

The required input is:

```text
--comment <github-comment-or-thread-url>
```

If `--comment` is missing, malformed, or points to a GitHub issue instead of a PR, ask for the correct URL before editing.

On Codex, prefer an installed GitHub connector when available; use `gh` when connector coverage is insufficient or unavailable. On Claude Code, use `gh`/CLI unless the user provides another integration.

## Deferred Fix-PR Orchestration Mode

When the handoff says to run the *autofix* skill in deferred mode for fix-pr orchestration with --comment <comment-or-thread URL>, keep the scope to a local fix for that comment or thread. Resolve the comment context, prepare or confirm the target PR checkout, implement the fix with focused tests and the *tdd* skill when practical, then stop before commit, autoreview, ship, GitHub replies, or thread resolution.

The deferred mode may continue on a dirty checkout only when those dirty changes are from the same active fix-pr orchestration and target the same PR. In deferred mode, stop before editing if the worktree contains unrelated changes, changes from another PR, or an ambiguous dirty state. Deferred mode must not commit, revert, or overwrite existing changes without explicit instruction.

Deferred mode must return changed files, tests/proof run, the original comment/thread URL, dirty-worktree status, and any blockers to the *fix-pr* skill. If test-first is not practical, return why and what proof was run instead.

## Workflow

1. Resolve the comment.
   - Parse `owner`, `repo`, PR number, and comment or thread id from the URL.
   - For `#discussion_r...` or file review anchors, read the PR review comment and surrounding thread. Use GitHub GraphQL when the thread id is needed for an in-thread reply.
   - For `#issuecomment-...`, read the issue comment and confirm the issue is a PR.
   - Read the full comment thread, linked context, changed file, and relevant PR diff. Do not rely only on the URL fragment.

2. Prepare the PR checkout.
   - For standalone autofix, ensure the local worktree is clean or contains only work for this autofix. For deferred fix-pr orchestration, the worktree may contain prior deferred fixes from the same active fix-pr orchestration targeting the same PR. Stop before overwriting unrelated local changes.
   - Check out the target PR branch with the GitHub connector or `gh pr checkout <number>`.
   - Fetch the PR base and head. Do not work directly on `main`, and do not attach the fix to a branch for a different PR.

3. Implement the fix.
   - Treat the GitHub comment as the scope boundary. Fix the requested issue and directly related sibling instances; avoid drive-by refactors.
   - Run the *tdd* skill for code changes when practical: write one focused failing test or reproduction first, make it pass, then refactor.
   - If a test-first path is not practical, record why and use the narrowest validation that proves the comment is addressed.
   - Ask the user instead of guessing when the requested behavior is ambiguous, stale, or conflicts with existing requirements.
   - In deferred fix-pr orchestration mode, stop after this local implementation and focused proof; do not continue to autoreview, ship, reply, or resolve threads.

4. Verify and autoreview.
   - Run focused tests, linters, typechecks, or builds that cover the change.
   - Run the *autoreview* skill on the change before shipping. For dirty local work, use the autoreview local mode; for already committed work, use branch or commit mode with the PR base.
   - Pass the comment URL and a short context note into autoreview when useful, for example with a prompt file.
   - Fix accepted/actionable autoreview findings and rerun the affected tests and autoreview until it exits cleanly. Do not ship with failing required checks or unresolved accepted findings unless the user explicitly overrides.

5. Ship to the existing PR.
   - Run the *ship* skill to commit and push the fix to the PR branch. Do not create a new PR for an autofix comment, and do not edit the existing PR title or description unless the user asks.
   - Keep the commit message specific to the comment-driven fix.
   - If the *ship* workflow requires a PR comment for newly pushed commits, make the original comment or thread reply carry that detailed update when possible rather than posting duplicate status comments.

6. Reply to the GitHub comment.
   - Reply only after the fix is pushed.
   - For review threads, reply in the original thread, using GraphQL `addPullRequestReviewThreadReply` or the review-comment reply endpoint.
   - For flat PR comments, add a PR comment that links back to the original comment.
   - Include a concise summary of the fix, the commit or pushed branch, tests run, and the clean autoreview result. If something could not be verified, state that plainly.

## Stop Conditions

- Stop before editing if the URL cannot be resolved to a PR comment or review thread.
- In deferred fix-pr orchestration mode, stop before commit, autoreview, ship, GitHub replies, or thread resolution.
- In deferred fix-pr orchestration mode, stop before editing if the worktree contains unrelated changes, changes from another PR, or an ambiguous dirty state.
- Stop before shipping if the fix requires product judgment, broad redesign, or unrelated cleanup.
- Stop before replying as "fixed" if tests or autoreview are failing.
- Never force-push, rewrite shared history, resolve a review thread, or dismiss a review comment unless the user explicitly asks.

## Final Report

Tell the user:

- Which comment URL was addressed.
- What changed and where.
- Which tests and autoreview command ran.
- What was pushed to the PR.
- Where you replied on GitHub, if the reply URL is available.
- For deferred fix-pr orchestration mode, report changed files, tests/proof run, the original comment/thread URL, dirty-worktree status, and any blockers instead of autoreview, ship, or reply evidence.
