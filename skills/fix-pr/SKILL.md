---
name: fix-pr
description: Watch a GitHub pull request for reviewer reactions and Codex/Claude feedback, then auto-fix only P0 and P1 findings by delegating to the *autofix* skill. Use when the user asks for a poll-and-fix loop on a single PR, asks to wait for review feedback and then resolve actionable P0/P1 findings, or invokes fix-pr on a PR number or URL.
---

# Fix PR

Use this skill when the user wants a long-running watcher on a single pull
request that turns reviewer feedback into scoped P0/P1 fixes.

Provide exactly one target:

```text
--pr <number-or-pull-request-url> [--repo <owner/repo>]
```

- `--pr`: pull request number, or a GitHub PR URL.
- `--repo`: required when `--pr` is a number and the repo is not the current
  checkout.

## Loop

Run this skill as a single conversational loop until a terminal verdict
appears.

1. **Resolve the PR.** Parse `owner`, `repo`, and PR number from `--pr` and
   optional `--repo`. For a bare number, infer the repo from `--repo`, an
   available GitHub integration, or `gh repo view`. For a URL, read the owner,
   repo, and number directly.
2. **Watch the PR.** Run the bundled helper to poll reactions and review
   comments on a 30-second heartbeat:

   ```bash
   python3 <skill-dir>/fix-pr-helper.py <pull-request-or-number> [--repo <owner/repo>]
   ```

   The helper prints a heartbeat per check. Continue the loop as long as the
   status is `under review` or `merged` (the PR is still being reviewed or
   already merged; nothing to do). The helper exits with a final verdict on
   its own when the PR is no longer under review.
3. **Read the final verdict.** The helper emits one of:
   - `Status: NEEDS FIX` with a list of `P0:` and `P1:` review-comment links.
   - `Status: READY TO MERGE` (no P0/P1 findings; nothing to fix).
   - `Status: NEEDS REVIEW` (no reactions and no Codex or Claude comments;
     the PR has not been reviewed yet; do not run autofix).
   - `Status: MERGED` (the PR is already merged; do not run autofix).
4. **Delegate to *autofix* for actionable findings.** When the verdict is
   `NEEDS FIX`, run the *autofix* skill in comment mode once per `P0:` or
   `P1:` link from the verdict. For each link, invoke
   `autofix --comment <link>` (with `--repo <owner/repo>` when needed so
   *autofix* can resolve the target repository). Run them sequentially in
   the order they appear in the verdict so a failing link does not block
   the remaining ones; continue the loop after the whole batch ships.
5. **Skip on non-actionable verdicts.** When the verdict is `READY TO MERGE`,
   `NEEDS REVIEW`, or `MERGED`, do not invoke *autofix*. Report the verdict
   and stop the loop.
6. **Repeat after the fix ships.** Re-run the helper to confirm the new
   commit is now `READY TO MERGE`, `NEEDS REVIEW`, or `MERGED`. Continue
   until one of those terminal verdicts appears.

## Notes

- This skill never edits the PR, leaves the current checkout, or creates new
  PRs. The *autofix* skill is responsible for all code changes, commits,
  pushes, replies, and thread resolutions.
- Do not downgrade P0/P1 findings to P3. If a listed link is no longer
  actionable on the current branch, report it in the *autofix* skill's
  decision list as `already fixed` or `rejected` and continue.
- If `gh` is missing or the user is not authenticated, surface the helper's
  error verbatim and stop the loop.
