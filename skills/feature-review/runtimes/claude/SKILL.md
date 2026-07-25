---
name: feature-review
description: "Run a read-only feature acceptance review — is the feature complete, safe, well-tested, maintainable, and documented? Use when the user asks whether a feature or PR is ready to accept, or invokes /feature-review with a PR number or feature name and an optional focus list. Examples - /feature-review #42, /feature-review scanner, /feature-review #15 safety,quality"
argument-hint: "<PR number or feature name> [focus: product|safety|quality|maintainability|documentation]"
disallowed-tools: Edit, Write, NotebookEdit
---

# Feature Review

Evaluate a feature at the product level — not code style or syntax, but whether it is complete, safe, well-tested, maintainable, and documented — and deliver an acceptance verdict.

Announce at start: "I'm using the feature-review skill to run an acceptance review."

Run this skill inline as the acceptance lead. Do not fork the whole skill into a subagent. Its checkpoints depend on `AskUserQuestion`, which does not work in forked/subagent contexts, and a forked lead would force five reviewer reports through a single return value. Dispatch only the leaf roles below.

## Roles And Dispatch

| Role | How to dispatch |
|---|---|
| Acceptance lead (you) | Never delegated — owns mode detection, context, checkpoints, consolidation, verdict |
| `product-reviewer` | One agent; prompt from [roles/product-reviewer.md](roles/product-reviewer.md) |
| `safety-reviewer` | One agent; prompt from [roles/safety-reviewer.md](roles/safety-reviewer.md) |
| `quality-reviewer` | One agent; prompt from [roles/quality-reviewer.md](roles/quality-reviewer.md) |
| `maintainability-reviewer` | One agent; prompt from [roles/maintainability-reviewer.md](roles/maintainability-reviewer.md) |
| `documentation-reviewer` | One agent; prompt from [roles/documentation-reviewer.md](roles/documentation-reviewer.md) |

Roles are leaf workers — they must not spawn further agents.

## Hard Constraints

<HARD-GATE>
This skill is READ-ONLY. Read the repository and the pull request. Do not
change anything.

Never modify files. Do not edit, create, or delete files — not with an editor
tool, and not with shell commands.

Never mutate git state. No `git add`, `git commit`, `git push`, or any other
repository-mutating command.

Never write to the pull request. `gh pr view` and `gh pr diff` are reads and
are expected. Do not post comments, submit reviews, edit, close, or merge.
The deliverable is a report presented in chat; the human decides what to do
with it.

Never apply a fix. Reviewing is not fixing.

No exceptions. If you catch yourself about to run a write operation, stop.
</HARD-GATE>

Carry this gate into every role prompt — the role files each restate it, so dispatch them verbatim rather than paraphrasing.

## Scope

Read `$ARGUMENTS`:

- **subject** (required): a PR number (`#42`, `PR 42`, `pull request 42`) selects **PR mode**; anything else is a feature name and selects **feature mode**.
- **focus** (optional): comma-separated reviewer list. Valid values are `product`, `safety`, `quality`, `maintainability`, and `documentation`. Default to all five. Reject unknown values with a concise usage note.

If the subject is ambiguous — it could be a PR reference or a feature name, or no subject was given — use `AskUserQuestion` to resolve it rather than guessing. Offer the concrete readings ("PR #42", "the feature named 42", "something else"). Guessing wrong here wastes five reviewer passes on the wrong subject.

## Phase 1: Discover The Project

1. Read `AGENTS.md` if it exists, falling back to `CLAUDE.md` — the primary source of architecture context.
2. Read `README.md` if it exists.
3. Scan for framework/language markers and note language, framework, and key dependencies:
   - `go.mod` → Go. Note module name and key dependencies.
   - `package.json` → Node/TypeScript. Note framework (React, Next.js, Express, …).
   - `pyproject.toml` / `setup.py` / `requirements.txt` → Python. Note framework (Django, Flask, FastAPI, …).
   - `Cargo.toml` → Rust.
   - Other markers as appropriate.
4. Identify architecture patterns, directory conventions, and the testing approach from the docs and file layout.

## Phase 2: Gather Feature Context

**PR mode:** `gh pr view <N> --json title,body,additions,deletions,changedFiles,baseRefName,headRefName,files,state,author` for metadata, `gh pr view <N>` for the description, `gh pr diff <N>` for the diff, and `gh pr view <N> --json files --jq '.files[].path'` for the changed-file list.

**Feature mode:** check for a directory matching the feature name (`<feature>/`, `*/<feature>/`, `**/<feature>/`), grep for references across source files, and glob all source files in the identified directories — including test files, since reviewers assess coverage. Identify cross-cutting references (other modules that import or depend on the feature). When in doubt about scope, include more files; reviewers can focus.

## Phase 3: Build The Context Summary

Assemble the `[REVIEW CONTEXT]` block the role files expect:

```
[REVIEW CONTEXT]
- Review mode: PR | Feature
- Subject: <PR number and title, or feature name>
- Project type: <language, framework, architecture style>
- Description: <PR body or feature purpose summary>
- Key files: <changed files for PR, module files for feature>
- Related files: <modules that import or interact with the feature>
- Test files: <corresponding tests>
- Project patterns: <key patterns from AGENTS.md/CLAUDE.md/README.md>
- Statistics: <PR: additions/deletions/files changed; feature: total files, lines, test count>
```

Checkpoint: present the summary and the reviewer list to the user, then confirm before dispatching. Use `AskUserQuestion` with choices such as "Correct - run the review", "Scope is off - let me redirect", and "Drop some reviewers". Five reviewers reading a mis-scoped file list is the most expensive mistake this skill can make, and it is cheap to prevent here.

Skip this checkpoint only when the user asked for an unattended run.

## Phase 4: Dispatch Reviewers

Standard mode is the default: launch the selected reviewers in a single message so they run concurrently.

Workflow mode is opt-in only, when the user asks for a thorough, comprehensive, or deep review, or says "workflow mode". Be honest about cost before starting: it spawns roughly 10-20 agents. Structure it as a `pipeline()` — one `agent()` per selected role returning schema-validated findings from [findings-schema.md](findings-schema.md), each role's findings flowing straight into adversarial verification without waiting for the other roles. Verifiers are prompted to **refute**; a finding that survives keeps its severity, one that does not is downgraded with the doubt recorded rather than dropped. Verify only findings marked `needs_verification`, which includes every blocker.

Workflow mode does not remove the Phase 3 checkpoint — it makes it more valuable, since a mis-scoped file list now costs three times the agents.

Workflow agents are not persistent, so a follow-up deep-dive after workflow mode spawns a fresh focused pass rather than continuing a reviewer.

In both modes, build each prompt from the matching role file with the `[REVIEW CONTEXT]` block from Phase 3 filled in.

Keep only findings in main context.

## Phase 5: Consolidate

Once every dispatched reviewer has returned:

- Group findings by severity.
- Note agreements across reviewers — two reviewers reaching the same conclusion independently carries more weight than one.
- Note conflicting assessments and resolve them with your own judgment, saying which way you went and why.
- If a dispatched reviewer produced no findings, say so explicitly rather than omitting it — a silent omission reads as "clean" when it may mean "did not run".

### Severity Tiers

- **Blocker**: must be addressed before merge/acceptance. The feature is broken, unsafe, or violates project invariants.
- **Significant**: should be addressed. The feature works but has meaningful gaps in testing, security, documentation, or completeness.
- **Minor**: nice to have. Suggestions that don't block acceptance.
- **Note**: observations for awareness. No action required.

## Phase 6: Deliver The Verdict

End with exactly one of:

- **ACCEPT** — the feature is ready as-is.
- **ACCEPT WITH CONDITIONS** — acceptable if specific, enumerated conditions are met. List each condition.
- **REQUEST CHANGES** — the feature has blockers that must be resolved. List each blocker.

Report format:

```
# Feature Acceptance Review: [subject]

## Summary
<2-3 sentence overview of what was reviewed and the verdict>

## Verdict: <ACCEPT | ACCEPT WITH CONDITIONS | REQUEST CHANGES>

### Blockers
<numbered list with description and rationale, or "None">

### Significant Issues
<numbered list with description and rationale, or "None">

### Minor Suggestions
<numbered list, or "None">

### Notes
<numbered list, or "None">

## Reviewer Reports

### Product
<findings, or "not selected">

### Safety
<findings, or "not selected">

### Quality
<findings, or "not selected">

### Maintainability
<findings, or "not selected">

### Documentation
<findings, or "not selected">
```

You read each reviewer's report directly, so preserve its substance — rationale and file references included — rather than compressing five reports into bullet points.

Offer to deep-dive on any section. Do not apply fixes or post the report to the PR unless the user explicitly asks in a later turn, and treat that as leaving this skill.

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| Forking the whole skill into a subagent | Run inline; the checkpoints need `AskUserQuestion`. |
| Guessing PR vs feature mode on an ambiguous subject | Ask. Five reviewers on the wrong subject is the expensive failure. |
| Dispatching reviewers before confirming scope | Checkpoint the context summary first. |
| Dispatching reviewers sequentially | Send them in one message so they run concurrently. |
| Reaching for workflow mode unprompted | It is opt-in; state the agent cost before starting. |
| Compressing reviewer reports into one-liners | You have the full text in context — preserve the substance. |
| Reviewing code style instead of feature acceptance | That is the go-review skill's job. |
| Posting the verdict to the PR | Read-only. Present in chat. |
| Silently dropping a reviewer that found nothing | Say "no findings" explicitly. |
| A blocker vanishing during verification | Downgrade it visibly and say why — the verdict turns on it. |

## Red Flags

- You are about to run a command that modifies a file, git state, or the PR.
- You are about to default to feature mode on an ambiguous subject instead of asking.
- Your verdict does not follow from the findings you listed.
- You are reporting a blocker with no file reference.
