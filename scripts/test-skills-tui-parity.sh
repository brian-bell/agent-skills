#!/bin/bash
set -euo pipefail

# Cross-implementation parity harness: drives the bash implementation
# (scripts/skills-tui.sh, the ground-truth spec) and the Go port
# (tools/skills-tui) through identical scenarios on identical fresh fixture
# trees, then diffs full filesystem snapshots and stdout/stderr transcripts.
#
# Determinism: dir-creation modes depend on umask in both implementations
# (bash mkdir -p, Go os.MkdirAll with 0o777), so pin one umask for the run.
umask 022

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BASH_TUI="$ROOT/scripts/skills-tui.sh"
GO_BIN="$ROOT/tools/skills-tui/bin/skills-tui"

# The harness controls these; never inherit them from the caller.
unset SKILL_SYMLINKS_DIR SKILL_INSTALL_TARGETS

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

echo "building Go binary..."
(cd "$ROOT/tools/skills-tui" && go build -o bin/skills-tui .) \
  || fail "go build failed"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

# Portable permission-bits reader (macOS stat first, GNU stat fallback).
pm() {
  stat -f '%Lp' "$1" 2>/dev/null || stat -c '%a' "$1"
}

# Build one fixture: repo/ (skills with varied permission bits incl. an
# executable script, a third-party skill, a claude-only team, a hybrid team),
# home/, and elsewhere/ (foreign symlink targets). The bash script infers the
# repo from its own location, so a copy of it is planted at repo/scripts/.
make_fixture() {
  local fx="$1" repo="$1/repo"

  mkdir -p "$fx/home" "$fx/elsewhere"
  mkdir -p "$repo/scripts" \
    "$repo/skills/alpha/scripts" "$repo/skills/beta" \
    "$repo/third-party/gamma" \
    "$repo/agent-teams/demo-team" \
    "$repo/agent-teams/hybrid-team/agents"

  echo "agents context" > "$repo/AGENTS.md"
  cp "$BASH_TUI" "$repo/scripts/skills-tui.sh"

  echo "alpha skill v1" > "$repo/skills/alpha/SKILL.md"
  printf '#!/bin/sh\necho run\n' > "$repo/skills/alpha/scripts/run.sh"
  echo "secret" > "$repo/skills/alpha/private.txt"
  chmod 644 "$repo/skills/alpha/SKILL.md"
  chmod 755 "$repo/skills/alpha/scripts/run.sh"
  chmod 600 "$repo/skills/alpha/private.txt"
  chmod 750 "$repo/skills/alpha/scripts"

  echo "beta skill" > "$repo/skills/beta/SKILL.md"

  echo "gamma skill" > "$repo/third-party/gamma/SKILL.md"
  chmod 644 "$repo/third-party/gamma/SKILL.md"
  echo "attribution" > "$repo/third-party/ATTRIBUTION.md" # plain file: skipped

  echo "demo manifest" > "$repo/agent-teams/demo-team/SKILL.md"
  echo "demo readme" > "$repo/agent-teams/demo-team/README.md"
  echo "demo lead" > "$repo/agent-teams/demo-team/lead.md"

  echo "hybrid manifest" > "$repo/agent-teams/hybrid-team/SKILL.md"
  echo "hybrid lead" > "$repo/agent-teams/hybrid-team/lead.md"
  echo "interface:" > "$repo/agent-teams/hybrid-team/agents/openai.yaml"
}

# Capture the full state of a tree: every path from find, its type,
# permission bits, symlink readlink target, and file content hash — sorted.
# The stage dir (~/.skill-symlinks) lives inside the fixture $HOME, so one
# root covers both.
snapshot() {
  local root="$1" p rel
  find "$root" -print | LC_ALL=C sort | while IFS= read -r p; do
    rel="${p#"$root"}"
    rel="${rel:-.}"
    if [ -L "$p" ]; then
      printf 'link %s -> %s\n' "$rel" "$(readlink "$p")"
    elif [ -d "$p" ]; then
      printf 'dir %s %s\n' "$rel" "$(pm "$p")"
    elif [ -f "$p" ]; then
      printf 'file %s %s %s\n' "$rel" "$(pm "$p")" "$(shasum -a 256 < "$p" | awk '{print $1}')"
    else
      printf 'other %s\n' "$rel"
    fi
  done
}

# Normalize fixture-specific noise so bash and Go outputs are comparable:
#  - the per-implementation mktemp paths (home first: it is inside the fixture)
#  - staged-backup timestamp directory names (YYYYmmddHHMMSS[-N] -> TS[-N]);
#    the two implementations run at different moments, and comparing backup
#    trees aside from the timestamp dir name is the scenario-c contract.
# Nothing else is normalized: any other difference is a real divergence.
normalize() {
  sed -e "s|$HOMEDIR|HOME|g" -e "s|$FIXTURE|FIXTURE|g" \
    | sed -E \
        -e 's,/([0-9]{14})(-[0-9]+)?/,/TS\2/,g' \
        -e 's,/([0-9]{14})(-[0-9]+)? ,/TS\2 ,g' \
        -e 's,/([0-9]{14})(-[0-9]+)?$,/TS\2,'
}

# Run the implementation under test against the current fixture, appending
# stdout/stderr to the scenario transcripts plus an exit-code marker.
tui() {
  local rc=0
  if [ "$IMPL" = bash ]; then
    HOME="$HOMEDIR" bash "$REPO/scripts/skills-tui.sh" "$@" \
      >> "$OUT" 2>> "$ERR" || rc=$?
  else
    HOME="$HOMEDIR" "$GO_BIN" --repo "$REPO" "$@" \
      >> "$OUT" 2>> "$ERR" || rc=$?
  fi
  echo "-- exit $rc" >> "$OUT"
}

run_scenario() {
  local name="$1" fn="scenario_$1" impl

  for impl in bash go; do
    FIXTURE="$(mktemp -d "$WORK/fx.XXXXXX")"
    make_fixture "$FIXTURE"
    REPO="$FIXTURE/repo"
    HOMEDIR="$FIXTURE/home"
    ELSEWHERE="$FIXTURE/elsewhere"
    IMPL="$impl"
    OUT="$WORK/$name.$impl.out.raw"
    ERR="$WORK/$name.$impl.err.raw"
    : > "$OUT"
    : > "$ERR"

    "$fn"

    snapshot "$HOMEDIR" | normalize > "$WORK/$name.$impl.snap"
    normalize < "$OUT" > "$WORK/$name.$impl.out"
    normalize < "$ERR" > "$WORK/$name.$impl.err"
    rm -rf "$FIXTURE"
  done

  diff -u "$WORK/$name.bash.snap" "$WORK/$name.go.snap" \
    || fail "$name: filesystem snapshots diverge (bash vs go)"
  diff -u "$WORK/$name.bash.out" "$WORK/$name.go.out" \
    || fail "$name: stdout transcripts diverge (bash vs go)"
  diff -u "$WORK/$name.bash.err" "$WORK/$name.go.err" \
    || fail "$name: stderr transcripts diverge (bash vs go)"
  echo "ok $name"
}

# --- scenarios --------------------------------------------------------------

# a. --all from clean.
scenario_all_clean() {
  tui --all
}

# b. --all twice: the second run must be a no-op ("nothing to do").
scenario_all_twice() {
  tui --all
  tui --all
}

# c. Upgrade path: content change, mode-only change, and a team-file change,
# each of which must refresh the staged copy and back up the previous one.
scenario_upgrade() {
  tui --all
  echo "alpha skill v2" > "$REPO/skills/alpha/SKILL.md"
  chmod 755 "$REPO/third-party/gamma/SKILL.md"
  echo "demo lead v2" > "$REPO/agent-teams/demo-team/lead.md"
  tui --all
}

# d. Roundtrip: --none after --all must remove owned links but leave the
# shared skills roots in place.
scenario_roundtrip() {
  tui --all
  tui --none
  local d
  for d in .agents .claude .cursor; do
    [ -d "$HOMEDIR/$d/skills" ] \
      || fail "roundtrip($IMPL): shared root $d/skills did not survive --none"
  done
}

# e. --force with a stale real dir at one target: the only path allowed to
# destroy non-repo data.
scenario_force_stale() {
  mkdir -p "$HOMEDIR/.claude/skills/alpha"
  echo "stale local edit" > "$HOMEDIR/.claude/skills/alpha/SKILL.md"
  tui --force
}

# f. Cursor-only targets: portable skills link into ~/.cursor only and teams
# are skipped entirely (no ~/.claude at all).
scenario_cursor_targets() {
  SKILL_INSTALL_TARGETS=cursor tui --all
  [ ! -e "$HOMEDIR/.claude" ] \
    || fail "cursor_targets($IMPL): claude root was created"
}

# g. A foreign symlink at one target under plain --all. Bash (the spec) counts
# the foreign link as upgradeable, so the upgrade path relinks it under
# force=true — non-destructively: the data it pointed at must survive.
scenario_foreign() {
  mkdir -p "$ELSEWHERE/other"
  echo "keep me" > "$ELSEWHERE/other/data.txt"
  mkdir -p "$HOMEDIR/.claude/skills"
  ln -s "$ELSEWHERE/other" "$HOMEDIR/.claude/skills/alpha"
  tui --all
  [ -f "$ELSEWHERE/other/data.txt" ] \
    || fail "foreign($IMPL): relinking the foreign symlink destroyed its data"
}

# --- run --------------------------------------------------------------------

run_scenario all_clean
run_scenario all_twice
run_scenario upgrade
run_scenario roundtrip
run_scenario force_stale
run_scenario cursor_targets
run_scenario foreign

echo "PASS: skills-tui parity"
