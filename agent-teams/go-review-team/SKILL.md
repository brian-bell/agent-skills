---
name: go-review
description: Run a Go code review. Usage - /go-review [path] [focus]. Examples - /go-review, /go-review ./cmd/server, /go-review . security, /go-review ./pkg error,style
user_invocable: true
---

# Go Review

Run a read-only Go code review for production source files. Arguments are:

- **path**: optional directory or file scope. Default to `.`.
- **focus**: optional comma-separated reviewer list. Valid values are
  `structure`, `error`, `style`, and `security`. Default to `all`.

Follow only the platform block for the runtime you are using.

## Platform — Claude Code

Invoke the `review-lead` agent to coordinate the review.

Pass the user's arguments directly in the agent prompt:

```
Review this Go project.
Path: <path or "." if not specified>
Focus: <comma-separated reviewers or "all" if not specified>
```

Use the Agent tool with `subagent_type: "review-lead"`.

## Platform — Codex

Run the review inline in the main agent unless the user explicitly asks for
delegation and the current Codex surface exposes a safe delegation mechanism.
The review is read-only: do not modify files, stage changes, or apply fixes.

1. Parse the optional path and focus arguments. Default path to `.` and focus to
   `all`. Reject unknown focus values with a concise usage note.
2. Enumerate non-test Go files under the scoped path. Prefer `rg --files <path>`
   and filter to `.go` files while excluding `*_test.go`; use `find` only if
   `rg` is unavailable. If no production Go files are found, report that and
   stop.
3. Read the reviewer checklist files from `<skill-dir>` for the selected focus
   areas:
   - `structure-reviewer.md`
   - `error-reviewer.md`
   - `style-reviewer.md`
   - `security-reviewer.md`
   Use those files as checklist source material only. Ignore their frontmatter,
   tool lists, model settings, task-completion directions, and any instruction
   to report back to a team lead.
4. Run one review pass per selected checklist. For each pass, inspect the listed
   production Go files and use repository searches where the checklist calls for
   broad patterns. Keep the pass read-only.
5. Consolidate findings across passes. Deduplicate overlapping findings, keep
   concrete `file/path.go:LINE` references, and prefer fewer high-signal items
   over exhaustive noise.
6. Output one numbered report grouped by priority:
   - **P0 (Bug risk):** runtime failures, exploitable vulnerabilities, data
     races, or silent data loss.
   - **P1 (Robustness):** missing error checks, resource leaks, context
     propagation gaps, defensive hardening, or concrete security medium-risk
     findings.
   - **P2 (Maintainability):** duplication, large functions, unclear
     abstractions, package coupling, and meaningful simplification opportunities.
   - **P3 (Style):** naming, idioms, comments, documentation, and low-risk
     cleanup.

Each finding should use:

```
N. file/path.go:LINE — [Category]
   Description of the issue.
   Suggested fix: concrete recommendation.
```

If a selected checklist produces no findings, say so briefly in the final
report. Do not review `*_test.go` files.
