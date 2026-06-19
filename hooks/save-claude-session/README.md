# save-session hook

A Claude Code **`SessionEnd`** hook that archives each session's transcript plus
a metadata sidecar when the session ends.

- `save-session.sh` — the hook script (installed as a symlink, so this repo stays
  the single source of truth).
- `install.sh` — installs/uninstalls the hook (symlink + `settings.json` merge).

## What it captures

On `SessionEnd`, Claude Code pipes a JSON payload to the script on stdin
(`session_id`, `transcript_path`, `cwd`, `reason`, `permission_mode`,
`hook_event_name`). The script writes to:

```
~/.agent-sessions/claude/<session-id>/
    transcript.jsonl   # a copy of the live transcript (JSON Lines)
    metadata.json      # archive stats + the full raw hook payload
```

The folder is keyed by session id. `SessionEnd` fires repeatedly during a
session (with `reason: prompt_input_exit`), so the script updates one folder per
session **in place** rather than accumulating snapshots — the last fire leaves
the most complete transcript. A failed/empty copy never clobbers a good prior
`transcript.jsonl`.

A run log is appended to `~/.agent-sessions/claude/save-session.log`. The script
**always exits 0** and logs problems instead of failing, so it can never block or
break a session (`SessionEnd` hooks can't block anyway — they're cleanup-only).

## Install

```bash
./install.sh            # symlink + add the SessionEnd entry to settings.json
./install.sh --force    # also replace a non-repo file already at the target
./install.sh --uninstall
```

Requires `jq` (used to safely merge `settings.json`). `settings.json` is backed
up before any edit. The installer is idempotent — re-running won't duplicate the
hook entry. It installs into `~/.claude` only, since hooks are Claude Code–specific.

> **Note:** Hooks are snapshotted at session start. After installing, start a
> fresh session (or run `/hooks` and reload) before the hook will fire.

## Backfill existing sessions

The hook only archives sessions that end *after* it is installed. To import
sessions that already exist in Claude Code's default transcript location
(`~/.claude/projects/<encoded-cwd>/<session-id>.jsonl`) into the same store, run:

```bash
./backfill.sh              # copy any session not already in the store (idempotent)
./backfill.sh --dry-run    # show what would happen, write nothing
./backfill.sh --update     # also refresh sessions whose source transcript grew
./backfill.sh --force      # re-copy every session unconditionally
```

Each imported session lands at `~/.agent-sessions/claude/<session-id>/` with a
`metadata.json` tagged `"source": "backfill"` and fields synthesized from the
transcript itself (`cwd`, `git_branch`, `version`, `started_at`, `ended_at`).
By default existing session folders are left untouched, so it's safe to run
alongside the live hook. Honors the same `CLAUDE_SESSION_ARCHIVE_DIR` override,
plus `CLAUDE_PROJECTS_DIR` for the source root.

## Manual configuration

If you'd rather wire it up by hand, add this to `~/.claude/settings.json` (the
script can live anywhere; point the command at it):

```json
{
  "hooks": {
    "SessionEnd": [
      {
        "matcher": "",
        "hooks": [
          { "type": "command", "command": "$HOME/.claude/hooks/save-session.sh" }
        ]
      }
    ]
  }
}
```

`matcher: ""` runs on every session end. To archive only certain endings, set
`matcher` to a `reason` value (`clear`, `logout`, `prompt_input_exit`, `other`,
…). Or use the `/hooks` slash command for an interactive editor.

## Customize

- **Archive location:** set `CLAUDE_SESSION_ARCHIVE_DIR` (e.g. under `"env"` in
  `settings.json`, or in your shell) to override the default
  `~/.agent-sessions/claude`.
- **Capture session start too:** add a matching `SessionStart` block pointing at
  the same script. On `SessionStart` the payload uses `source`
  (`startup`/`resume`/`clear`/`compact`) instead of `reason`; the script already
  falls back to `source`, though the transcript may be empty early in a session.

## Verify

```bash
find ~/.agent-sessions/claude -type f | sort
cat ~/.agent-sessions/claude/save-session.log
find ~/.agent-sessions/claude -name metadata.json | sort | tail -1 \
  | xargs jq '{archived_at, transcript_copied, transcript_lines, reason: .hook_payload.reason}'
```
