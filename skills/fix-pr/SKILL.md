---
name: fix-pr
description: Gather unresolved GitHub pull request review comments, classify each finding as accepted, rejected, or already fixed, and use $autofix on accepted findings. Use when the user asks to fix, triage, review, or report on unresolved PR comments or invokes $fix-pr, with optional repo and PR number inputs.
---

# Fix PR

Gather unresolved GitHub PR review threads, evaluate each comment against the current code, classify each finding, and delegate accepted findings to `$autofix`. Do not reply to GitHub comments or resolve GitHub review threads directly from this skill.

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

### 4. Use Autofix On Accepted Findings

For each `accepted` finding, use:

```text
$autofix --comment <thread URL>
```

Pass the accepted row's finding, evidence, and local action as context for the autofix run. Do not implement accepted findings directly in `fix-pr`; `$autofix` owns the code change, focused tests, autoreview, shipping, and reply to the original GitHub comment or thread.

Process accepted findings one at a time so each autofix remains scoped to a single review thread. If an accepted finding cannot be safely delegated because the thread URL is missing, stale, ambiguous, or requires broad product judgment, stop before editing and report the blocker.

### 5. Display The Report

Display the report as a Markdown table. Include:

- PR repo, number, title, and URL.
- Total unresolved thread count.
- One entry per unresolved thread, preserving thread URL and location.
- Decision: `accepted`, `already fixed`, or `rejected`.
- Evidence and reasoning.
- Autofix result or local action needed for `accepted` findings, and `none` for other decisions.

Use this table shape:

```text
| Decision | Location | Reviewer | Finding | Evidence | Action | URL |
|---|---|---|---|---|---|---|
| accepted | src/foo.go:42 | @reviewer | Missing nil check before dereference | Current code indexes before checking length | Handed to `$autofix --comment https://github.com/...` | https://github.com/... |
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
- The only permitted mutation path for accepted findings is the delegated `$autofix --comment <thread URL>` workflow.
- Surface exact `gh` or test errors if collection or validation fails.
