---
name: autofix
description: Fix actionable GitHub PR review comments or comment threads from a --comment URL, or triage and auto-fix high-severity unresolved PR feedback from a --pr number or PR link. Comment mode checks out the PR, implements one scoped fix, runs autoreview, ships, and replies to and resolves the thread. PR mode gathers unresolved feedback, classifies fixed vs unfixed findings, ranks severity, and auto-fixes P0/P1 issues with autoreview, ship, thread replies, and resolution. Also supports a deferred fix-pr orchestration mode that makes local fixes without committing, reviewing, shipping, or replying. Use when the user invokes autofix with --comment or --pr, asks to automatically address GitHub PR feedback, or wants review fixes shipped back to the same PR.
---

# Autofix

Use this workflow to turn GitHub PR review feedback into scoped fixes on the same PR. Normal standalone autofix --comment <url> behavior remains unchanged: run autoreview, ship, and reply. The deferred fix-pr orchestration mode is only for the *fix-pr* skill's aggregate review-and-ship flow.

Provide exactly one target:

```text
--comment <github-comment-or-thread-url>
```

or

```text
--pr <number-or-pull-request-url> [--repo <owner/repo>]
```

- `--comment`: fix one comment or review thread end-to-end (autoreview, ship, reply, resolve).
- `--pr`: triage all unresolved PR review threads, then auto-fix findings more severe than P2.
- `--repo`: optional when `--pr` is a number and the repo is not the current checkout.

If the input is missing, malformed, or points to a GitHub issue instead of a PR, ask for the correct target before editing.

## GitHub Access

Prefer an installed GitHub connector for PR metadata, comments, patches, issue comments, labels, reactions, and PR creation/update operations when available. Use `gh` for checkout, branch state, pushing, current-branch PR discovery, review-thread GraphQL operations, or connector coverage gaps.

## Deferred Fix-PR Orchestration Mode

When the handoff says to run the *autofix* skill in deferred mode for fix-pr orchestration with --comment <comment-or-thread URL>, keep the scope to a local fix for that comment or thread. Resolve the comment context, prepare or confirm the target PR checkout, implement the fix with focused tests and the *tdd* skill when practical, then stop before commit, autoreview, ship, GitHub replies, or thread resolution.

The deferred mode may continue on a dirty checkout only when those dirty changes are from the same active fix-pr orchestration and target the same PR. In deferred mode, stop before editing if the worktree contains unrelated changes, changes from another PR, or an ambiguous dirty state. Deferred mode must not commit, revert, or overwrite existing changes without explicit instruction.

Deferred mode must return changed files, tests/proof run, the original comment/thread URL, dirty-worktree status, and any blockers to the *fix-pr* skill. If test-first is not practical, return why and what proof was run instead.

## PR Mode

Use PR mode when `--pr` is provided instead of `--comment`.

### 1. Resolve The PR

- Parse `owner`, `repo`, and PR number from `--pr` and optional `--repo`.
- For pull request URLs, read repo and number from the URL.
- For bare numbers, infer repo from `--repo`, the connector, or `gh repo view`.
- Check out the PR branch with the connector when supported or `gh pr checkout --repo <owner/repo> <number>`. When `--pr` is a pull request URL, `gh pr checkout <pull-request-url>` is also valid. Do not run `gh pr checkout <number>` without `--repo` when the resolved repository is not the current checkout.
- Fetch the PR base and head with the same resolved `owner/repo`. Do not work directly on `main`, and do not attach fixes to a different PR.
- Ensure the local worktree is clean or contains only work for this autofix run. Stop before overwriting unrelated local changes.

### 2. Collect Unresolved Feedback

Run the bundled read-only collector when thread-level GraphQL state is needed:

```bash
python3 <skill-dir>/scripts/gather_unresolved_pr_comments.py --repo <owner/repo> --pr <number> --format json
```

Use `--format markdown` when a human-readable table is easier to scan. The collector uses `gh repo view`, `gh pr view`, and read-only GraphQL queries only.

If the collector cannot be used, gather the same data with connector reads or read-only GitHub commands and include all unresolved PR review threads, paging through results until complete.

### 3. Classify Each Thread

For every unresolved thread:

- Read the referenced file, nearby code, and relevant PR diff.
- Compare the comment with the current working tree.
- Run focused tests only when needed to determine whether the finding is still valid.
- Preserve the reviewer's intent; do not dismiss a comment because the wording is terse.

Use exactly one decision per thread:

- `accepted`: the finding is valid and still needs a code, test, docs, or behavior change.
- `already fixed`: the finding was valid, but the current code or tests already address it.
- `rejected`: the finding is incorrect, obsolete, out of scope, or conflicts with project requirements.

For `accepted` findings, state the concrete local change needed and keep the thread URL available.

For `already fixed` and `rejected` findings, cite specific evidence such as file paths, line numbers, test output, PR diff context, or requirements.

### 4. Rank Unfixed Findings By Severity

For each `accepted` finding, assign one severity:

| Severity | Meaning | Auto-fix in PR mode |
|---|---|---|
| P0 | Critical: security vulnerability, data loss/corruption, production crash, or broken core behavior | yes |
| P1 | High: correctness bug, missing guard that causes failure, broken contract/tests, or materially unsafe behavior | yes |
| P2 | Medium: maintainability, localized edge case, or quality issue with limited blast radius | no |
| P3 | Low: nit, naming/style preference, optional refactor, or docs polish | no |

When severity is ambiguous, choose the lower severity unless the comment clearly describes user-visible breakage, security impact, or data corruption.

Display a Markdown table before editing:

```text
| Decision | Severity | Location | Reviewer | Finding | Evidence | Action | URL |
|---|---|---|---|---|---|---|---|
| accepted | P1 | src/foo.go:42 | @reviewer | Missing nil check | Current code indexes before checking length | auto-fix | https://github.com/... |
| already fixed | - | src/bar.go:18 | @reviewer | Guard missing | Guard exists in current diff | none | https://github.com/... |
| accepted | P3 | docs/api.md:7 | @reviewer | Rephrase example | Wording preference only | report only | https://github.com/... |
```

If there are no `accepted` P0 or P1 findings, display the table and stop without editing. Report any P2/P3 accepted findings and non-actionable classifications for the user to handle separately.

### 5. Implement Auto-Fixable Findings

Fix every `accepted` P0 and P1 finding in the current PR checkout.

- Treat each review thread as its scope boundary. Fix the requested issue and directly related sibling instances; avoid drive-by refactors.
- Run the *tdd* skill for code changes when practical.
- If a test-first path is not practical, record why and use the narrowest validation that proves the comment is addressed.
- Ask the user instead of guessing when the requested behavior is ambiguous, stale, or conflicts with existing requirements.
- Stop before shipping if an auto-fixable finding requires product judgment, broad redesign, or unrelated cleanup; report the blocker and continue with the remaining auto-fixable findings only when they are independent.

### 6. Verify, Autoreview, And Ship

- Run focused tests, linters, typechecks, or builds that cover the aggregate change.
- Run the *autoreview* skill in local/dirty mode on the resulting diff before shipping.
- Fix accepted/actionable autoreview findings and rerun affected tests and autoreview until it exits cleanly.
- Do not ship with failing required checks or unresolved accepted autoreview findings unless the user explicitly overrides.
- Run the *ship* skill once to commit and push the aggregate fix to the PR branch. Do not create a new PR. Do not rewrite the existing PR title or description unless the user asks.
- Keep commit messages specific to the review-driven fixes.

### 7. Reply To And Resolve Fixed Threads

After the fix is pushed, for each addressed P0/P1 thread:

- Reply in the original thread using GraphQL `addPullRequestReviewThreadReply` or the review-comment reply endpoint.
- Resolve the thread with GraphQL `resolveReviewThread` once the reply confirms the fix, tests run, and clean autoreview result.
- Include a concise summary of the fix, the commit or pushed branch, tests run, and the clean autoreview result. If something could not be verified, state that plainly.

Do not reply as "fixed" or resolve a thread when its finding was not actually addressed in the pushed commits.

For `already fixed` and `rejected` threads, do not resolve them automatically unless the user explicitly asks. Report them in the final output instead.

## Comment Mode

Use comment mode when `--comment` is provided.

### 1. Resolve The Comment

- Parse `owner`, `repo`, PR number, and comment or thread id from the URL.
- For `#discussion_r...` or file review anchors, read the PR review comment and surrounding thread. Use GitHub GraphQL when the thread id is needed for an in-thread reply.
- For `#issuecomment-...`, read the issue comment and confirm the issue is a PR.
- Read the full comment thread, linked context, changed file, and relevant PR diff. Do not rely only on the URL fragment.

### 2. Prepare The PR Checkout

- For standalone autofix, ensure the local worktree is clean or contains only work for this autofix. For deferred fix-pr orchestration, the worktree may contain prior deferred fixes from the same active fix-pr orchestration targeting the same PR. Stop before overwriting unrelated local changes.
- Check out the target PR branch with the connector when supported or `gh pr checkout --repo <owner/repo> <number>`. When the comment URL includes a pull request URL, `gh pr checkout <pull-request-url>` is also valid. Do not run `gh pr checkout <number>` without `--repo` when the resolved repository is not the current checkout.
- Fetch the PR base and head with the same resolved `owner/repo`. Do not work directly on `main`, and do not attach the fix to a branch for a different PR.

### 3. Implement The Fix

- Treat the GitHub comment as the scope boundary. Fix the requested issue and directly related sibling instances; avoid drive-by refactors.
- Run the *tdd* skill for code changes when practical: write one focused failing test or reproduction first, make it pass, then refactor.
- If a test-first path is not practical, record why and use the narrowest validation that proves the comment is addressed.
- Ask the user instead of guessing when the requested behavior is ambiguous, stale, or conflicts with existing requirements.
- In deferred fix-pr orchestration mode, stop after this local implementation and focused proof; do not continue to autoreview, ship, reply, or resolve threads.

### 4. Verify And Autoreview

- Run focused tests, linters, typechecks, or builds that cover the change.
- Run the *autoreview* skill on the change before shipping. For dirty local work, use the autoreview local mode; for already committed work, use branch or commit mode with the PR base.
- Pass the comment URL and a short context note into autoreview when useful, for example with a prompt file.
- Fix accepted/actionable autoreview findings and rerun the affected tests and autoreview until it exits cleanly. Do not ship with failing required checks or unresolved accepted findings unless the user explicitly overrides.

### 5. Ship To The Existing PR

- Run the *ship* skill to commit and push the fix to the PR branch. Do not create a new PR for an autofix comment, and do not edit the existing PR title or description unless the user asks.
- Keep the commit message specific to the comment-driven fix.
- If the *ship* workflow requires a PR comment for newly pushed commits, make the original comment or thread reply carry that detailed update when possible rather than posting duplicate status comments.

### 6. Reply To And Resolve The Thread

- Reply only after the fix is pushed.
- For review threads, reply in the original thread using GraphQL `addPullRequestReviewThreadReply` or the review-comment reply endpoint.
- Resolve the review thread with GraphQL `resolveReviewThread` once the reply confirms the fix, tests run, and clean autoreview result.
- For flat PR comments that are not part of a review thread, add a PR comment that links back to the original comment. Flat comments cannot be resolved on GitHub; report that limitation in the final output.
- Include a concise summary of the fix, the commit or pushed branch, tests run, and the clean autoreview result. If something could not be verified, state that plainly.
- Do not reply as "fixed" or resolve a thread when the finding was not actually addressed in the pushed commits.

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

Tell the user:

- Which `--comment` URL or `--pr` target was addressed.
- The classification and severity table for PR mode, or the single comment addressed in comment mode.
- What changed and where.
- Which tests and autoreview command ran.
- What was pushed to the PR.
- Where you replied on GitHub and which review threads were resolved, if applicable.
- Which P2/P3 or non-auto-fixable findings remain open.
- For deferred fix-pr orchestration mode, report changed files, tests/proof run, the original comment/thread URL, dirty-worktree status, and any blockers instead of autoreview, ship, or reply evidence.
