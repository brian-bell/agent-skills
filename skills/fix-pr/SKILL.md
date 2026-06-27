---
name: fix-pr
description: Gather unresolved GitHub pull request review comments, classify each finding as accepted, rejected, or already fixed, and display a report without replying to comments, resolving review threads, or otherwise mutating GitHub comment state. Use when the user asks to fix, triage, review, or report on unresolved PR comments or invokes $fix-pr, with optional repo and PR number inputs.
---

# Fix PR

Gather unresolved GitHub PR review threads, evaluate each comment against the current code, and display a decision report. Do not reply to GitHub comments or resolve GitHub review threads.

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

For `accepted` findings, state the concrete local change needed. Do not implement changes unless the user explicitly asks for implementation in addition to the report.

For `already fixed` and `rejected` findings, cite specific evidence such as file paths, line numbers, test output, PR diff context, or requirements.

### 4. Display The Report

Display a report with:

- PR repo, number, title, and URL.
- Total unresolved thread count.
- One entry per unresolved thread, preserving thread URL and location.
- Decision: `accepted`, `already fixed`, or `rejected`.
- Evidence and reasoning.
- Local action needed, or `none`.

Prefer this shape:

```text
Status          Location          Reviewer        Finding
accepted        src/foo.go:42      @reviewer      Missing nil check before dereference
already fixed   src/bar.go:18      @reviewer      Guard exists in current diff
rejected        docs/api.md:7      @reviewer      Requested behavior conflicts with ADR-003
```

Follow the table with concise per-thread notes when evidence will not fit in one line.

## Hard Rules

- Do not reply to GitHub comments.
- Do not resolve or unresolve GitHub review threads.
- Do not submit, approve, request changes, dismiss, edit, or delete GitHub reviews.
- Do not run GraphQL mutations such as `resolveReviewThread`, `unresolveReviewThread`, or `addPullRequestReviewThreadReply`.
- Do not use `gh pr comment`, `gh pr review`, `gh issue comment`, or any equivalent write operation.
- Surface exact `gh` or test errors if collection or validation fails.
