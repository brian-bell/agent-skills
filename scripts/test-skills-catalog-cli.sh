#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SKILLS_CLI_VERSION="1.5.21"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

assert_tree_equal() {
  python3 - "$1" "$2" <<'PY'
from pathlib import Path
import stat
import sys


def manifest(root: Path):
    entries = {}
    for path in root.rglob("*"):
        relative = path.relative_to(root).as_posix()
        if path.is_symlink():
            entries[relative] = ("symlink", path.readlink().as_posix(), None)
        elif path.is_dir():
            entries[relative] = ("directory", None, None)
        else:
            entries[relative] = (
                "file",
                path.read_bytes(),
                stat.S_IMODE(path.stat().st_mode),
            )
    return entries


source = Path(sys.argv[1])
installed = Path(sys.argv[2])
if manifest(source) != manifest(installed):
    raise SystemExit(f"installed tree differs from generated package: {installed}")
PY
}

mode="${1:-all}"
case "$mode" in
  all | discovery | install) ;;
  *) fail "unknown mode: $mode" ;;
esac

test_root="$(mktemp -d)"
trap 'chmod -R u+w "$test_root" 2>/dev/null || true; rm -rf "$test_root"' EXIT
mkdir -p \
  "$test_root/home" \
  "$test_root/npm-cache" \
  "$test_root/xdg-cache" \
  "$test_root/xdg-config" \
  "$test_root/xdg-data" \
  "$test_root/work"

catalog="$test_root/catalog"
python3 "$ROOT/scripts/generate-skills-catalog.py" --output "$catalog"

run_skills() {
  (
    cd "$test_root/work"
    HOME="$test_root/home" \
      XDG_CACHE_HOME="$test_root/xdg-cache" \
      XDG_CONFIG_HOME="$test_root/xdg-config" \
      XDG_DATA_HOME="$test_root/xdg-data" \
      npm_config_cache="$test_root/npm-cache" \
      NPM_CONFIG_UPDATE_NOTIFIER=false \
      DISABLE_TELEMETRY=1 \
      npx --yes "skills@$SKILLS_CLI_VERSION" "$@"
  )
}

if [ "$mode" = all ] || [ "$mode" = discovery ]; then
  output="$(run_skills add "$catalog" --list)"
  grep -q 'Found 2 skills' <<<"$output" \
    || fail "pinned CLI did not discover exactly two skills"
  grep -q 'feature-review' <<<"$output" \
    || fail "pinned CLI did not list feature-review"
  grep -q 'last30days' <<<"$output" \
    || fail "pinned CLI did not list last30days"
  if grep -q 'skill-parity-audit' <<<"$output"; then
    fail "project-scoped skill leaked into the catalog"
  fi
  echo "PASS: skills catalog CLI discovery"
fi

if [ "$mode" = all ] || [ "$mode" = install ]; then
  run_skills add "$catalog" \
    --skill feature-review \
    --skill last30days \
    --agent codex \
    --agent claude-code \
    --global \
    --yes \
    --copy >/dev/null

  for target in .agents .claude; do
    install_root="$test_root/home/$target/skills"
    feature_review="$install_root/feature-review"
    last30days="$install_root/last30days"

    [ -d "$feature_review" ] || fail "$target missing installed feature-review"
    [ ! -L "$feature_review" ] || fail "$target feature-review must be a copy"
    assert_tree_equal "$catalog/skills/feature-review" "$feature_review"
    [ -f "$feature_review/SKILL.md" ] || fail "$target missing runtime router"
    [ -f "$feature_review/agents/openai.yaml" ] \
      || fail "$target missing feature-review Codex metadata"
    [ -f "$feature_review/runtimes/codex/roles/product-reviewer.md" ] \
      || fail "$target missing Codex shared role"
    [ -f "$feature_review/runtimes/claude/roles/product-reviewer.md" ] \
      || fail "$target missing Claude shared role"
    [ ! -e "$feature_review/runtimes/codex/findings-schema.md" ] \
      || fail "$target Codex assembly contains Claude-only schema"
    [ -f "$feature_review/runtimes/claude/findings-schema.md" ] \
      || fail "$target Claude assembly missing schema"

    [ -d "$last30days" ] || fail "$target missing installed last30days"
    [ ! -L "$last30days" ] || fail "$target last30days must be a copy"
    assert_tree_equal "$catalog/skills/last30days" "$last30days"
    [ -f "$last30days/agents/openai.yaml" ] \
      || fail "$target missing last30days Codex metadata"
    [ -f "$last30days/references/save-html-brief.md" ] \
      || fail "$target missing last30days reference"
    [ -f "$last30days/ATTRIBUTION.md" ] \
      || fail "$target missing last30days attribution"
    [ -x "$last30days/scripts/build-skill.sh" ] \
      || fail "$target lost last30days executable mode"
    cmp "$ROOT/third-party/last30days/scripts/build-skill.sh" \
      "$last30days/scripts/build-skill.sh" \
      || fail "$target changed last30days executable content"
  done
  echo "PASS: skills catalog CLI isolated install"
fi
