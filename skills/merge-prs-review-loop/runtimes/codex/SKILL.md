---
name: merge-prs-review-loop
description: Review and merge multiple pull requests with an iterative reviewer loop while minimizing conflicts. Use when the user asks to merge several PRs, land a PR batch, integrate stacked or overlapping PRs, choose a merge order, resolve conflicts safely, or combine review-loop quality gates with GitHub PR merging.
---

# Merge PRs With Review Loop

Use this workflow to land multiple PRs with explicit quality gates, a conflict-minimizing order, and verification after each merge. This skill composes with the *review-loop* skill: use *review-loop* for isolated PR reviews and final integration review, while this skill owns PR ordering, merge safety, conflict handling, and Git/GitHub execution.

## GitHub Access

Prefer an installed GitHub connector for PR metadata, comments, reviews, patches, and repository state when available. Use `gh` for merge operations, checks/log gaps, merge queue behavior, branch protection details, local checkout gaps, or any connector coverage gaps.

## Defaults

- Quality gate: `8/10`
- Minimum review loops: `2`
- Maximum review loops: `4`
- Merge strategy: preserve project history and repo conventions; prefer an integration branch for local conflict resolution, and use the repository's normal PR merge flow only when the user has allowed remote merges.
- Verification: use the repo's documented format, test, lint, build, and CI commands.

Honor user overrides for PR list, order, merge method, quality gate, loop count, branch names, or whether pushing/merging is allowed. If the user asks only for a merge plan or dry run, do not merge or push.

## Review-Loop Composition

Use the actual *review-loop* workflow for critique loops. Do not replace it with an ad hoc self-review.

- Load and follow *review-loop* defaults unless the user overrides them here.
- Pass this skill's quality gate, minimum loops, and maximum loops into each *review-loop* invocation.
- Use *review-loop* in review-existing mode for each PR before it is eligible to merge.
- Use *review-loop* again on the combined preflight result before any remote PR merge or before pushing/opening a local integration branch, then confirm the landed remote state afterward when using remote PR merges.
- Give *review-loop* only the bounded work product: PR metadata, diffs, relevant file excerpts, verification results, prior review findings, and task-specific criteria. Do not include private reasoning.
- Keep merge control in the main agent. The review loop may identify findings and proposed fixes, but this skill decides whether a PR branch, local integration branch, or remote PR merge path is safe.

When delegating review, spawn Codex subagents only when the user explicitly asks for delegation or parallel agent work and the current surface exposes a documented safe mechanism. If no safe mechanism is available, run the review loop inline and state that no separate reviewer was used.

## 1. Establish Safety And Inputs

1. Read project instructions and discover test/build commands from repo docs, CI, and manifests.
2. Check current branch, remote, dirty state, and default/base branch:
   - `git status --short --branch`
   - `git remote -v`
   - connector metadata reads or `gh repo view --json defaultBranchRef,mergeCommitAllowed,squashMergeAllowed,rebaseMergeAllowed,viewerDefaultMergeMethod,viewerPermission`
   - connector PR reads or `gh pr view <number> --json number,title,state,isDraft,baseRefName,baseRefOid,headRefName,headRefOid,headRepository,headRepositoryOwner,isCrossRepository,maintainerCanModify,mergeStateStatus,statusCheckRollup,files,commits`
3. Protect unrelated work:
   - Do not stage or revert unrelated dirty/untracked files.
   - Stop and ask only when unrelated tracked changes block checkout, merge, or verification.
   - Prefer creating a new integration branch instead of working directly on the default branch unless the user explicitly requested the default branch.
4. Fetch and sync the base branch before planning merges:
   - `git fetch --prune`
   - fast-forward the local base branch when clean and allowed.
5. Exclude PRs that are closed, already merged, drafts, target a different base branch, or have failed/pending required checks from merge eligibility. A user override may include them in planning or review, but they must not be merged until required checks pass or repo policy confirms the checks are non-required.
6. For a draft PR explicitly included by the user, confirm whether to skip it or mark it ready first; do not merge it while it is still draft.

## 2. Map Conflict Risk

Before merging, build a concrete conflict map.

- List changed files for every PR.
- Identify overlapping files and likely semantic conflicts.
- Ensure each PR head commit is locally available. If `git cat-file -e <headRefOid>^{commit}` fails, fetch the PR head with a repo-appropriate refspec; for a GitHub `origin` remote, use `git fetch origin pull/<number>/head:refs/tmp/pr-<number>`.
- Simulate pairwise and ordered merges without changing the worktree when possible:
  - `git merge-tree --write-tree <current-integration-commit> <pr-head-commit>`
  - `git merge-tree --write-tree --merge-base <merge-base> <current-integration-commit> <pr-head-commit>`
- Prefer an order that lands clean, foundational, or low-overlap PRs first; defers the largest overlapping PR until after dependencies; resolves each conflict cluster once; and keeps independent PRs separated from risky integration work.

If the best order differs from the user's requested order, explain the reason briefly unless the user explicitly fixed the order.

## 3. Review Each PR In Isolation

Run *review-loop* for each PR before merging. Treat a PR as ineligible to merge until its review-loop result satisfies the configured gate, or until the user explicitly accepts the below-gate risk.

Reviewer criteria should include correctness, edge cases, backward compatibility, tests for new behavior and regressions, error handling, safety boundaries, and documentation or UI copy alignment when user-facing behavior changes.

If review-loop findings are false positives, document the evidence from code or tests before ignoring them. If the score is below the quality gate or findings are blocking, require the fix to land on the PR branch for normal PR merging, or make an integration-only fix on a local integration branch only when that path is chosen and permitted.

Never merge a PR that finishes below the quality gate unless the user explicitly accepts the risk.

## 4. Merge Sequentially

Choose one merge path before starting, then keep it consistent unless a conflict or user override requires switching.

- Remote PR-by-PR landing path:
  - Before any remote merge, create the chosen ordered integration state in a temporary local branch or worktree and run the final integration review loop against that preflight state.
  - Review the PR, verify required checks, re-check the head SHA, and use `gh pr merge <number> --match-head-commit <headRefOid> --merge`, `--squash`, or `--rebase` according to the repo's allowed/default method and the user's instruction only after the preflight integration review passes.
  - If the base branch requires a merge queue, let `gh pr merge` enqueue the PR, never use `--admin`, and treat queued PRs as not yet landed until GitHub reports the merge complete.
  - After each remote merge, update the local base branch before reviewing or merging the next PR.
- Local integration branch path:
  - Create a named integration branch from the synced base.
  - Fetch PR heads, merge them locally in the chosen order, resolve conflicts, and run final integration review before any push.
  - Push, open, or merge the integration branch only when the user and repository policy allow it.

Immediately before each merge, re-check PR state, draft status, base/head SHAs, mergeability, review decision, and required checks. If the base or head changed, rebuild the preflight state and rerun the relevant review before merging.

If conflicts occur, inspect all conflict markers, resolve by preserving both PR intents, search for related tests and definitions that must stay consistent, run formatters only on touched files or as documented, and stage only files that belong to the merge or conflict resolution.

Run focused tests for touched subsystems after each merge, and full repo tests when merge risk is moderate or high.

## 5. Final Integration Review

Run at least one final review loop focused on combined behavior before any remote PR merge or before pushing/opening a local integration branch, and rerun it whenever the reviewed base or head changes.

Review criteria should cover interactions between features from different PRs, conflict-resolution files, public APIs, config, schema, migrations, UI states, stale async handling, ordering, state transitions, safety gates, documentation consistency, and adequacy of integration tests.

Address actionable findings before pushing unless they are explicitly out of scope or disproven by code/tests.

## 6. Final Verification And Push

Before pushing or marking done:

1. Run the final documented checks: formatter check, full test suite, build, and lint/typecheck when documented.
2. Check status and confirm only intended changes are present.
3. Push or complete GitHub merges only as allowed by the user and the repository's branch protection. If permission is ambiguous, stop after local verification and report the exact next command that would be run. If branch protection blocks the merge, leave the PR unmerged and report the blocker instead of using an admin bypass.
4. Confirm PR states and remote CI with connector reads or:
   - `gh pr view <number> --json number,state,mergedAt,mergeCommit`
   - `gh run list --branch <base> --limit 5`
5. For queued PRs, continue checking until GitHub reports a merge commit or report that the PR remains queued.

## Report

Summarize merge order, final base commit, review-loop scores, fixes made, conflicts resolved, local checks, remote CI results, remaining unrelated dirty files or warnings, and whether PRs are merged, pushed, or still awaiting user action.

## Guardrails

- Never hide failed checks or unresolved reviewer findings.
- Never force-push unless the user explicitly requested it.
- Never revert unrelated user work.
- Never merge draft PRs by default; if the user explicitly includes one, confirm whether to skip it or mark it ready first.
- Never bypass branch protection, admin-merge, merge queue, or required-review gates.
- Prefer exact command output summaries over vague claims.
