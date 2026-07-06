---
name: merge-prs-review-loop
description: Review and merge multiple pull requests with an iterative reviewer loop while minimizing conflicts. Use when the user asks to merge several PRs, land a PR batch, integrate stacked or overlapping PRs, choose a merge order, resolve conflicts safely, or combine review-loop quality gates with GitHub PR merging.
---

# Merge PRs With Review Loop

Use this workflow to land multiple PRs with explicit quality gates, a conflict-minimizing order, and verification after each merge. This skill composes with the *review-loop* skill: use *review-loop* for isolated PR reviews and final integration review, while this skill owns PR ordering, merge safety, conflict handling, and Git/GitHub execution.

## GitHub Access

Use `gh` for repository, PR, review, check, merge, queue, and branch-protection operations unless the user provides another GitHub integration.

## Defaults

- Quality gate: `8/10`
- Minimum review loops: `2`
- Maximum review loops: `4`
- Merge strategy: preserve project history and repo conventions; prefer an integration branch for local conflict resolution, and use the repository's normal PR merge flow only when the user has allowed remote merges.
- Verification: use the repo's documented format, test, lint, build, and CI commands.

Honor user overrides for PR list, order, merge method, quality gate, loop count, branch names, or whether pushing/merging is allowed. If the user asks only for a merge plan or dry run, do not merge or push.

## Review-Loop Composition

Use the actual *review-loop* workflow for critique loops. Spawn reviewer agents through the available native delegation path. Independent PR reviews may run in parallel after conflict-risk mapping.

- Load and follow *review-loop* defaults unless the user overrides them here.
- Pass this skill's quality gate, minimum loops, and maximum loops into each *review-loop* invocation.
- Use *review-loop* in review-existing mode for each PR before it is eligible to merge.
- Use *review-loop* again on the combined preflight result before any remote PR merge or before pushing/opening a local integration branch, then confirm the landed remote state afterward when using remote PR merges.
- Give *review-loop* only the bounded work product: PR metadata, diffs, relevant file excerpts, verification results, prior review findings, and task-specific criteria. Do not include private reasoning.
- Keep merge control in the main agent. The review loop may identify findings and proposed fixes, but this skill decides whether a PR branch, local integration branch, or remote PR merge path is safe.

## 1. Establish Safety And Inputs

1. Read project instructions and discover test/build commands from repo docs, CI, and manifests.
2. Check current branch, remote, dirty state, and default/base branch:
   - `git status --short --branch`
   - `git remote -v`
   - `gh repo view --json defaultBranchRef,mergeCommitAllowed,squashMergeAllowed,rebaseMergeAllowed,viewerDefaultMergeMethod,viewerPermission`
   - `gh pr view <number> --json number,title,state,isDraft,baseRefName,baseRefOid,headRefName,headRefOid,headRepository,headRepositoryOwner,isCrossRepository,maintainerCanModify,mergeStateStatus,statusCheckRollup,files,commits`
3. Protect unrelated work: do not stage or revert unrelated files; stop only when unrelated changes block checkout, merge, or verification.
4. Fetch and sync the base branch with `git fetch --prune`, then fast-forward the local base branch when clean and allowed.
5. Exclude PRs that are closed, already merged, drafts, target a different base branch, or have failed/pending required checks from merge eligibility.

## 2. Map Conflict Risk

List changed files for every PR, identify overlapping files and semantic conflicts, ensure each PR head commit is locally available, and simulate pairwise and ordered merges with `git merge-tree` when possible. Prefer an order that lands clean foundational PRs first, defers large overlap until after dependencies, resolves each conflict cluster once, and keeps independent PRs separated from risky integration work.

## 3. Review Each PR In Isolation

Run *review-loop* for each PR before merging. Treat a PR as ineligible to merge until its review-loop result satisfies the configured gate, or until the user explicitly accepts the below-gate risk.

Reviewer criteria should include correctness, edge cases, backward compatibility, tests, error handling, safety boundaries, and documentation or UI copy alignment when user-facing behavior changes.

If findings are false positives, document evidence from code or tests before ignoring them. If findings are blocking, require the fix to land on the PR branch for normal PR merging, or make an integration-only fix on a local integration branch only when that path is chosen and permitted.

## 4. Merge Sequentially

Choose a remote PR-by-PR path or a local integration branch path before starting.

- For remote PR-by-PR landing, build a temporary local preflight integration state and run the final integration review loop before remote merges. Then verify required checks, re-check the head SHA, and use `gh pr merge <number> --match-head-commit <headRefOid>` with the repo's allowed merge method. For merge queues, let `gh pr merge` enqueue the PR and do not use admin bypass.
- For local integration, create a named branch from the synced base, fetch PR heads, merge in the chosen order, resolve conflicts, and run final integration review before any push.

Immediately before each merge, re-check PR state, draft status, base/head SHAs, mergeability, review decision, and required checks. If the base or head changed, rebuild the preflight state and rerun the relevant review.

If conflicts occur, inspect all markers, preserve both PR intents, search for related tests and definitions, run formatters only on touched files or as documented, and stage only merge/conflict-resolution files.

Run focused tests for touched subsystems after each merge, and full repo tests when merge risk is moderate or high.

## 5. Final Integration Review

Run at least one final review loop focused on combined behavior before any remote PR merge or before pushing/opening a local integration branch, and rerun it whenever the reviewed base or head changes. Address actionable findings before pushing unless they are explicitly out of scope or disproven by code/tests.

## 6. Final Verification And Push

Run final documented checks, confirm status contains only intended changes, then push or complete GitHub merges only as allowed by the user and repository policy. Confirm PR states and remote CI with `gh pr view` and `gh run list`. For queued PRs, continue checking until GitHub reports a merge commit or report that the PR remains queued.

## Report

Summarize merge order, final base commit, review-loop scores, fixes made, conflicts resolved, local checks, remote CI results, remaining unrelated dirty files or warnings, and whether PRs are merged, pushed, or still awaiting user action.

## Guardrails

- Never hide failed checks or unresolved reviewer findings.
- Never force-push unless the user explicitly requested it.
- Never revert unrelated user work.
- Never merge draft PRs by default; if the user explicitly includes one, confirm whether to skip it or mark it ready first.
- Never bypass branch protection, admin-merge, merge queue, or required-review gates.
- Prefer exact command output summaries over vague claims.
