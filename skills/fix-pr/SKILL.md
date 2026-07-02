---
name: fix-pr
description: Gather unresolved GitHub pull request review comments, classify each finding as accepted, rejected, or already fixed, ask whether to run autofix for all or selected accepted findings, then run aggregate autoreview and ship to the existing PR after approval. Use when the user asks to fix, triage, review, or report on unresolved PR comments, with optional repo and PR number inputs.
---

# Fix PR

Gather unresolved GitHub PR review threads, evaluate each comment against the current code, classify each finding, ask whether to run the *autofix* skill for accepted findings, then aggregate-review and ship approved fixes to the existing PR. Do not reply to GitHub comments or resolve GitHub review threads directly from this skill.

On Codex, prefer an installed GitHub connector when available; use `gh` when connector coverage is insufficient or unavailable. On Claude Code, use `gh`/CLI unless the user provides another integration.

## Inputs

- `--repo <owner/repo>`: target repository. If omitted, infer the current repo with `gh repo view`.
- `--pr <number>`: target pull request. If omitted, infer the PR for the current branch with `gh pr view`.

## Workflow

### 1. Collect Unresolved Comments

Run the bundled read-only collector:

```bash
python3 <skill-dir>/scripts/gather_unresolved_pr_comments.py --repo <owner/repo> --pr <number> --format markdown
```

Use `--format json` when structured data is easier to process. The collector uses `gh repo view`, `gh pr view`, and `gh api graphql` queries only. It does not send GraphQL mutations, comments, reviews, or thread-resolution requests.

If the collector cannot be used, gather the same data with read-only GitHub commands and include all unresolved PR review threads, paging through results until complete.

### 2. Inspect The Code

For every unresolved thread:

- Read the referenced file and nearby code.
- Compare the comment with the current PR diff and current working tree.
- Run focused tests only when needed to determine whether the finding is still valid.
- Preserve the reviewer's intent; do not dismiss a comment because the wording is terse.

### 3. Classify Each Finding

Use exactly one decision per thread:

- `accepted`: the finding is valid and still needs a code, test, docs, or behavior change.
- `already fixed`: the finding was valid, but the current code or tests already address it.
- `rejected`: the finding is incorrect, obsolete, out of scope, or conflicts with project requirements.

For `accepted` findings, state the concrete local change needed and keep the thread URL available for the autofix handoff.

For `already fixed` and `rejected` findings, cite specific evidence such as file paths, line numbers, test output, PR diff context, or requirements.

### 4. Ask Whether To Use Autofix On Accepted Findings

If there are no `accepted` findings, display the report and stop.

Before asking for autofix approval, display the classified findings with each accepted finding's URL, evidence, and pending local action.

Ask the user whether to run the *autofix* skill for all accepted findings or a selected subset before mutating the PR.

If the user declines or does not explicitly approve, do not run autofix; report the pending autofix commands/actions and stop.

For each approved `accepted` finding, run the *autofix* skill in deferred mode for fix-pr orchestration with --comment <comment-or-thread URL>. Invoke the skill with `--comment <comment-or-thread URL>` and include the deferred-mode handoff phrase in the task context; do not invent a new autofix CLI flag.

```text
autofix --comment <comment-or-thread URL>
```

Pass the accepted row's finding, evidence, and local action as context for the autofix run. Do not implement accepted findings directly in `fix-pr`; accepted PR-review-comment fixes are delegated to the *autofix* skill.

Process accepted findings one at a time so each autofix remains scoped to a single review thread. If an accepted finding cannot be safely delegated because the thread URL is missing, stale, ambiguous, or requires broad product judgment, stop before editing and report the blocker. If a later approved finding blocks after earlier deferred fixes succeeded, report completed fixes and the blocker, then ask before continuing to aggregate autoreview/ship the partial set. If the user declines partial review/ship, leave local uncommitted changes intact, do not revert, do not ship, and report exact changed files/proof/blockers.

### 5. Run Aggregate Review And Ship

After all approved deferred autofix runs complete, run the *autoreview* skill in local/dirty mode on the aggregate resulting uncommitted diff. Accepted/actionable aggregate autoreview findings are handled as ordinary local code fixes within the active orchestrated workflow, not as direct GitHub review-comment mutation by fix-pr. Rerun affected tests and the *autoreview* skill until autoreview exits clean with no accepted/actionable findings.

Do not run the *ship* skill until autoreview exits clean with no accepted/actionable findings.

Before invoking the *ship* skill, verify the current checkout still targets the original PR: same repo, PR number, PR URL, and head branch/ref. Stop before calling the *ship* skill if no existing PR is found, the current branch maps to a different PR, or PR identity is ambiguous.

Run the *ship* skill to push the reviewed fixes to the existing PR. The handoff context to the *ship* skill must include `existing PR only; stop rather than create`. Do not create a new PR or rewrite the existing PR description from `fix-pr`.

The deferred fix-pr path does not reply to original review threads or resolve them. If the *ship* skill posts its normal existing-PR summary comment, report that separately from original review-thread replies.

### 6. Display The Report

Display the report as a Markdown table. Include:

- PR repo, number, title, and URL.
- Total unresolved thread count.
- One entry per unresolved thread, preserving thread URL and location.
- Decision: `accepted`, `already fixed`, or `rejected`.
- Evidence and reasoning.
- Autofix result, pending autofix command, local action needed, or consent/approval state for `accepted` findings, and `none` for other decisions.
- For approved findings, include selected, declined, partial, blocked, review-clean, PR-verified, shipped, and no-thread-reply states when they apply.

Use this table shape:

```text
| Decision | Location | Reviewer | Finding | Evidence | Action | URL |
|---|---|---|---|---|---|---|
| accepted | src/foo.go:42 | @reviewer | Missing nil check before dereference | Current code indexes before checking length | Handed to the *autofix* skill with `--comment https://github.com/...` | https://github.com/... |
| already fixed | src/bar.go:18 | @reviewer | Guard missing | Guard exists in current diff and focused test passes | none | https://github.com/... |
| rejected | docs/api.md:7 | @reviewer | Change public behavior | Conflicts with ADR-003 compatibility requirement | none | https://github.com/... |
```

Keep the table concise. If a finding needs more detail than fits cleanly, keep the table row and add a short `Notes` section after the table keyed by location or thread URL.

## Hard Rules

- Do not reply to GitHub comments directly from `fix-pr`.
- Do not resolve or unresolve GitHub review threads directly from `fix-pr`.
- Do not submit, approve, request changes, dismiss, edit, or delete GitHub reviews directly from `fix-pr`.
- Do not run GraphQL mutations such as `resolveReviewThread`, `unresolveReviewThread`, or `addPullRequestReviewThreadReply` directly from `fix-pr`.
- Do not use `gh pr comment`, `gh pr review`, `gh issue comment`, or any equivalent write operation directly from `fix-pr`.
- The only permitted mutation path for accepted findings is delegated skill workflow after explicit post-classification approval.
- Surface exact `gh` or test errors if collection or validation fails.
