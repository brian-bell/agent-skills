---
name: go-review
description: Run a Go code review. Usage - /go-review [path] [focus]. Examples - /go-review, /go-review ./cmd/server, /go-review . security, /go-review ./pkg error,style
user_invocable: true
---

Invoke the `review-lead` agent to coordinate a Go code review.

Pass the user's arguments directly in the agent prompt:

```
Review this Go project.
Path: <path or "." if not specified>
Focus: <comma-separated reviewers or "all" if not specified>
```

Use the Agent tool with `subagent_type: "review-lead"`.
