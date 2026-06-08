# Autoreview Ship Workflow

This repo exposes a reusable GitHub Actions workflow for other repositories:

```text
brian-bell/skills/.github/workflows/autoreview-ship.yml@main
```

The workflow resolves a pull request, checks out the PR branch, checks out this
skills repo into `.skills/`, copies `$autoreview`, `$commit`, and `$ship` into
Codex home, then runs `$autoreview` as an explicit shell gate before invoking
`openai/codex-action` for `$ship`. If autoreview fails, disconnects, or reports
accepted/actionable findings, the job fails and `$ship` is not run.

## Requirements

- Add `OPENAI_API_KEY` as a repository or organization secret.
- If `brian-bell/skills` is private to the consuming repository, also provide a
  `SKILLS_REPOSITORY_TOKEN` secret that can read it. Public consumers do not
  need this extra token.
- Call the workflow from trusted events only. The recommended trigger is a PR
  comment or review from `OWNER`, `MEMBER`, or `COLLABORATOR`.
- Give the caller job write permissions only when you want Codex to push or
  update a PR.

## Recommended Caller

Create `.github/workflows/codex.yml` in the consuming repository:

```yaml
name: Codex

on:
  issue_comment:
    types: [created]
  pull_request_review_comment:
    types: [created]
  pull_request_review:
    types: [submitted]
  workflow_dispatch:
    inputs:
      pr_number:
        description: Pull request number. Defaults to the most recently updated open PR.
        required: false
        type: string
      instructions:
        description: Optional instructions for the ship workflow.
        required: false
        type: string
      autoreview_parallel_tests:
        description: Optional test command Codex should run as part of the autoreview gate.
        required: false
        type: string
      codex_version:
        description: Optional @openai/codex version for openai/codex-action.
        required: false
        type: string

jobs:
  autoreview_ship:
    if: |
      (github.event_name == 'issue_comment' && github.event.issue.pull_request && (contains(github.event.comment.body, '@codex') || contains(github.event.comment.body, '@autoreview')) && contains(fromJSON('["OWNER", "MEMBER", "COLLABORATOR"]'), github.event.comment.author_association)) ||
      (github.event_name == 'pull_request_review_comment' && (contains(github.event.comment.body, '@codex') || contains(github.event.comment.body, '@autoreview')) && contains(fromJSON('["OWNER", "MEMBER", "COLLABORATOR"]'), github.event.comment.author_association)) ||
      (github.event_name == 'pull_request_review' && (contains(github.event.review.body, '@codex') || contains(github.event.review.body, '@autoreview')) && contains(fromJSON('["OWNER", "MEMBER", "COLLABORATOR"]'), github.event.review.author_association)) ||
      github.event_name == 'workflow_dispatch'
    uses: brian-bell/skills/.github/workflows/autoreview-ship.yml@main
    permissions:
      contents: write
      pull-requests: write
      issues: write
      actions: read
    secrets: inherit
    with:
      pr_number: ${{ github.event.inputs.pr_number || '' }}
      instructions: ${{ github.event.inputs.instructions || '' }}
      autoreview_parallel_tests: ${{ github.event.inputs.autoreview_parallel_tests || '' }}
      codex_version: ${{ github.event.inputs.codex_version || '' }}
```

Then invoke it from a pull request comment:

```text
@codex
```

or:

```text
@autoreview
```

Both forms run the same path: autoreview first, then `$ship` if the gate passes.

## Inputs

| Input | Default | Description |
|---|---:|---|
| `pr_number` | empty | Manual dispatch PR number. If omitted on manual runs, the workflow chooses the most recently updated open PR. |
| `instructions` | empty | Optional manual-dispatch instructions passed into Codex ship mode. Comment/review text is used for comment-triggered runs. |
| `skills_repository` | `brian-bell/skills` | Repository that contains the portable skills and reusable workflow support files. |
| `skills_ref` | `main` | Ref checked out from `skills_repository`. Consumers should usually leave this as `main`. |
| `autoreview_model` | empty | Optional Codex model passed to `autoreview --model`. |
| `autoreview_thinking` | empty | Optional Codex reasoning effort passed to `autoreview --thinking`. |
| `autoreview_parallel_tests` | empty | Optional test command Codex should run as part of the autoreview gate. |
| `codex_version` | empty | Optional `@openai/codex` version installed for the autoreview gate and passed to `openai/codex-action`. |
| `post_feedback` | `true` | Whether to post the autoreview or Codex result back to the PR. |

## Secrets

| Secret | Required | Description |
|---|---:|---|
| `OPENAI_API_KEY` | yes | Used by the explicit autoreview gate and passed to `openai/codex-action`. |
| `SKILLS_REPOSITORY_TOKEN` | no | Optional token used to checkout `skills_repository` when the caller token cannot read it. |

## Behavior

The reusable workflow:

1. Resolves the PR from the triggering comment, review, or manual input.
2. Checks out the PR branch with full history.
3. Checks out `brian-bell/skills` into `.skills/` and hides that directory from
   the reviewed repository's git status.
4. Copies `.skills/catalog/portable/autoreview`,
   `.skills/catalog/portable/commit`, and `.skills/catalog/portable/ship` into
   `codex-home`.
5. Fetches the PR base branch.
6. Installs the Codex CLI and runs `$autoreview` directly in a shell step
   against the already-fetched base ref.
7. Fails the job before `$ship` if autoreview fails, disconnects, or reports
   accepted/actionable findings.
8. Invokes `openai/codex-action` with that `codex-home` for the `$ship` phase
   only after the autoreview gate passes.
9. Reruns the shell autoreview gate if the Codex ship phase changes `HEAD` or
   leaves worktree changes behind.

## Safety Notes

- Do not run this reusable workflow from untrusted `pull_request` events with
  write permissions and secrets.
- The recommended caller gates comment triggers to repository collaborators.
- Fork PRs may not be pushable with the caller repository token. In that case
  Codex should report the push blocker instead of guessing.
- The `.skills/` checkout is support material only. The workflow excludes it
  from git status, copies the runnable skills into Codex home, and tells Codex
  not to stage, edit, or commit `.skills/`.
