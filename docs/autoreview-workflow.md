# Autoreview Workflow

This repo exposes a reusable GitHub Actions workflow for other repositories:

```text
brian-bell/skills/.github/workflows/autoreview.yml@main
```

The workflow resolves a pull request, checks out the PR branch, checks out this
skills repo into `.skills/`, mounts the portable autoreview/commit/ship skills
into an isolated Codex home, verifies the autoreview `SKILL.md` and executable
script are visible there, runs the autoreview helper outside the Codex sandbox,
then launches Codex headlessly with this prompt:

```text
autoreview this PR. after final completion, push changes to the PR and add a comment
```

The shell helper is responsible for the nested review/model call. The headless
Codex action is responsible for reading the helper's report, addressing
accepted/actionable findings, committing and pushing any fixes to the PR branch,
and leaving a PR comment when it is done.

## Requirements

- Add `OPENAI_API_KEY` as a repository or organization secret.
- If `brian-bell/skills` is private to the consuming repository, also provide a
  `SKILLS_REPOSITORY_TOKEN` secret that can read it. Public consumers do not
  need this extra token.
- Call the workflow from trusted events only. The recommended trigger is a PR
  comment or review from `OWNER` or `COLLABORATOR` users with write access. If
  you want to allow organization `MEMBER` users without direct write access,
  configure the Codex action authorization policy explicitly.
- Give the caller job write permissions so Codex can push commits and comment on
  the PR.

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
        description: Optional instructions for the autoreview workflow.
        required: false
        type: string
      codex_model:
        description: Optional Codex model for autoreview.
        required: false
        type: string
      codex_effort:
        description: Optional Codex reasoning effort for autoreview.
        required: false
        type: string
      codex_version:
        description: Optional @openai/codex version for openai/codex-action.
        required: false
        type: string

jobs:
  autoreview:
    if: |
      (github.event_name == 'issue_comment' && github.event.issue.pull_request && (contains(github.event.comment.body, '@codex') || contains(github.event.comment.body, '@autoreview')) && contains(fromJSON('["OWNER", "COLLABORATOR"]'), github.event.comment.author_association)) ||
      (github.event_name == 'pull_request_review_comment' && (contains(github.event.comment.body, '@codex') || contains(github.event.comment.body, '@autoreview')) && contains(fromJSON('["OWNER", "COLLABORATOR"]'), github.event.comment.author_association)) ||
      (github.event_name == 'pull_request_review' && (contains(github.event.review.body, '@codex') || contains(github.event.review.body, '@autoreview')) && contains(fromJSON('["OWNER", "COLLABORATOR"]'), github.event.review.author_association)) ||
      github.event_name == 'workflow_dispatch'
    uses: brian-bell/skills/.github/workflows/autoreview.yml@main
    permissions:
      contents: write
      pull-requests: write
      issues: write
    secrets: inherit
    with:
      pr_number: ${{ github.event.inputs.pr_number || '' }}
      instructions: ${{ github.event.inputs.instructions || '' }}
      codex_model: ${{ github.event.inputs.codex_model || '' }}
      codex_effort: ${{ github.event.inputs.codex_effort || '' }}
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

## Inputs

| Input | Default | Description |
|---|---:|---|
| `pr_number` | empty | Manual dispatch PR number. If omitted on manual runs, the workflow chooses the most recently updated open PR. |
| `instructions` | empty | Optional manual-dispatch instructions appended to the Codex prompt. Comment/review text is used for comment-triggered runs. |
| `skills_repository` | `brian-bell/skills` | Repository that contains the portable skills and reusable workflow support files. |
| `skills_ref` | `main` | Ref checked out from `skills_repository`. Consumers should usually leave this as `main`. |
| `codex_model` | empty | Optional Codex model passed to `openai/codex-action`. |
| `codex_effort` | empty | Optional Codex reasoning effort passed to `openai/codex-action`. |
| `codex_version` | empty | Optional `@openai/codex` version passed to `openai/codex-action`. |
| `responses_api_endpoint` | empty | Optional Responses API endpoint override passed to `openai/codex-action`. |

## Secrets

| Secret | Required | Description |
|---|---:|---|
| `OPENAI_API_KEY` | yes | Passed to `openai/codex-action`. |
| `SKILLS_REPOSITORY_TOKEN` | no | Optional token used to checkout `skills_repository` when the caller token cannot read it. |

## Behavior

The reusable workflow:

1. Resolves the PR from the triggering comment, review, or manual input.
2. Checks out the PR branch with full history.
3. Verifies the checked-out branch still matches the resolved PR head SHA.
4. Fetches the PR base branch from the caller repository through a dedicated
   `base` remote, then pins the resolved base SHA as
   `refs/autoreview/pr-base-resolved`
   for review.
5. Checks out `brian-bell/skills` into `.skills/` and hides that directory from
   the reviewed repository's git status.
6. Mounts `.skills/catalog/portable/autoreview`, `commit`, and `ship` into an
   isolated Codex home under `skills/`.
7. Fails early unless `skills/autoreview/SKILL.md` exists and
   `skills/autoreview/scripts/autoreview` is executable.
8. Configures the GitHub Actions bot identity for any commits Codex creates.
9. Uses `openai/codex-action` to prepare the shared Codex home and Responses API
   proxy for the helper.
10. Runs `skills/autoreview/scripts/autoreview` as a normal shell step outside
   the Codex sandbox against `refs/autoreview/pr-base-resolved`, with
   `--skip-fetch`, writing its JSON report and summary under
   `.codex-autoreview/`.
11. Runs `openai/codex-action` in workspace-write mode with the same Codex home
   passed through `codex-home` and `CODEX_HOME`.
12. Tells Codex not to rerun `$autoreview` inside the sandbox, and instead to
   read `.codex-autoreview/autoreview-report.json` and
   `.codex-autoreview/autoreview-summary.txt`.
13. Lets Codex fix findings from the helper report, push commits, and leave the
   final PR comment.
14. Posts a fallback PR comment if no marked autoreview comment exists for the
   workflow run. Successful Codex runs use Codex's final message; setup or Codex
   failures get a failure-specific comment with the workflow run URL.

## Migration Notes

This replaces the old reusable workflow path:

```text
brian-bell/skills/.github/workflows/autoreview-ship.yml@main
```

Consumers must update callers to:

```text
brian-bell/skills/.github/workflows/autoreview.yml@main
```

Old inputs such as `autoreview_model`, `autoreview_thinking`, and
`autoreview_parallel_tests` are not supported by the new workflow. Use
`codex_model` and `codex_effort` for Codex configuration.

## Safety Notes

- Do not run this reusable workflow from untrusted `pull_request` events with
  write permissions and secrets.
- The recommended caller gates comment triggers to repository collaborators.
- Only trigger this workflow for PR code you are comfortable letting a
  workspace-write Codex session inspect and operate around with `OPENAI_API_KEY`
  and write-capable GitHub tokens available.
- Fork PRs may not be pushable with the caller repository token. In that case
  Codex should report the push blocker in its PR comment or final message.
- The `.skills/` checkout is support material only. The workflow excludes it
  from git status, copies runnable skills into an isolated Codex home, and tells
  Codex not to stage, edit, or commit `.skills/`.
