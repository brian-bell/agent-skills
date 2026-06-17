---
name: commit
description: Create local git commits for the current worktree without pushing or opening a PR. Use when the user wants recent changes committed, wants the current diff split into one or more logical commits, wants the branch checked against its remote before committing, or asks for a safe local-only git checkpoint.
---

# Commit

Create clean local-only commits from the current worktree. Start from a remote-synced base when possible, separate independent changes into distinct commits, and leave the branch unpushed.

## Contract

- Identify intended changes before staging anything.
- Exclude obvious noise such as caches, build artifacts, editor files, and unrelated untracked files unless the user clearly wants them committed.
- Group the remaining changes into logical changesets. Prefer fewer commits when the split is ambiguous.
- Check remote state before before deciding whether the local starting point is current.
- Stage and commit one logical changeset at a time.
- Do not push.
- Do not create, update, or inspect pull requests unless the user separately asks for that workflow.
- Do not rewrite history unless the user explicitly asks.
- Do not amend existing commits unless requested.
- Do not create empty commits unless the user explicitly wants one.
- Do not include a `Co-Authored-By` trailer in commit messages unless explicitly requested.
- If there is nothing to commit, say so plainly.
- If commit hooks or git identity settings block the commit, surface the exact error and stop.
- When a new local branch is needed, choose a short descriptive name tied to the work. If there is no obvious topic, use a neutral name such as `codex/<brief-topic>` rather than committing on a protected or ambiguous branch.
