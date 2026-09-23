---
name: code-review-loop
description: "Run Claude Code's built-in /code-review skill in a loop, verifying and fixing its findings between rounds, until a review round comes back with no unresolved findings. Use when the user asks to loop code review until clean, keep reviewing and fixing until there are no findings, or invokes /code-review-loop with an optional effort level, target, and round limit. Examples - /code-review-loop, /code-review-loop max, /code-review-loop high src/api --max-rounds 3"
argument-hint: "[low|medium|high|xhigh|max] [target] [--max-rounds N]"
---

# Code Review Loop

Run this skill inline as the orchestrator. Each round runs the built-in
`code-review` skill, verifies every finding against the real code, fixes the
accepted ones, and reviews again. The loop ends when a round has no
unresolved findings.

Announce at start: "I'm using the code-review-loop skill to review and fix
until /code-review comes back clean."

## Rules

- Run each review with the Skill tool (`skill: "code-review"`). Do not stand
  in your own review or a subagent's for it.
- Never pass `ultra`. It starts a billed cloud review that only the user can
  launch. If the user asks for `ultra`, stop and tell them to run
  `/code-review ultra` themselves.
- Never pass `--comment` or `--post`. The loop is local and must not post to a
  pull request.
- Do not pass `--fix`. The orchestrator verifies each finding before changing
  code, because higher effort levels can report uncertain findings.
- Leave fixes uncommitted in the working tree. Do not commit, push, stash,
  reset, restore, checkout, or rebase, and do not change pull request state.
- Treat review output as advisory. Verify each finding before you act on it.

## 1. Parse Arguments

Read `$ARGUMENTS`:

- **Level:** `low`, `medium`, `high`, `xhigh`, or `max`. Default to `high`.
  Pass the level explicitly every round. Without one, `/code-review` reuses
  the last level the user typed, and the loop would drift.
- **Target:** anything else that `/code-review` accepts (a PR number, branch,
  or path). Default to no target, which reviews the current diff.
- **`--max-rounds N`:** default 5.

Plain-language equivalents work too: "loop code review at max, three rounds
at most" means `max --max-rounds 3`.

## 2. Check the Target

Record the repository root, current branch, `HEAD`, and
`git status --short`.

- With no target, the working tree must have a diff to review. If it has
  none, stop and say so.
- With a PR or branch target, that branch must be checked out here, or the
  fixes would land somewhere other than the reviewed code. If it is not,
  stop and ask the user. Do not check it out yourself.
- With a path target, the path must exist in the checkout.

Note any uncommitted changes that were already present so the final report
can separate the user's own edits from the loop's fixes.

## 3. Run a Round

Round `n` of `max-rounds`:

1. **Review.** Invoke the `code-review` skill with args `<level> [target]`.
   Wait for it to finish. If it runs in the background, wait for its
   completion notice rather than polling or starting another review.
2. **Match.** Compare each finding against the ledger (below). A finding is
   a repeat when it names the same defect in the same file; line numbers may
   have shifted.
3. **Verify.** For each new finding, read the code path and adjacent code,
   plus dependency docs or types when the finding depends on external
   behavior. Classify it:
   - **accepted:** a real defect within the target's scope;
   - **rejected:** a false positive, an intended behavior, or a speculative
     edge case. Record the evidence (code, tests, or docs that disprove it);
   - **deferred:** real, but fixing it needs a decision from the user, such as
     a public API change, a product behavior choice, or a broad refactor.
4. **Fix.** For each accepted finding, make the smallest fix at the right
   ownership boundary. When a finding points to a repeated pattern, fix the
   sibling instances within the target's scope. Leave unrelated code alone.
5. **Check.** Run the focused tests, type checks, or linters that cover the
   touched code. When they fail because of a fix, repair the fix before the
   next round.
6. **Report the round:**

   ```text
   Round n/max: F findings - A accepted and fixed, R rejected, D deferred, P repeats
   - <file:line> <one-line summary> -> <outcome>
   ```

If the target cannot include uncommitted changes (for example, a PR reviewed
from remote state rather than the checkout), round 2 would re-review stale
code. Stop after round 1 and report that instead of looping.

## 4. Keep a Ledger

Keep one ledger for the whole run. Each entry has the file, a one-line
summary, the round it was first seen, its status, and evidence or the fix.

Handle repeats this way:

- A **rejected** finding that comes back with nothing new stays rejected and
  does not block a clean result. When the reviewer adds new evidence,
  re-verify it.
- A **fixed** finding that comes back means the fix did not hold. Re-verify
  and fix it again. If it comes back a second time, mark it **stuck** and stop.
- A **deferred** finding that comes back stays deferred.

## 5. Decide Whether to Continue

Check these in order after each round:

1. **Clean:** the round had no new findings and no findings that came back
   after a fix. Everything left is rejected or deferred. Stop with status
   `clean`, or `clean with deferrals` when deferred items remain.
2. **Stuck:** a finding is stuck, or for two rounds in a row the fixes for
   earlier findings have caused new findings in the same code. Stop with
   status `stalled`, and ask the user how to proceed.
3. **Limit reached:** `n` equals `max-rounds`. Stop with status
   `round limit reached`. Do not run more rounds without the user's consent.
4. Otherwise, start the next round.

A round that accepted and fixed anything always needs a later round to
confirm it. Never declare `clean` on the same round that made fixes.

Stop early and ask the user when a deferred finding blocks the rest of the
work, or when a fix would reach well outside the target's scope.

## 6. Final Report

```text
--- Code Review Loop: <target or "current diff"> at <level> ---
Status: clean | clean with deferrals | stalled | round limit reached
Rounds: n/max

Round 1: 6 findings - 4 fixed, 2 rejected
Round 2: 1 finding - 1 fixed
Round 3: 0 new findings - clean

Fixed:
- <file:line> <summary> - <what changed>
Rejected:
- <file:line> <summary> - <evidence>
Deferred (needs your decision):
- <file:line> <summary> - <the decision needed>

Checks run: <commands and results>
Changes left uncommitted in: <files touched by the loop>
```

List pre-existing uncommitted changes apart from the loop's own changes. When
the user wants to commit, suggest running the *ship* skill.
