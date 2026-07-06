---
name: ship
description: Commit the current branch by first following the `commit` skill workflow, then push it and create or update a PR. Use when the user wants to ship changes, open or reuse a PR, or run the repo's push-and-PR workflow with a detailed new PR description, while leaving an existing PR description unchanged unless explicitly asked and adding a detailed comment when new commits are pushed to an existing PR.
---

# Ship

Use this skill when the user wants the current work committed and pushed, with a PR created only if one does not already exist.

## GitHub Access

Prefer an installed GitHub connector for repository and PR metadata, issue comments, and PR creation or updates when available. Use local `git` and `gh` for branch state, pushing, current-branch PR discovery, or any connector coverage gaps.

## Workflow

If the handoff context says `existing PR only; stop rather than create`, do not use the normal new-PR fallback. When that handoff is present, check for the existing PR before committing, pushing, or creating a PR. Stop instead of creating a new PR if no existing PR is associated with the branch.

1. Start by following the *commit* skill workflow.
2. Push the resulting branch to its upstream. If there is no upstream, set one on push.
3. Resolve whether a PR already exists for the current branch with the connector when available, or with `gh pr view`.
4. If a PR already exists, do not edit the title or description unless the user explicitly asks you to. When the push adds new commits to that existing PR, add a detailed PR comment that explains how the new work relates to the existing PR, especially if it changes scope or rationale.
5. If no PR exists, create one with a detailed description:
   - Summarize the user-visible change.
   - Call out the main implementation points.
   - Mention verification or testing when relevant.
   - Keep the description specific to the shipped diff rather than generic template text.

## Rules

- Do not rewrite an existing PR description unless the user explicitly requests it; use a new PR comment to document newly pushed commits on an existing PR.
- Use the *commit* skill's branch-sync and commit-splitting rules rather than inventing a separate local commit flow here.
- If commit or push fails, surface the exact blocker instead of guessing.
- Keep the workflow minimal: no branch cleanup, no force-push, no history rewriting.
