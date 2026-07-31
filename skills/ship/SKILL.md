---
name: ship
description: Commit and push the current branch, then create or update a PR. Use when the user wants to ship changes, open or reuse a PR, or run the repo's commit-push-PR workflow with a detailed new PR description, while leaving an existing PR description unchanged unless explicitly asked and adding a detailed comment when new commits are pushed to an existing PR.
---

# Ship

Use this skill when the user wants the current work committed and pushed, with a PR created only if one does not already exist.

## GitHub Access

Prefer an installed GitHub integration for repository and PR metadata, issue comments, and PR creation or updates when available. Use local `git` and `gh` for branch state, pushing, current-branch PR discovery, or any integration coverage gaps.

## Workflow

If the handoff context says `existing PR only; stop rather than create`, do not use the normal new-PR fallback. When that handoff is present, check for the existing PR before committing, pushing, or creating a PR. Stop instead of creating a new PR if no existing PR is associated with the branch.

1. Identify intended changes before staging anything.
2. Exclude obvious noise such as caches, build artifacts, editor files, and unrelated untracked files unless the user clearly wants them committed.
3. Group the remaining changes into logical changesets. Prefer fewer commits when the split is ambiguous.
4. Refresh remote state using `git fetch` before deciding whether the local starting point is current.
5. When a new local branch is needed, choose a short descriptive name tied to the work rather than committing on a protected or ambiguous branch.
6. Stage and commit one logical changeset at a time.
7. Push the resulting branch to its upstream. If there is no upstream, set one on push.
8. Resolve whether a PR already exists for the current branch with the GitHub integration when available, or with `gh pr view`.
9. If a PR already exists, do not edit the title or description unless the user explicitly asks you to. When the push adds new commits to that existing PR, add a detailed PR comment that explains how the new work relates to the existing PR, especially if it changes scope or rationale.
10. If no PR exists, create one with a detailed description:
   - Summarize the user-visible change.
   - Call out the main implementation points.
   - Mention verification or testing when relevant.
   - Keep the description specific to the shipped diff rather than generic template text.

## Rules

- Do not rewrite an existing PR description unless the user explicitly requests it; use a new PR comment to document newly pushed commits on an existing PR.
- Do not rewrite history unless the user explicitly asks.
- Do not amend existing commits unless requested.
- Do not create empty commits unless the user explicitly wants one.
- Do not include a `Co-Authored-By` trailer in commit messages unless explicitly requested.
- Never include attribution to a coding agent or model, or links to vendor-generated artifacts, in commit messages or pull request titles, descriptions, or comments.
- If there is nothing to commit, say so plainly.
- If commit hooks or git identity settings block the commit, surface the exact error and stop.
- If commit or push fails, surface the exact blocker instead of guessing.
- Keep the workflow minimal: no branch cleanup or force-push.
