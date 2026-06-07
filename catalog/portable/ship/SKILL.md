---
name: ship
description: Commit the current branch by first following the `commit` skill workflow, then push it and create a PR if needed. Use when the user wants to ship changes, open or reuse a PR, or run the repo's push-and-PR workflow with a detailed new PR description, while leaving an existing PR description unchanged unless explicitly asked and adding a detailed comment when new commits are pushed to an existing PR.
---

# Ship

Use this skill when the user wants the current work committed and pushed, with a PR created only if one does not already exist.

## Workflow

1. Start by following the `commit` skill workflow.
   - Sync with the relevant remote branch before creating new commits.
   - Split discrete changes into separate commits when appropriate.
   - Use reasonably detailed commit messages.
   - Leave the branch unpushed at the end of the commit phase.
2. Push the resulting branch to its upstream. If there is no upstream, set one on push.
3. Check whether a PR already exists for the branch.
4. If a PR already exists, do not edit the title or description unless the user explicitly asks you to. When the push adds new commits to that existing PR, add a detailed PR comment that explains:
   - What changed in the newly pushed commits.
   - Why the changes were made.
   - Any verification, test results, or important caveats.
   - How the new work relates to the existing PR, especially if it changes scope or rationale.
5. If no PR exists, create one with a detailed description.
   - Summarize the user-visible change.
   - Call out the main implementation points.
   - Mention verification or testing when relevant.
   - Keep the description specific to the shipped diff rather than generic template text.

## Rules

- Do not rewrite an existing PR description unless the user explicitly requests it; use a new PR comment to document newly pushed commits on an existing PR.
- Prefer `gh pr view` to detect whether a PR already exists before creating a new one.
- Use the `commit` skill's branch-sync and commit-splitting rules rather than inventing a separate local commit flow here.
- If commit or push fails, surface the exact blocker instead of guessing.
- Keep the workflow minimal: no branch cleanup, no force-push, no history rewriting.
