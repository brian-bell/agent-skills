PRAGMA foreign_keys = ON;
BEGIN IMMEDIATE;

CREATE TABLE directors (
    id TEXT PRIMARY KEY,
    thread_id TEXT NOT NULL UNIQUE,
    repo_key TEXT NOT NULL,
    checkout_path TEXT NOT NULL,
    codex_project_id TEXT,
    charter TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'retired')),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
CREATE UNIQUE INDEX one_active_director_per_repo
    ON directors(repo_key) WHERE status = 'active';

CREATE TABLE work_items (
    id TEXT PRIMARY KEY,
    director_id TEXT NOT NULL REFERENCES directors(id),
    kind TEXT NOT NULL CHECK (kind IN ('investigate', 'plan', 'implement')),
    title TEXT NOT NULL,
    outcome TEXT NOT NULL,
    acceptance_criteria TEXT NOT NULL,
    constraints TEXT NOT NULL,
    authorization TEXT NOT NULL,
    context TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'underway', 'blocked', 'done', 'cancelled')),
    result TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    CHECK (status != 'done' OR (result IS NOT NULL AND length(trim(result)) > 0))
);
CREATE INDEX work_by_director ON work_items(director_id, status);

CREATE TABLE assignments (
    id TEXT PRIMARY KEY,
    work_id TEXT NOT NULL REFERENCES work_items(id),
    worker_kind TEXT NOT NULL CHECK (worker_kind IN ('task', 'subagent')),
    worker_id TEXT,
    host_id TEXT,
    creation_ref TEXT,
    status TEXT NOT NULL DEFAULT 'reserved'
        CHECK (status IN ('reserved', 'active', 'finished', 'unavailable')),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    CHECK (status != 'active' OR (worker_id IS NOT NULL AND length(trim(worker_id)) > 0))
);
CREATE UNIQUE INDEX one_live_assignment_per_work
    ON assignments(work_id) WHERE status IN ('reserved', 'active');

CREATE TABLE events (
    id INTEGER PRIMARY KEY,
    work_id TEXT NOT NULL REFERENCES work_items(id),
    kind TEXT NOT NULL CHECK (kind IN ('decision', 'authorization', 'transition', 'blocker', 'evidence')),
    detail TEXT NOT NULL,
    source TEXT NOT NULL,
    created_at INTEGER NOT NULL
);
CREATE INDEX events_by_work ON events(work_id, id);

PRAGMA user_version = 1;
COMMIT;
