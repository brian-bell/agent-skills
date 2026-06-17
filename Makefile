SHELL := /bin/sh

.PHONY: help check test install clean

help:
	@printf '%s\n' \
		'Targets:' \
		'  make install    Install catalog skills into local agent roots' \
		'  make check      Smoke-test installation with a temporary HOME' \
		'  make test       Alias for check' \
		'  make clean      Remove repo-local generated importer roots'

install:
	./install.sh

check:
	@tmp_home="$$(mktemp -d)"; \
	trap 'rm -rf "$$tmp_home"' EXIT; \
	HOME="$$tmp_home" ./install.sh >/dev/null; \
	test -L "$$tmp_home/.agents/skills/tdd"; \
	test -L "$$tmp_home/.claude/skills/tdd"; \
	test -L "$$tmp_home/.claude/skills/go-review"; \
	test -L "$$tmp_home/.claude/agents/go-review-team/review-lead.md"

test: check

clean:
	rm -rf "$(CURDIR)/.skill-importer"
