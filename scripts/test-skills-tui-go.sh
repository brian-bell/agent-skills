#!/bin/bash
set -euo pipefail

# Format, vet, build, and test the Go TUI installer at tools/skills-tui.
cd "$(dirname "$0")/../tools/skills-tui"

unformatted="$(gofmt -l .)"
if [[ -n "$unformatted" ]]; then
  echo "gofmt -l found unformatted files:" >&2
  echo "$unformatted" >&2
  exit 1
fi

go vet ./...
go build ./...
GOOS=windows GOARCH=amd64 go build ./...
go test ./...

# Supply-chain check: run govulncheck when it is available (best-effort so the
# suite still runs on machines without it). Install with
# `go install golang.org/x/vuln/cmd/govulncheck@latest`.
if command -v govulncheck >/dev/null 2>&1; then
  govulncheck ./...
else
  echo "govulncheck not installed; skipping vulnerability scan" >&2
fi
