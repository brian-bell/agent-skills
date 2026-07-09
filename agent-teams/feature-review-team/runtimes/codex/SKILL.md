---
name: feature-review
description: Run a feature acceptance review. Usage - $feature-review <PR number or feature name> [focus]. Examples - $feature-review #42, $feature-review scanner, $feature-review #15 safety,quality
---

# Feature Review

Run a read-only feature acceptance review. You are the acceptance lead: you
gather context, fan out one reviewer per focus area, and consolidate their
findings into a verdict.

This review is **read-only**. Do not modify files, stage changes, apply
fixes, or post PR comments. The only output is the report.

## Arguments

Arguments are free text after the skill mention:

- **subject** (required): a PR number (`#42`, `PR 42`, `pull request 42`)
  selects **PR mode**; anything else is a feature name and selects
  **feature mode**. If ambiguous, default to feature mode.
- **focus** (optional): comma-separated reviewer list. Valid values are
  `product`, `safety`, `quality`, `maintainability`, and `documentation`.
  Default to all five. Reject unknown values with a concise usage note.

## GitHub Access

Prefer an installed GitHub connector for PR metadata and diffs when
available. Use `gh` when connector coverage is insufficient.

## Workflow

### 1. Discover the project

1. Read `AGENTS.md` if it exists, falling back to `CLAUDE.md` — the primary
   source of architecture context.
2. Read `README.md` if it exists.
3. Scan for framework/language markers (`go.mod`, `package.json`,
   `pyproject.toml`, `Cargo.toml`, and similar) and note the language,
   framework, and key dependencies.
4. Identify architecture patterns, directory conventions, and the testing
   approach from the docs and file layout.

### 2. Gather feature context

**PR mode:** fetch the PR title, body, base/head refs, changed-file list,
and full diff (connector reads, or `gh pr view <N>`, `gh pr view <N> --json
files`, and `gh pr diff <N>`).

**Feature mode:** locate directories matching the feature name, search for
references to it across source files, and build a file list including test
files and cross-cutting references (modules that import or depend on the
feature).

### 3. Build the context summary

Create a structured context block containing: project type; review mode;
subject; description (PR body or feature purpose); key files; related
files; test files; project patterns reviewers should check against; and
statistics (PR: additions/deletions/files changed; feature: total files,
total lines, test-file count).

### 4. Fan out reviewers in parallel

The checklists live beside this file — one per focus area:

- `<skill-dir>/product-reviewer.md`
- `<skill-dir>/safety-reviewer.md`
- `<skill-dir>/quality-reviewer.md`
- `<skill-dir>/maintainability-reviewer.md`
- `<skill-dir>/documentation-reviewer.md`

Use the native subagent tools: call `spawn_agent` once per selected focus
area, then collect every reviewer with the subagent wait tool
(`wait_agent`). Five reviewers fit within the default concurrent-thread
limit. Each spawn prompt must contain:

1. The review mode and the full context summary from step 3.
2. The absolute path of that reviewer's checklist file, with this
   instruction: read the file and treat it as
   **checklist source material only** — ignore its frontmatter, tool
   lists, model settings, task-completion directions, and any instruction
   to report to a team lead.
3. The output contract: a findings list where each finding has a severity
   (`blocker`, `significant`, `minor`, or `note`), a description with
   file:line references, the scenario where it manifests, and a suggested
   fix — plus a short overall assessment.
4. Constraints: the reviewer is read-only and must not spawn further
   agents.

**Fallback:** if subagent spawning is unavailable, blocked, or declined,
run the same checklist passes yourself, sequentially, with identical
inputs and output contract. State in the final report which mode was used.

### 5. Consolidate

- Group findings by severity.
- Note agreements across reviewers (these carry more weight) and
  conflicting assessments (resolve with your judgment).
- If a selected checklist produced no findings, say so briefly.

## Severity Tiers

- **Blocker**: must be addressed before merge/acceptance. The feature is
  broken, unsafe, or violates project invariants.
- **Significant**: should be addressed. The feature works but has
  meaningful gaps in testing, security, documentation, or completeness.
- **Minor**: nice to have. Suggestions that don't block acceptance.
- **Note**: observations for awareness. No action required.

## Verdict

End the report with one of:

- **ACCEPT** — feature is ready as-is.
- **ACCEPT WITH CONDITIONS** — acceptable if specific, enumerated
  conditions are met. List each condition.
- **REQUEST CHANGES** — feature has blockers that must be resolved. List
  each blocker.

## Output Format

The report consolidates the work of the reviewers — preserve the substance
of each reviewer's findings, including rationale and file references. Do
not truncate or over-summarize.

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
<full findings, or "not selected">

### Safety
<full findings, or "not selected">

### Quality
<full findings, or "not selected">

### Maintainability
<full findings, or "not selected">

### Documentation
<full findings, or "not selected">
```
