---
name: feature-review
description: Run a read-only feature acceptance review. Usage - $feature-review <PR number or feature name> [focus]. Examples - $feature-review #42, $feature-review scanner, $feature-review #15 safety,quality
---

# Feature Review

Run a read-only feature acceptance review. You are the acceptance lead: you
gather context, fan out one reviewer per focus area, and consolidate their
findings into a verdict.

This review is **read-only**. Do not modify files, mutate git state, apply
fixes, or post PR comments. The only output is the report.

## Arguments

Arguments are free text after the skill mention:

- **subject** (required): a PR number (`#42`, `PR 42`, `pull request 42`)
  selects **PR mode**; anything else is a feature name and selects
  **feature mode**. If the subject is ambiguous or missing, ask the user
  which reading is meant rather than guessing — five reviewers on the wrong
  subject is the most expensive mistake this skill can make.
- **focus** (optional): comma-separated reviewer list. Valid values are
  `product`, `safety`, `quality`, `maintainability`, and `documentation`.
  Default to all five. Reject unknown values with a concise usage note.

## GitHub Access

Prefer an installed GitHub connector for PR metadata and diffs when
available. Use `gh` when connector coverage is insufficient. Reads only:
`gh pr view` and `gh pr diff` are expected; commands that post or change PR
state are forbidden.

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

### 3. Build the review context

Assemble the block the role briefs expect:

```
[REVIEW CONTEXT]
- Review mode: PR | Feature
- Subject: <PR number and title, or feature name>
- Project type: <language, framework, architecture style>
- Description: <PR body or feature purpose summary>
- Key files: <changed files for PR, module files for feature>
- Related files: <modules that import or interact with the feature>
- Test files: <corresponding tests>
- Project patterns: <key patterns from AGENTS.md/README.md>
- Statistics: <PR: additions/deletions/files changed; feature: total files, lines, test count>
```

Confirm the scope with the user before dispatching if anything about the
subject or file list is uncertain.

### 4. Fan out reviewers in parallel

The role briefs live beside this file — one per focus area:

- `<skill-dir>/roles/product-reviewer.md`
- `<skill-dir>/roles/safety-reviewer.md`
- `<skill-dir>/roles/quality-reviewer.md`
- `<skill-dir>/roles/maintainability-reviewer.md`
- `<skill-dir>/roles/documentation-reviewer.md`

Use the native subagent tools: call `spawn_agent` once per selected focus
area, then collect every reviewer with the subagent wait tool
(`wait_agent`). Five reviewers fit within the default concurrent-thread
limit. Each spawn prompt must contain:

1. The absolute path of that reviewer's role file, with the instruction to
   read it and follow it as its complete brief. The role briefs are
   self-contained prompts — they carry their own read-only gate, output
   format, and severity scale.
2. The `[REVIEW CONTEXT]` block from step 3, which fills the role brief's
   `[REVIEW CONTEXT]` placeholder.
3. A restatement of the constraints: the reviewer is read-only, must not
   post to the PR, and must not spawn further agents.

**Fallback:** if subagent spawning is unavailable, blocked, or declined,
run the same role briefs yourself, sequentially, with identical inputs and
output contract. State in the final report which mode was used.

**Partial fan-out:** treat each role independently. If some spawns succeed
and others are rejected — a concurrency limit, a declined approval, a
worker that dies without returning — do not abandon the run and do not
silently drop the missing roles. Wait for the ones that launched, then run
every role that did not return inline, sequentially. Every selected role
must produce findings from exactly one of the two paths before you
consolidate. Name any role that fell back in the final report; a missing
safety pass that nobody mentions reads as a clean safety result.

### 5. Consolidate

- Group findings by severity.
- Note agreements across reviewers (these carry more weight) and
  conflicting assessments (resolve with your judgment, and say which way
  you went).
- If a selected role produced no findings, say so briefly rather than
  omitting it — a silent omission reads as "clean" when it may mean "did
  not run".

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
of each reviewer's findings, including rationale and file references.

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
