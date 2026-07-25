---
name: go-review
description: Run a read-only Go code review across structure, error handling, style, and security. Usage - $go-review [path] [focus]. Examples - $go-review, $go-review ./cmd/server, $go-review . security, $go-review ./pkg error,style
---

# Go Review

Run a read-only Go code review for production source files. You are the review
orchestrator: you scope the review, fan out one reviewer per focus area, and
consolidate their findings into a single prioritized report.

This review is **read-only**. Do not modify files, stage changes, apply fixes,
or mutate git state. The only output is the report.

## Arguments

Arguments are free text after the skill mention:

- **path** (optional): directory or file to scope the review. Default to `.`.
- **focus** (optional): comma-separated reviewer list. Valid values are
  `structure`, `error`, `style`, and `security`. Default to all four. Reject
  unknown values with a concise usage note.

## Workflow

### 1. Enumerate files

Find non-test Go files under the scoped path. Prefer `rg --files <path>`
filtered to `.go` and excluding `*_test.go`; use `find` only if `rg` is
unavailable. Test files are out of scope.

If no production Go files are found, report that and stop.

If the file list is very large (roughly 150+ files), state the count and offer
to narrow the path before spending four full reviewer passes on it.

### 2. Build the review context

Create a context block containing:

```
[REVIEW CONTEXT]
- Repo root: <absolute path>
- Scope path: <path or "." if not specified>
- Files to review: <the enumerated non-test .go files>
```

### 3. Fan out reviewers in parallel

The role briefs live beside this file — one per focus area:

- `<skill-dir>/roles/structure-reviewer.md`
- `<skill-dir>/roles/error-reviewer.md`
- `<skill-dir>/roles/style-reviewer.md`
- `<skill-dir>/roles/security-reviewer.md`

Use the native subagent tools: call `spawn_agent` once per selected focus area,
then collect every reviewer with the subagent wait tool (`wait_agent`). Four
reviewers fit within the default concurrent-thread limit. Each spawn prompt
must contain:

1. The absolute path of that reviewer's role file, with the instruction to read
   it and follow it as its complete brief. The role files are self-contained
   prompts — they carry their own read-only gate, output format, and severity
   scale.
2. The `[REVIEW CONTEXT]` block from step 2, which fills the role file's
   `[REVIEW CONTEXT]` placeholder.
3. A restatement of the constraints: the reviewer is read-only and must not
   spawn further agents.

**Fallback:** if subagent spawning is unavailable, blocked, or declined, run
the same role briefs yourself, sequentially, with identical inputs and output
contract. State in the final report which mode was used.

### 4. Consolidate

- Deduplicate. The same issue is often flagged by two reviewers — merge into
  one finding and note the agreement, which raises its weight.
- Map each finding onto a priority tier.
- Keep concrete `file/path.go:LINE` references. A finding without a location is
  not actionable — either locate it or drop it.
- Prefer fewer high-signal items over exhaustive noise.
- If a selected role produced no findings, say so briefly rather than omitting
  it — a silent omission reads as "clean" when it may mean "did not run".

## Priority Tiers

- **P0 (Bug risk):** runtime failures, exploitable vulnerabilities, data races,
  or silent data loss.
- **P1 (Robustness):** missing error checks, resource leaks, context
  propagation gaps, defensive hardening, or medium-risk security findings.
- **P2 (Maintainability):** duplication, large functions, unclear abstractions,
  package coupling, and meaningful simplification opportunities.
- **P3 (Style):** naming, idioms, comments, documentation, and low-risk
  cleanup.

## Output Format

Output one numbered report grouped by priority tier. Each finding uses:

```
N. file/path.go:LINE — [Category]
   Description of the issue.
   Suggested fix: concrete recommendation.
```

Offer to deep-dive on any finding. Do not apply fixes unless the user
explicitly asks in a later turn, and treat that as leaving this skill.
