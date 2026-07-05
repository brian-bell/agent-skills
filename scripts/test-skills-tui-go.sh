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
go test ./...
