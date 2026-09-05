# Durable coordination state

Use `~/.local/state/codex-director/director.db`, independent of the repository,
Codex home, and other coordinator skills. Director owns coordination writes;
workers return evidence. Use SQLite directly through `sqlite3` or the Python
standard library. No service or production dependency is required.

## Initialization

Create the state directory if absent. Inspect `PRAGMA user_version` and the
existing tables before writing. Only initialize a new, empty database using
the bundled [schema.sql](../schema.sql). It creates version 1 atomically and
intentionally fails on existing tables rather than overwriting state.

For an existing version 1 database, inspect its tables, indexes, and constraints
against the bundled schema before use. A different version or incompatible
schema is a storage blocker: report it without replacing, deleting, or silently
migrating the database. Run integrity checks when corruption is suspected.

Enable `PRAGMA foreign_keys = ON` and a bounded `busy_timeout` on every
connection. Use SQL parameter binding for text, especially prompts and user
decisions. Never interpolate that text into shell commands or SQL. Use short
transactions; do not hold a transaction open while waiting on a tool or worker.

## Identity and records

Use UUIDs for Director, work, and assignment IDs; timestamps are UTC Unix
seconds. Derive a stable repository key from the verified remote host and
repository path, normalizing equivalent SSH/HTTPS URLs and the `.git` suffix
without blindly lowercasing case-sensitive paths. Without a remote, use the
canonical Git common directory, or canonical project directory for non-Git
work. Check existing records before treating a changed remote as a new project.

- `directors` binds the current task to its repository, checkout, optional
  saved project ID, and plain-language charter. One active Director per repo is
  the default; the schema enforces that initial design.
- `work_items` stores the current work order, authorized actions with their
  source, context, status, and result. Keep prerequisite work IDs and issue
  references in context; do not invent an issue hierarchy or scope enum.
- `assignments` records dispatch reservations and the actual worker mechanism
  and ID. `creation_ref` can hold a pending task creation reference; it is not
  a usable task ID. `host_id` identifies a task's host when returned.
- `events` preserves decisions, authorizations, blockers, transitions, and
  evidence with a source such as a user message, worker task, or verified check.
  Append events; do not rewrite history to match the latest order.

Do not register a second Director when another active task owns the same repo.
Inspect that task and report the existing owner. Transfer ownership only when
the user requests it, recording the handoff in the charter and preserving work
and assignment IDs. Never steal ownership because a task is quiet. The current
runtime must supply or expose a verified task ID; do not guess one from a title.

## Transitions

Create work as `queued`. Reserve its assignment transactionally, then dispatch
outside the transaction. Save the returned ID, activate the assignment, and
mark work `underway` together. Read affected records after each write.

Work is `blocked` only for a concrete impediment recorded in an event. Resume
as `queued` before dispatch or `underway` with an active worker. Mark `done`
only after Director verifies acceptance evidence and stores a useful result.
Mark `cancelled` only when the work has been withdrawn; coordinate worker
stoppage before treating an active assignment as finished.

After a result, set the assignment to `finished` while retaining its worker ID.
Reuse that worker for corrections or a new authorized stage of the same work:
append the previous result and the new authorization to events, update kind and
order, clear the current result, and move the item to `queued`. Reactivate the
existing assignment when continuing the accessible worker. The unique index
prevents two live assignments for one work item.

Keep changes to an order, its assignment, and corresponding events in one
transaction where they represent one transition. On contention, reread state
before retrying. A stale observation is not authority to replace another write.

## Recovery

At the start of a resumed turn, load the Director charter, unfinished work,
relevant events, and assignments. Check actual worker status and reconcile it
with the ledger before dispatching anything new.

- A reserved assignment may represent a successful creation whose reply was
  lost. Inspect task listings and initial prompts for the exact assignment ID,
  and resolve any pending creation reference. Titles alone are insufficient.
  Record a verified match rather than creating another worker. If the outcome
  remains uncertain, keep it reserved and report the routing blocker.
- An active worker may already have finished. Retrieve its result and verify
  it; do not restart work merely because SQLite still says `underway`.
- Session subagents may not survive a restart. Verify availability using the
  recorded mechanism. Preserve recovered results, mark a confirmed lost
  assignment `unavailable`, and block the work. If the worker is definitively
  gone and the existing delegation authority permits replacement, reserve a
  replacement for the same work ID and pass the durable evidence. Uncertainty
  about a persistent task is not evidence that it is gone.

SQLite preserves coordination, not proof of completion. Recheck the repository,
issue tracker, or required checks when the result depends on their current
state. Keep useful report text in the result or reference durable files under
the state directory. Never rely solely on expiring scratch files or task titles.
