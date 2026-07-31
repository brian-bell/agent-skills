#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SKILLS_CLI_VERSION="1.5.21"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

for command in codex node npx python3; do
  command -v "$command" >/dev/null || fail "missing required command: $command"
done
codex login status 2>&1 | grep -q '^Logged in' \
  || fail "Codex CLI must be logged in before the live catalog smoke"

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

(
  cd "$test_root/work"
  HOME="$test_root/home" \
    XDG_CACHE_HOME="$test_root/xdg-cache" \
    XDG_CONFIG_HOME="$test_root/xdg-config" \
    XDG_DATA_HOME="$test_root/xdg-data" \
    npm_config_cache="$test_root/npm-cache" \
    NPM_CONFIG_UPDATE_NOTIFIER=false \
    DISABLE_TELEMETRY=1 \
    npx --yes "skills@$SKILLS_CLI_VERSION" add "$catalog" \
      --skill feature-review \
      --agent codex \
      --global \
      --yes \
      --copy >/dev/null
)

installed="$test_root/home/.agents/skills/feature-review"
[ -f "$installed/SKILL.md" ] || fail "installed package is missing root SKILL.md"
[ -f "$installed/agents/openai.yaml" ] || fail "installed package is missing UI metadata"
[ -f "$installed/roles/product-reviewer.md" ] || fail "installed package is missing shared role"
[ ! -e "$installed/runtimes" ] || fail "installed package contains runtime routing"
[ ! -e "$installed/findings-schema.md" ] || fail "installed package contains Claude-only schema"
cmp "$catalog/skills/feature-review/SKILL.md" "$installed/SKILL.md" \
  || fail "installed root SKILL.md differs from the generated Codex entry point"

runtime_output="$($SCRIPT_DIR/check-skills-catalog-codex.mjs \
  "$test_root/work" \
  "$test_root/home/.agents/skills" \
  "$installed/SKILL.md")"
grep -q '^CATALOG_CODEX_DISCOVERY_OK|feature-review|Feature Review|' \
  <<<"$runtime_output" || fail "Codex did not expose feature-review UI metadata"
grep -qx 'CATALOG_CODEX_BODY_OK|roles/product-reviewer.md|spawn_agent' \
  <<<"$runtime_output" || fail "Codex did not load the root Codex skill body"

echo "$runtime_output"
echo "PASS: live Codex catalog skill discovery and activation"
