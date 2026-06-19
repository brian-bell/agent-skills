#!/usr/bin/env bash
#
# backfill.sh — import existing Claude Code transcripts into the session store.
#
# The save-session SessionEnd hook only archives sessions that end after it is
# installed. This backfills sessions that already exist in Claude Code's default
# transcript location into the same store the hook writes to, with synthesized
# metadata (cwd, git branch, timestamps, version) read from each transcript.
#
# Source:  ~/.claude/projects/<encoded-cwd>/<session-id>.jsonl   (one per session)
# Dest:    ~/.agent-sessions/claude/<session-id>/
#              transcript.jsonl   (copy of the source)
#              metadata.json      (synthesized; "source": "backfill")
#
# Usage:
#   ./backfill.sh [--dry-run] [--update | --force] [--quiet]
#
#   (default)    Skip any session already present in the store. Idempotent.
#   --update     Overwrite when the source transcript has more lines than the
#                archived copy (keeps the most complete version).
#   --force      Overwrite every session unconditionally.
#   --dry-run    Report what would happen without writing anything.
#   --quiet      Only print the final summary.
#
# Env overrides (shared with the hook):
#   CLAUDE_SESSION_ARCHIVE_DIR   dest store   (default ~/.agent-sessions/claude)
#   CLAUDE_PROJECTS_DIR          source root  (default ~/.claude/projects)

set -uo pipefail

ARCHIVE_ROOT="${CLAUDE_SESSION_ARCHIVE_DIR:-$HOME/.agent-sessions/claude}"
PROJECTS_DIR="${CLAUDE_PROJECTS_DIR:-$HOME/.claude/projects}"

MODE=skip   # skip | update | force
DRY_RUN=false
QUIET=false
while [ "$#" -gt 0 ]; do
  case "$1" in
    --update) MODE=update ;;
    --force)  MODE=force ;;
    --dry-run) DRY_RUN=true ;;
    --quiet)  QUIET=true ;;
    -h|--help) sed -n '2,30p' "$0"; exit 0 ;;
    *) echo "Unknown option: $1" >&2; exit 1 ;;
  esac
  shift
done

command -v jq >/dev/null 2>&1 || { echo "ERROR: jq is required" >&2; exit 1; }

if [ ! -d "$PROJECTS_DIR" ]; then
  echo "No source directory at $PROJECTS_DIR — nothing to backfill." >&2
  exit 0
fi

say() { [ "$QUIET" = true ] || printf '%s\n' "$*"; }

scanned=0 backfilled=0 updated=0 skipped=0 errors=0
archived_at="$(date '+%Y-%m-%dT%H:%M:%S%z')"

# First .jsonl per session across all projects. Globbing is recursive one level.
shopt -s nullglob
for src in "$PROJECTS_DIR"/*/*.jsonl; do
  scanned=$((scanned + 1))
  sid="$(basename "$src" .jsonl)"
  dest="$ARCHIVE_ROOT/$sid"
  src_lines="$(wc -l <"$src" 2>/dev/null | tr -d ' ')"

  if [ -d "$dest" ]; then
    case "$MODE" in
      skip)
        say "skip   $sid (already in store)"
        skipped=$((skipped + 1)); continue ;;
      update)
        old_lines="$(wc -l <"$dest/transcript.jsonl" 2>/dev/null | tr -d ' ')"
        old_lines="${old_lines:-0}"
        if [ "${src_lines:-0}" -le "$old_lines" ]; then
          say "skip   $sid (store copy >= source: $old_lines >= ${src_lines:-0})"
          skipped=$((skipped + 1)); continue
        fi
        action=update ;;
      force) action=update ;;
    esac
  else
    action=create
  fi

  if [ "$DRY_RUN" = true ]; then
    say "would $action $sid (${src_lines:-0} lines) -> $dest"
    [ "$action" = update ] && updated=$((updated + 1)) || backfilled=$((backfilled + 1))
    continue
  fi

  if ! mkdir -p "$dest" 2>/dev/null; then
    say "ERROR  $sid (cannot create $dest)"; errors=$((errors + 1)); continue
  fi
  if ! cp "$src" "$dest/transcript.jsonl" 2>/dev/null; then
    say "ERROR  $sid (copy failed)"; errors=$((errors + 1)); continue
  fi

  # Pull metadata from the transcript itself (each line is a JSON object).
  cwd="$(jq -r '.cwd // empty'        "$src" 2>/dev/null | head -1)"
  git_branch="$(jq -r '.gitBranch // empty' "$src" 2>/dev/null | head -1)"
  version="$(jq -r '.version // empty' "$src" 2>/dev/null | head -1)"
  started_at="$(jq -r '.timestamp // empty' "$src" 2>/dev/null | head -1)"
  ended_at="$(jq -r '.timestamp // empty' "$src" 2>/dev/null | grep . | tail -1)"
  bytes="$(wc -c <"$dest/transcript.jsonl" 2>/dev/null | tr -d ' ')"

  jq -n \
    --arg archived_at "$archived_at" \
    --arg dest "$dest" \
    --arg sid "$sid" \
    --arg src "$src" \
    --arg cwd "$cwd" \
    --arg git_branch "$git_branch" \
    --arg version "$version" \
    --arg started_at "$started_at" \
    --arg ended_at "$ended_at" \
    --argjson lines "${src_lines:-0}" \
    --argjson bytes "${bytes:-0}" \
    '{
       source: "backfill",
       archived_at: $archived_at,
       archive_dir: $dest,
       transcript_copied: true,
       transcript_lines: $lines,
       transcript_bytes: $bytes,
       session: {
         session_id: $sid,
         source_transcript: $src,
         cwd: (if $cwd == "" then null else $cwd end),
         git_branch: (if $git_branch == "" then null else $git_branch end),
         version: (if $version == "" then null else $version end),
         started_at: (if $started_at == "" then null else $started_at end),
         ended_at: (if $ended_at == "" then null else $ended_at end)
       }
     }' >"$dest/metadata.json" 2>/dev/null \
    || { say "ERROR  $sid (metadata write failed)"; errors=$((errors + 1)); continue; }

  say "$action $sid (${src_lines:-0} lines) -> $dest"
  [ "$action" = update ] && updated=$((updated + 1)) || backfilled=$((backfilled + 1))
done
shopt -u nullglob

prefix=""; [ "$DRY_RUN" = true ] && prefix="[dry-run] "
say ""
echo "${prefix}Backfill complete: scanned=$scanned backfilled=$backfilled updated=$updated skipped=$skipped errors=$errors"
echo "${prefix}Store: $ARCHIVE_ROOT"
