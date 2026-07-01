---
name: slice-issues
description: Break a GitHub issue into independently-grabbable sub-issues using tracer-bullet vertical slices. Use when user wants to slice an issue, create implementation tickets, or break down an issue into work items.
---

# Slice Issues

Break a large GitHub issue into independently-grabbable sub-issues using vertical slices (tracer bullets).

On Codex, prefer an installed GitHub connector when available; use `gh` when connector coverage is insufficient or unavailable. On Claude Code, use `gh`/CLI unless the user provides another integration.

## Process

### 1. Locate the issue

Ask the user for the GitHub issue number (or URL).

If the issue is not already in your context window, fetch it with the GitHub connector on Codex when available; otherwise use `gh issue view <number> --comments` so GitHub comments are included. Use structured connector output or `gh issue view <number> --json title,body,comments` if you need structured data.

### 2. Explore the codebase (optional)

If you have not already explored the codebase, do so to understand the current state of the code.

### 3. Draft vertical slices

Break the issue into **tracer bullet** sub-issues. Each issue is a thin vertical slice that cuts through ALL integration layers end-to-end, NOT a horizontal slice of one layer.

Slices may be 'HITL' or 'AFK'. HITL slices require human interaction, such as an architectural decision or a design review. AFK slices can be implemented and merged without human interaction. Prefer AFK over HITL where possible.

<vertical-slice-rules>
- Each slice delivers a narrow but COMPLETE path through every layer (schema, API, UI, tests)
- A completed slice is demoable or verifiable on its own
- Prefer many thin slices over few thick ones
</vertical-slice-rules>

### 4. Quiz the user

Present the proposed breakdown as a numbered list. For each slice, show:

- **Title**: short descriptive name
- **Type**: HITL / AFK
- **Blocked by**: which other slices (if any) must complete first
- **User stories covered**: which user stories from the issue this addresses. Infer a set of user stories if one does not exist in the issue.

Ask the user:

- Does the granularity feel right? (too coarse / too fine)
- Are the dependency relationships correct?
- Should any slices be merged or split further?
- Are the correct slices marked as HITL and AFK?

Iterate until the user approves the breakdown.

### 5. Create the GitHub issues

For each approved slice, create a GitHub sub-issue with the GitHub connector on Codex when available; otherwise use `gh issue create --parent <issue-number>`. Use the issue body template below, preserving the approved HITL/AFK type in the created issue body.

Create issues in dependency order (blockers first) so you can reference real issue numbers in the "Blocked by" field.

<issue-template>
## Parent

#<issue-number>

## Type

HITL or AFK. Use the exact classification approved in step 4.

## What to build

A concise description of this vertical slice. Describe the end-to-end behavior, not layer-by-layer implementation. Reference specific sections of the parent issue rather than duplicating content.

## Acceptance criteria

- [ ] Criterion 1
- [ ] Criterion 2
- [ ] Criterion 3

## Blocked by

- Blocked by #<issue-number> (if any)

Or "None - can start immediately" if no blockers.

## User stories addressed

Reference by number from the parent issue:

- User story 3
- User story 7

</issue-template>

Do NOT close or modify the parent issue.
