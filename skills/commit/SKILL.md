---
name: commit
description: Create local git commits for the current worktree without pushing or opening a PR. Use when the user wants recent changes committed, wants the current diff split into one or more logical commits, wants the branch checked against its remote before committing, or asks for a safe local-only git checkpoint.
---

# Commit

Create clean local-only commits from the current worktree. Start from a remote-synced base when possible, separate independent changes into distinct commits, and leave the branch unpushed.

## Workflow

1. Inspect the repo state with `git status --short --branch`, the current branch name, `git diff HEAD`, and recent commit style with `git log --oneline -5`.
2. Identify intended changes before staging anything.
3. Exclude obvious noise such as caches, build artifacts, editor files, and unrelated untracked files unless the user clearly wants them committed.
4. Group the remaining changes into logical changesets.
   - Use one commit when the work is one cohesive change.
   - Create multiple commits only when there are clearly separable changes in behavior, scope, or file ownership.
   - Prefer fewer commits when the split is ambiguous.
5. Check remote state before creating new commits.
   - Determine the upstream branch when one exists.
   - Fetch before deciding whether the local starting point is current.
   - If fetch is blocked by sandbox or network restrictions, request approval or report the blocker instead of pretending the branch is synced.
6. Choose the branch strategy.
   - If the current branch is behind its upstream and has no local-only commits to preserve, fast-forward or rebase onto the remote tip before committing.
   - If the current branch is behind or diverged and syncing it would require intrusive history edits, create a new local branch from the updated remote base and commit there.
   - If the user is on `main`, `master`, a release branch, or detached `HEAD`, prefer creating a new local branch unless the user explicitly asked to commit there.
7. Stage and commit one logical changeset at a time.
   - Before committing, run `gofmt` on changed Go files that will be included in the commit.
   - Use path-based staging or `git add -p` to keep commits clean.
   - Write reasonably detailed commit messages.
   - Use a clear subject line plus a body when the change is non-trivial.
   - In the message body, summarize what changed and why it changed. Include implementation detail only when it helps future readers.
   - Do not include a `Co-Authored-By` trailer unless the user explicitly asks for one.
   - Do not sweep unrelated files into a commit just to make the worktree clean.
8. Verify the result with `git status --short --branch` and `git log --oneline -5`.

## Sync Rule

Bring the starting point in sync with the relevant remote branch before creating new commits. After committing, it is expected that the local branch is ahead of remote by the new local commit or commits. Do not push as part of this skill.

## Constraints

- Do not push.
- Do not create, update, or inspect pull requests unless the user separately asks for that workflow.
- Do not rewrite history unless the user explicitly asks.
- Do not amend existing commits unless requested.
- Do not create empty commits unless the user explicitly wants one.
- Do not include a `Co-Authored-By` trailer in commit messages unless explicitly requested.
- If there is nothing to commit, say so plainly.
- If commit hooks or git identity settings block the commit, surface the exact error and stop.
- Avoid vague subjects such as `fix stuff` or `updates`.

## Branch Naming

When a new local branch is needed, choose a short descriptive name tied to the work. If there is no obvious topic, use a neutral name such as `codex/<brief-topic>` rather than committing on a protected or ambiguous branch.
