#!/usr/bin/env bash
#
# save-session.sh — Claude Code SessionEnd hook
#
# Archives the session transcript plus a metadata sidecar to:
#   ~/.agent-sessions/claude/<session-id>/
#       transcript.jsonl   (a copy of the live transcript)
#       metadata.json      (session metadata + archive stats)
#
# Keyed by session id, so the SessionEnd event firing repeatedly during a
# session (reason=prompt_input_exit) updates one folder in place instead of
# piling up snapshots; the last fire leaves the most complete transcript.
#
# Reads the hook event JSON from stdin. Designed to never disrupt Claude Code:
# it always exits 0 and logs any problems to <archive-root>/save-session.log.
#
# See README.md for how to install and wire this into ~/.claude/settings.json.

set -uo pipefail

ARCHIVE_ROOT="${CLAUDE_SESSION_ARCHIVE_DIR:-$HOME/.agent-sessions/claude}"
LOG_FILE="$ARCHIVE_ROOT/save-session.log"

mkdir -p "$ARCHIVE_ROOT" 2>/dev/null

log() { printf '%s %s\n' "$(date '+%Y-%m-%dT%H:%M:%S%z')" "$*" >>"$LOG_FILE" 2>/dev/null; }

# Read the full hook payload from stdin.
INPUT="$(cat)"

if [ -z "$INPUT" ]; then
  log "WARN: empty stdin payload; nothing to do"
  exit 0
fi

# jq is the happy path; fall back to a minimal grep if it is missing.
if command -v jq >/dev/null 2>&1; then
  session_id="$(printf '%s' "$INPUT"   | jq -r '.session_id      // ""')"
  transcript="$(printf '%s' "$INPUT"   | jq -r '.transcript_path // ""')"
  cwd="$(printf '%s' "$INPUT"          | jq -r '.cwd             // ""')"
  reason="$(printf '%s' "$INPUT"       | jq -r '.reason          // .source // ""')"
  event="$(printf '%s' "$INPUT"        | jq -r '.hook_event_name // ""')"
  perm_mode="$(printf '%s' "$INPUT"    | jq -r '.permission_mode // ""')"
else
  log "WARN: jq not found; using fallback parser"
  extract() { printf '%s' "$INPUT" | grep -o "\"$1\"[[:space:]]*:[[:space:]]*\"[^\"]*\"" | head -1 | sed 's/.*:[[:space:]]*"\(.*\)"/\1/'; }
  session_id="$(extract session_id)"
  transcript="$(extract transcript_path)"
  cwd="$(extract cwd)"
  reason="$(extract reason)"; [ -z "$reason" ] && reason="$(extract source)"
  event="$(extract hook_event_name)"
  perm_mode="$(extract permission_mode)"
fi

short_id="${session_id:0:8}"; [ -z "$short_id" ] && short_id="unknown"
archived_at="$(date '+%Y-%m-%dT%H:%M:%S%z')"

# Key the archive by session_id so repeated SessionEnd fires (e.g. the recurring
# prompt_input_exit reason) update one folder per session in place rather than
# accumulating duplicate snapshots. The final fire leaves the most complete copy.
session_dir="${session_id:-unknown}"
DEST="$ARCHIVE_ROOT/$session_dir"
mkdir -p "$DEST" 2>/dev/null || { log "ERROR: cannot create $DEST"; exit 0; }

# Copy the transcript if we have a readable path.
transcript_copied=false
transcript_lines=0
transcript_bytes=0
if [ -n "$transcript" ] && [ -f "$transcript" ]; then
  if cp "$transcript" "$DEST/transcript.jsonl" 2>/dev/null; then
    transcript_copied=true
    transcript_lines="$(wc -l <"$DEST/transcript.jsonl" 2>/dev/null | tr -d ' ')"
    transcript_bytes="$(wc -c <"$DEST/transcript.jsonl" 2>/dev/null | tr -d ' ')"
  else
    log "ERROR: failed to copy transcript from '$transcript'"
  fi
else
  log "WARN: transcript_path missing or not a file: '$transcript'"
fi

# Write metadata. Prefer jq so we get valid JSON with the raw payload embedded.
if command -v jq >/dev/null 2>&1; then
  printf '%s' "$INPUT" | jq \
    --arg archived_at "$archived_at" \
    --arg dest "$DEST" \
    --argjson copied "$transcript_copied" \
    --argjson lines "${transcript_lines:-0}" \
    --argjson bytes "${transcript_bytes:-0}" \
    '{
       archived_at: $archived_at,
       archive_dir: $dest,
       transcript_copied: $copied,
       transcript_lines: $lines,
       transcript_bytes: $bytes,
       hook_payload: .
     }' >"$DEST/metadata.json" 2>/dev/null \
    || log "ERROR: jq failed to write metadata.json"
else
  cat >"$DEST/metadata.json" <<EOF
{
  "archived_at": "$archived_at",
  "archive_dir": "$DEST",
  "transcript_copied": $transcript_copied,
  "transcript_lines": ${transcript_lines:-0},
  "transcript_bytes": ${transcript_bytes:-0},
  "session_id": "$session_id",
  "hook_event_name": "$event",
  "reason": "$reason",
  "cwd": "$cwd",
  "permission_mode": "$perm_mode"
}
EOF
fi

log "OK: archived session $short_id (event=$event reason=$reason copied=$transcript_copied lines=$transcript_lines) -> $DEST"
exit 0
