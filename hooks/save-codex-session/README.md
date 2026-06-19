# save-codex-session hook

A Codex **`Stop`** hook that archives each local Codex session transcript plus a
metadata sidecar when a turn stops.

- `save-session.sh` - the hook script (installed as a symlink, so this repo stays
  the single source of truth).
- `install.sh` - installs/uninstalls the hook (symlink + `hooks.json` merge).
- `backfill.sh` - imports existing Codex transcripts into the same archive store.

## What it captures

Codex command hooks receive a JSON payload on stdin. `save-session.sh` accepts
session identifiers and transcript paths from the payload when present, and can
also find Codex transcripts in the normal local store:

```
~/.codex/sessions/YYYY/MM/DD/rollout-<timestamp>-<session-id>.jsonl
~/.codex/archived_sessions/rollout-<timestamp>-<session-id>.jsonl
```

The script writes to:

```
~/.agent-sessions/codex/<session-id>/
    transcript.jsonl   # a copy of the Codex transcript
    metadata.json      # archive stats + raw hook payload + parsed session_meta
```

The folder is keyed by session id. `Stop` can run more than once for a thread,
so the script updates one folder per session in place. A failed, empty, or
shorter transcript copy never clobbers a longer existing `transcript.jsonl`.

A run log is appended to `~/.agent-sessions/codex/save-session.log`. The script
always exits 0 and logs problems instead of failing, so it should not break a
Codex session.

## Install

```bash
./install.sh
./install.sh --force
./install.sh --uninstall
```

Requires `jq` to safely edit `~/.codex/hooks.json`. The installer backs up
`hooks.json` before edits and is idempotent. It installs a symlink at
`~/.codex/hooks/save-session.sh` and adds a `Stop` command hook to
`~/.codex/hooks.json`.

Codex requires non-managed command hooks to be reviewed and trusted before they
run. After installing, start a fresh Codex session and run `/hooks` if prompted.

## Backfill existing sessions

The hook only archives sessions after it is installed and trusted. To import
existing local Codex transcripts, run:

```bash
./backfill.sh              # copy sessions not already in the store
./backfill.sh --dry-run    # show what would happen, write nothing
./backfill.sh --update     # refresh sessions whose source transcript grew
./backfill.sh --force      # re-copy every session unconditionally
```

Backfill scans `~/.codex/sessions` and `~/.codex/archived_sessions`. Each
imported session lands under `~/.agent-sessions/codex/<session-id>/` with
`metadata.json` tagged `"source": "backfill"`.

## Manual configuration

If you want to wire the hook by hand, add this to `~/.codex/hooks.json`:

```json
{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "$HOME/.codex/hooks/save-session.sh",
            "timeout": 30,
            "statusMessage": "Saving Codex session"
          }
        ]
      }
    ]
  }
}
```

## Customize

- `CODEX_SESSION_ARCHIVE_DIR` overrides the archive location.
- `CODEX_HOME` overrides the Codex state root. Default: `~/.codex`.

## Verify

```bash
find ~/.agent-sessions/codex -type f | sort
cat ~/.agent-sessions/codex/save-session.log
find ~/.agent-sessions/codex -name metadata.json | sort | tail -1 \
  | xargs jq '{source, archived_at, transcript_copied, transcript_lines, session: .session.session_id}'
```
