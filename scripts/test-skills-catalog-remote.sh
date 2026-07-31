#!/bin/bash
set -euo pipefail

SKILLS_CLI_VERSION="${SKILLS_CLI_VERSION:-1.5.21}"
CATALOG_SOURCE="${CATALOG_SOURCE:-https://github.com/brian-bell/agent-skills/tree/main/catalog}"
EXPECTED_SKILLS=(
  autofix
  autoreview
  batch-grill-me
  chrome-reading-list
  docs
  feature-review
  go-review
  grill-me
  improve-codebase-architecture
  last30days
  prd-to-issues
  prd-to-plan
  product-manager
  review-loop
  ship
  slice-issues
  tdd
  tdd-with-review
  teach
  wizard
  write-a-prd
)

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

test_root="$(mktemp -d)"
trap 'chmod -R u+w "$test_root" 2>/dev/null || true; rm -rf "$test_root"' EXIT
mkdir -p \
  "$test_root/home" \
  "$test_root/npm-cache" \
  "$test_root/xdg-cache" \
  "$test_root/xdg-config" \
  "$test_root/xdg-data" \
  "$test_root/work"

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

output="$(run_skills add "$CATALOG_SOURCE" --list)"
grep -q 'Found 21 skills' <<<"$output" \
  || fail "$CATALOG_SOURCE did not expose exactly 21 skills"
for skill in "${EXPECTED_SKILLS[@]}"; do
  grep -q "$skill" <<<"$output" || fail "$CATALOG_SOURCE did not list $skill"
done

run_skills add "$CATALOG_SOURCE" \
  --skill '*' \
  --agent codex \
  --global \
  --yes \
  --copy >/dev/null

install_root="$test_root/home/.agents/skills"
for skill in "${EXPECTED_SKILLS[@]}"; do
  [ -f "$install_root/$skill/SKILL.md" ] || fail "Codex missing installed $skill"
done
[ ! -e "$test_root/home/.claude/skills" ] \
  || fail "Codex-only remote install touched the Claude skill root"
[ -f "$install_root/feature-review/roles/product-reviewer.md" ] \
  || fail "remote feature-review package is missing its shared role"
[ -f "$install_root/feature-review/agents/openai.yaml" ] \
  || fail "remote feature-review package is missing Codex UI metadata"
[ ! -e "$install_root/feature-review/runtimes" ] \
  || fail "remote feature-review package contains runtime routing"
[ ! -e "$install_root/feature-review/findings-schema.md" ] \
  || fail "remote feature-review package contains Claude-only schema"

echo "PASS: skills@$SKILLS_CLI_VERSION installed the canonical Codex catalog"
