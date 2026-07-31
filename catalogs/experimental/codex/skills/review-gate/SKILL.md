---
name: review-gate
description: Run an evidence-backed, read-only review gate over one local change, commit, clean branch delta, or GitHub pull request. Use when a user wants a blind challenger and the dedicated Codex reviewer independently checked, verified, and adjudicated before the change is called clean.
---

# Review Gate

Run this skill inline as the orchestrator for a read-only, dual-pass review of
exactly one change target. Dispatch only the blind challenger role.

Announce that the review-gate skill is being used and that it will not modify
the checkout or remote state.

## Hard Gate

<HARD-GATE>
This skill is read-only.

- Do not edit, create, delete, rename, or format files.
- Use shell and git only for read-only inspection. Do not redirect output to
  files, install dependencies, run generators, or invoke mutating commands.
- Do not mutate git state, refs, branches, commits, the index, or the working
  tree. In particular, do not fetch, pull, checkout, switch, stash, reset,
  restore, clean, add, commit, merge, rebase, or push.
- Use GitHub access read-only. Do not create or change pull requests, reviews,
  comments, labels, issues, or other external state.
- Do not apply fixes. The only deliverable is the report in chat.
</HARD-GATE>

Focused tests or reproductions are allowed only during verification when their
code and side effects are trusted, execution is authorized, and they will not
write into the repository. Recheck repository state afterward. Otherwise use
static evidence or report that required verification could not be completed.

## 1. Resolve One Complete Target

Read the user's arguments as an optional target: `local`, a commit or ref, a
base branch, or a PR number/URL. Do not guess between multiple plausible
targets.

First record:

- repository root and current branch;
- `HEAD` as a full commit SHA;
- staged, unstaged, and untracked paths;
- the candidate base ref and its full commit SHA;
- any associated PR's URL, head SHA, base ref, and base SHA.

Use read-only commands such as `git rev-parse`, `git status --short --branch`,
`git diff`, `git diff --cached`, `git ls-files --others --exclude-standard`,
`git log`, `git merge-base`, and `git for-each-ref`. For PR metadata, prefer an
installed GitHub connector and use `gh` only when connector coverage is
insufficient. Never fetch to make local refs agree with remote metadata.

Resolve exactly one mode:

- **Local:** all staged, unstaged, and untracked work in the current checkout.
  The complete native command will be `codex review --uncommitted`.
- **Commit:** one commit resolved to a full immutable SHA. The complete native
  command will be `codex review --commit <full-sha>`.
- **Branch:** the clean `base...HEAD` delta. Record the base ref, resolved base
  SHA, merge base, and head SHA. The complete native command will be
  `codex review --base <base-ref>`.
- **PR:** the clean checked-out PR head against its advertised base. Require
  current `HEAD` to equal the PR head SHA and find a local base ref that equals
  the PR base SHA. Record the PR URL and metadata. The complete native command
  will be `codex review --base <matching-local-base-ref>`.

When no target is explicit, use a single associated PR when it exactly matches
the checkout; otherwise use a uniquely resolvable branch delta or purely local
work. If no unique complete target exists, stop with status: `incomplete`.

Dirty local work combined with a commit, branch, or PR delta is mixed scope.
Stop with status: `incomplete`; do not silently narrow to either portion and do
not stash, checkout, or otherwise mutate the checkout to resolve it. Also stop
as incomplete when a non-local target is dirty, the PR head differs from
`HEAD`, the advertised base SHA is unavailable locally, the base is ambiguous,
or the target has no reviewable delta.

Build a `[REVIEW TARGET]` block containing the exact mode, repository, head,
base, PR metadata, complete changed-file list, and read-only commands that
reproduce the selected delta.

## 2. Start the Blind Challenger

Read `<skill-dir>/roles/challenger.md` and append the complete
`[REVIEW TARGET]` block.

Use the native subagent tools to dispatch one challenger.
Dispatch the challenger before reading native review output. The prompt must be
self-contained: include the role file's full
contents and target block, state that the worker is read-only, and prohibit
further delegation. Do not give it prior findings, suspected defects, expected
answers, or native reviewer output.

If dispatch is unavailable, blocked, or declined, run the same challenger
brief inline to completion before starting native review. This ordering keeps
the fallback unanchored. Record whether the pass used native dispatch or the
inline fallback.

## 3. Run the Required Native Reviewer

Check `command -v codex` and `codex login status` without changing
authentication. Missing or unauthenticated Codex CLI is incomplete in both
runtimes. Do not substitute another reviewer.

Run exactly one applicable native command selected in step 1. Force the
reviewer's own command sandbox to read-only and deny approval escalation:

```text
codex review --uncommitted -c 'sandbox_mode="read-only"' -c 'approval_policy="never"'
codex review --commit <full-sha> -c 'sandbox_mode="read-only"' -c 'approval_policy="never"'
codex review --base <base-ref> -c 'sandbox_mode="read-only"' -c 'approval_policy="never"'
```

The `sandbox_mode="read-only"` and `approval_policy="never"` overrides are part
of the one invocation, not a separate review pass. Do not run more than one,
retry with another mode, or silently replace a failed run. Preserve the
complete output for adjudication. A launch failure, authentication failure,
nonzero exit, or unusable result makes the gate incomplete. If repository state
changes despite the sandbox, preserve the evidence and mark the gate
incomplete.

Collect the blind challenger result after launching the native pass. If the
worker did not return, run the untouched challenger brief inline only if doing
so still keeps it blind to native output; otherwise report the missing pass and
mark the gate incomplete.

## 4. Verify and Adjudicate Every Candidate

Create one candidate set from native and challenger output and deduplicate
overlap. Do not accept findings merely because both passes agree.

For every plausible candidate:

1. Locate the exact line in the target's version of the file.
2. State a reachable scenario with concrete inputs and control flow.
3. Read the real ownership path for affected data or state.
4. Read unchanged consumers and callers that determine impact.
5. Compare against the exact reviewed delta and relevant repository contracts.
6. Run the smallest focused tests or reproductions only when execution is
   trusted and authorized. First identify their writes and route caches,
   temporary files, and other outputs outside the repository. If repository
   immutability cannot be guaranteed, do not execute them. Record the command,
   environment controls, and result.
7. Accept or reject the candidate with evidence and rationale.

Findings in unchanged files remain valid when the reviewed change demonstrably
causes or activates them. Reject pre-existing, unreachable, speculative, or
out-of-scope candidates with the evidence that refutes them. If a plausible
candidate requires verification that cannot safely or authoritatively be
performed, do not guess: record the limitation and mark the gate incomplete.

Never fix code, stage changes, commit, push, or otherwise mutate git state
during the gate.

## 5. Check Target Integrity

Repeat the read-only repository and target checks from step 1. The head, base,
dirty paths, changed-file set, and PR metadata must still describe the same
target. Any target drift or new repository change makes the result incomplete,
even when both reviewers otherwise reported no findings.

## 6. Report

Report every section:

1. **Exact target:** mode, repository, head SHA, base ref/SHA, PR URL when
   applicable, changed files, and the delta inspected.
2. **Completed passes:** challenger dispatch mode and the one exact native
   command, including failures.
3. **Verification evidence:** ownership paths, unchanged consumers, focused
   tests or reproductions, and target-integrity result.
4. **Accepted findings:** priority, `path:line`, reachable scenario, evidence,
   and rationale for every finding.
5. **Rejected candidates:** source pass, `path:line`, reachable scenario
   claimed, evidence checked, and rejection rationale.
6. **Limitations:** anything unavailable, unsafe, unauthorized, ambiguous, or
   unverified; write `none` when empty.
7. **Status:** end with one status: `clean`, `findings`, or `incomplete`.

Use `clean` only when the exact target stayed stable, both required passes
completed, every candidate was adjudicated, required verification succeeded,
no accepted findings remain, and no blocking limitation exists. Use `findings`
when those completeness conditions hold and at least one finding is accepted.
Use `incomplete` for ambiguous or mixed scope, a missing or failed pass, target
drift, failed required verification, or any other condition that prevents a
defensible gate result. Never describe an incomplete run as clean.

This is a procedural review contract, not a CI gate. Do not promise
machine-readable output or process exit-code semantics.
