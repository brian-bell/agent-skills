---
name: merge-prs-review-loop
description: Review and merge multiple pull requests with an iterative reviewer loop while minimizing conflicts. Use when the user asks to merge several PRs, land a PR batch, integrate stacked or overlapping PRs, choose a merge order, resolve conflicts safely, or combine review-loop quality gates with GitHub PR merging.
---

# Merge PRs With Review Loop

Use this workflow to land multiple PRs with explicit quality gates, a conflict-minimizing order, and verification after each merge. This skill composes with `$review-loop`: use `$review-loop` for the isolated PR reviews and the final integration review, while this skill owns PR ordering, merge safety, conflict handling, and Git/GitHub execution.

## Defaults

- Quality gate: `8/10`
- Minimum review loops: `2`
- Maximum review loops: `4`
- Merge strategy: preserve project history and repo conventions; prefer an integration branch for local conflict resolution, and use the repository's normal PR merge flow only when the user has allowed remote merges.
- Verification: use the repo's documented format, test, lint, build, and CI commands.

Honor user overrides for PR list, order, merge method, quality gate, loop count, branch names, or whether pushing/merging is allowed. If the user asks only for a merge plan or dry run, do not merge or push.

## Review-Loop Composition

This skill must use the actual `$review-loop` workflow for critique loops. Do not replace it with an ad hoc self-review.

- Load and follow `$review-loop` defaults unless the user overrides them here.
- Pass this skill's quality gate, minimum loops, and maximum loops into each `$review-loop` invocation.
- Use `$review-loop` in review-existing mode for each PR before it is eligible to merge.
- Use `$review-loop` again on the combined preflight result before any remote PR merge or before pushing/opening a local integration branch, then confirm the landed remote state afterward when using remote PR merges.
- Give `$review-loop` only the bounded work product: PR metadata, diffs, relevant file excerpts, verification results, prior review findings, and task-specific criteria. Do not include private reasoning.
- Keep merge control in the main agent. `$review-loop` may identify findings and proposed fixes, but this skill decides whether a PR branch, local integration branch, or remote PR merge path is safe.

## 1. Establish Safety And Inputs

1. Read project instructions and discover test/build commands from repo docs, CI, and manifests.
2. Check current branch, remote, dirty state, and default/base branch:
   - `git status --short --branch`
   - `git remote -v`
   - `gh repo view --json defaultBranchRef,mergeCommitAllowed,squashMergeAllowed,rebaseMergeAllowed,viewerDefaultMergeMethod,viewerPermission`
   - `gh pr view <number> --json number,title,state,isDraft,baseRefName,baseRefOid,headRefName,headRefOid,headRepository,headRepositoryOwner,isCrossRepository,maintainerCanModify,mergeStateStatus,statusCheckRollup,files,commits`
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
- Prefer an order that:
  - Lands clean, foundational, or low-overlap PRs first.
  - Defers the largest overlapping PR until after its dependencies.
  - Resolves each conflict cluster once.
  - Keeps independent PRs separated from risky integration work.

If the best order differs from the user's requested order, explain the reason briefly unless the user explicitly fixed the order.

## 3. Review Each PR In Isolation

Run `$review-loop` for each PR before merging. Treat a PR as ineligible to merge until its `$review-loop` result satisfies the configured gate, or until the user explicitly accepts the below-gate risk.

For each PR-level `$review-loop`:
1. Invoke `$review-loop` in review-existing mode with the PR title, base/head SHAs, diff or file list, relevant tests, prior loop findings, and task-specific criteria.
2. Require the `$review-loop` report to include score, loop count, actionable findings, and file/line references where applicable.
3. If `$review-loop` findings are false positives, document the evidence from code or tests before ignoring them.
4. If the `$review-loop` score is below the quality gate or findings are blocking:
   - For normal `gh pr merge` landing, the fix must land on the PR branch or the PR remains unmerged.
   - For local integration branch landing, merge the PR head locally, make any integration-only fix on the integration branch, verify it there, and push/open/merge that integration branch only with permission.
   - Require explicit user permission before pushing fixes to a contributor PR branch, even when `maintainerCanModify` is true.
5. Let `$review-loop` apply its stop order exactly:
   - If loops completed is below the minimum, continue even if the score already meets the gate.
   - If the minimum is met and the score is at or above the quality gate, proceed.
   - If the maximum loop count is reached below the quality gate, stop and report the blocker.
   - Otherwise, revise and review again.
6. Never merge a PR that finishes below the quality gate unless the user explicitly accepts the risk.

Reviewer criteria should include:
- Correctness and edge cases for the PR's behavior.
- Backward compatibility with the base branch.
- Tests for new behavior and regressions.
- Error handling and safety boundaries.
- Documentation or UI copy alignment when user-facing behavior changes.

Do not use subagents for merge conflict resolution unless the work can be split into disjoint files. The main agent owns the final integration.

## 4. Merge Sequentially

Choose one merge path before starting, then keep it consistent unless a conflict or user override requires switching.

- Remote PR-by-PR landing path:
  - Before any remote merge, create the chosen ordered integration state in a temporary local branch or worktree and run the final integration `$review-loop` against that preflight state. Use `git merge-tree` simulation only for conflict-risk mapping, not for final review that depends on tests, generated files, or line-level inspection.
  - Review the PR, verify required checks, re-check the head SHA, and use `gh pr merge <number> --match-head-commit <headRefOid> --merge`, `--squash`, or `--rebase` according to the repo's allowed/default method and the user's instruction only after the preflight integration review passes.
  - If the base branch requires a merge queue, do not pass a merge strategy unless the CLI/repo requires one; let `gh pr merge` enqueue the PR, never use `--admin` to bypass the queue, and treat queued PRs as not yet landed until GitHub reports the merge complete.
  - After each remote merge, update the local base branch before reviewing or merging the next PR.
  - Do not use this path for a PR with blocking findings unless the fix is already on that PR branch.
- Local integration branch path:
  - Create a named integration branch from the synced base.
  - Fetch PR heads, merge them locally in the chosen order, resolve conflicts, and run final integration review before any push.
  - Push, open, or merge the integration branch only when the user and repository policy allow it.

For each PR in the chosen path:

1. Re-check PR state, mergeability, and checks immediately before merge:
   - `gh pr view <number> --json state,isDraft,baseRefOid,headRefOid,mergeStateStatus,reviewDecision`
   - `gh pr checks <number> --required --json name,state,bucket,workflow,link`
   - Confirm the current `baseRefOid` still matches the base commit used for the preflight integration review. If the base changed, rebuild the preflight state and rerun the final integration review before merging.
   - Confirm the current `headRefOid` still matches the head commit that was reviewed and use `gh pr merge --match-head-commit <headRefOid>` for remote merges.
   - Treat `CHANGES_REQUESTED` or `REVIEW_REQUIRED` as blockers when required reviews are part of repository policy or branch protection.
   - Treat any required check with `bucket` equal to `fail`, `pending`, `cancel`, or `skipping` as a blocker unless the repo explicitly treats it as non-required.
   - Report optional check failures separately and merge only when the repo's policy allows it.
   - If checks are still running, wait or watch when practical instead of merging early.
2. Execute the selected merge path:
   - For normal protected-branch landing, use `gh pr merge <number> --match-head-commit <headRefOid> --merge`, `--squash`, or `--rebase` according to the repo's allowed/default method and the user's instruction.
   - For merge-queue repositories, let `gh pr merge` enqueue the PR according to GitHub's requirements, then wait for or report the queued state instead of claiming the PR landed immediately.
   - For local integration or conflict resolution, merge into a named integration branch. Prefer `git merge --no-ff <pr-head-commit>` when preserving PR boundaries helps later review; otherwise follow the repo's merge convention.
3. If conflicts occur:
   - Inspect all conflict markers.
   - Resolve by preserving both PR intents, not by mechanically choosing one side.
   - Search for related tests and message/types/API definitions that must stay consistent.
   - Run formatters only on touched files or as documented by the repo.
4. Stage only files that belong to the merge or conflict resolution.
5. Commit the merge or complete the PR merge.
6. Run verification after each merge:
   - Focused tests for touched subsystems.
   - Full repo tests when merge risk is moderate or high.
   - Build when the repo has a documented build command.

If a local integration merge reveals an unexpected semantic issue, fix it in the integration branch and document why. If a remote PR-by-PR preflight reveals an issue, stop before merging and require the fix to land on the relevant PR branch or switch to a local integration branch.

## 5. Final Integration Review

Run at least one final `$review-loop` focused on combined behavior before any remote PR merge or before pushing/opening a local integration branch, and rerun it whenever the reviewed base or head changes. After a remote PR-by-PR path completes, sync the local base and confirm the landed state still matches the reviewed integration intent.

- For the remote PR-by-PR path, review the local preflight integration state first, then sync the local base to the latest remote base after all merges and confirm the combined base state.
- For the local integration branch path, review the integration branch before pushing or opening it.

Review criteria should cover:
- Interactions between features from different PRs.
- Conflict-resolution files.
- Public APIs, config, schema, migrations, or UI states changed by multiple PRs.
- Stale async handling, ordering, state transitions, or safety gates when relevant.
- Documentation consistency and roadmap/checklist updates.
- Adequacy of integration tests.

Address actionable findings before pushing unless they are explicitly out of scope or disproven by code/tests.

## 6. Final Verification And Push

Before pushing or marking done:

1. Run the final documented checks, typically:
   - formatter check
   - full test suite
   - build
   - lint/typecheck when documented
2. Check status and confirm only intended changes are present.
3. Push or complete GitHub merges only as allowed by the user and the repository's branch protection. If permission is ambiguous, stop after local verification and report the exact next command that would be run. If branch protection blocks the merge, leave the PR unmerged and report the blocker instead of using an admin bypass.
4. Confirm PR states and remote CI:
   - `gh pr view <number> --json number,state,mergedAt,mergeCommit`
   - `gh run list --branch <base> --limit 5`
   - For queued PRs, continue checking until GitHub reports a merge commit or report that the PR remains queued.
   - Watch the relevant run when practical.
5. Close or clean up any reviewer subagents/threads.

## Report

Summarize:

- Merge order and final base commit.
- Review-loop scores and any fixes made.
- Conflicts resolved and the files involved.
- Local checks and remote CI results.
- Remaining unrelated dirty files or warnings.
- Whether PRs are merged, pushed, or still awaiting user action.

## Guardrails

- Never hide failed checks or unresolved reviewer findings.
- Never force-push unless the user explicitly requested it.
- Never revert unrelated user work.
- Never merge draft PRs by default; if the user explicitly includes one, confirm whether to skip it or mark it ready first.
- Never bypass branch protection, admin-merge, merge queue, or required-review gates.
- Prefer exact command output summaries over vague "looks good" claims.
