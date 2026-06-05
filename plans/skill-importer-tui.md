# Plan: Skill Importer TUI

> Source PRD: GitHub issue #5, "Build skill importer TUI"

## Progress tracker

- [x] Phase 1: Merged Skill Discovery
- [x] Phase 2: JSON Inventory Command
- [x] Phase 3: Markdown Import Into Managed Storage
- [x] Phase 4: Local Path Import With Supporting Files
- [ ] Phase 5: URL Import Behind Injectable Fetching
- [ ] Phase 6: Repository Import With Skill Selection
- [ ] Phase 7: Enable And Disable Per Agent
- [ ] Phase 8: Promote Imported Skills Safely
- [ ] Phase 9: Delete Unpromoted Imports
- [ ] Phase 10: Keyboard-First TUI Over Core State

## Architectural decisions

Durable decisions that apply across all phases:

- **Product surface**: Build a Rust terminal UI named `skill-importer`. The TUI is the primary user experience from the first release.
- **Automation surface**: Provide non-interactive commands for listing, importing, enabling, disabling, promoting, and deleting imports so behavior can be tested and scripted without driving terminal rendering.
- **Core boundary**: Keep discovery, import parsing, storage, symlink management, promotion, deletion, and app state outside the rendering layer. The TUI consumes state and dispatches actions.
- **Managed roots**: Treat canonical local skills, imported skills, Claude Code skills, and Codex skills as configurable roots. Defaults should match the current repo convention: canonical portable skills in the local skills repository, Claude Code skills in `~/.claude/skills`, and Codex skills in `~/.agents/skills`.
- **Skill inventory model**: Discovery produces one merged inventory of skills, including canonical skills, imported skills, Claude Code entries, Codex entries, external symlinks, real directories, missing roots, and broken symlinks.
- **Skill validation**: Imported portable skills require minimal frontmatter with `name` and `description`.
- **Import storage**: Store unpromoted imports in a dedicated imports area. Each import has manifest metadata for source type, source location, import time, content hash, and promotion status.
- **Source boundaries**: Keep network fetching and repository fetching behind injectable boundaries so import behavior can be tested deterministically.
- **Enablement model**: Enablement is per-agent. A skill can be enabled for Claude Code, Codex, both, or neither by creating managed symlinks from the selected agent root to either an imported skill or a promoted canonical skill.
- **Safety model**: Only managed symlinks may be removed or rewritten. Real directories, external symlinks, and canonical skill collisions must be protected.
- **Promotion model**: Promotion copies an imported skill into the canonical local skill collection, preserves supporting files, and rewrites enabled managed symlinks to the promoted canonical skill. Promotion does not edit documentation or installer scripts.
- **TDD workflow**: Build each phase as red-green-refactor. Write one behavior test through a public interface, implement the minimum code needed to pass, then refactor while green.

---

## Phase 1: Merged Skill Discovery

**User stories**: 1, 2, 3, 4, 5, 8, 9, 10, 39, 41, 43

### What to build

Create the first public discovery path that can inspect configurable canonical, import, Claude Code, and Codex roots and return a merged inventory. This first tracer bullet should use temporary roots and prove that installed skills are visible across both agents without touching real user directories.

### TDD checklist

- [x] RED: Add one behavior test that creates temporary roots with at least one canonical skill enabled for both agents and expects one merged inventory entry.
- [x] GREEN: Implement the smallest public discovery interface that returns that merged entry.
- [x] REFACTOR: Name the public inventory concepts clearly enough to support later status and source classifications.

### Acceptance criteria

- [x] Discovery works with fully configurable roots.
- [x] Missing roots do not crash discovery.
- [x] A canonical skill symlinked into both agent roots appears once in the merged inventory.
- [x] The inventory reports whether the skill is enabled for Claude Code, Codex, both, or neither.
- [x] Tests verify behavior through the public discovery interface, not private helpers.

---

## Phase 2: JSON Inventory Command

**User stories**: 1, 2, 3, 4, 5, 6, 7, 8, 9, 39, 40, 41, 43

### What to build

Add a non-interactive listing command that exposes the merged discovery state as JSON. This creates the first CLI-visible vertical slice and gives future tests and scripts a stable way to inspect the same inventory the TUI will use.

### TDD checklist

- [x] RED: Add one command-level behavior test that lists temporary roots as JSON and asserts observable skill status.
- [x] GREEN: Implement the minimal command path that calls the public discovery interface and prints JSON.
- [x] REFACTOR: Keep output serialization separate from discovery decisions.

### Acceptance criteria

- [x] The list command outputs valid JSON.
- [x] JSON includes skill name, description when available, source classification, and per-agent enablement.
- [x] Canonical, imported, external, real directory, and broken symlink statuses can be represented.
- [x] Missing skill roots are represented safely or ignored without failure, according to the inventory model.
- [x] Tests assert the command output through the public command surface.

---

## Phase 3: Markdown Import Into Managed Storage

**User stories**: 11, 15, 16, 17, 18, 19, 20, 31, 39, 41, 45

### What to build

Support importing pasted skill Markdown through the core import interface and a non-interactive command. A successful import validates frontmatter, stores the skill in the imports area, records metadata, reports the filesystem actions taken, and makes the imported skill visible in the inventory without enabling it.

### TDD checklist

- [x] RED: Add one behavior test that imports valid Markdown and expects stored contents, manifest metadata, and an inventory entry.
- [x] GREEN: Implement minimal Markdown validation, content hashing, storage, and action reporting.
- [x] REFACTOR: Shape validation errors and import results for reuse by the TUI status area.

### Acceptance criteria

- [x] Valid pasted Markdown with `name` and `description` is stored as an unpromoted import.
- [x] Import metadata records source type, source location when available, import time, content hash, and promotion status.
- [x] Imported skills refuse name collisions with existing imports and canonical skills by default.
- [x] Invalid frontmatter fails before storage or enablement.
- [x] Validation errors name the missing or invalid field.
- [x] The action result clearly reports success or failure.

---

## Phase 4: Local Path Import With Supporting Files

**User stories**: 14, 15, 16, 17, 18, 19, 20, 22, 31, 39, 41, 45

### What to build

Add local path imports for skill directories or skill Markdown files. The slice should preserve supporting files that belong to the selected skill so scripts, references, templates, and assets remain available before promotion.

### TDD checklist

- [x] RED: Add one behavior test that imports a local skill directory with a supporting file and expects the import copy to preserve it.
- [x] GREEN: Implement minimal local path import for one valid skill source.
- [x] REFACTOR: Share validation, hashing, collision handling, and result reporting with Markdown imports.

### Acceptance criteria

- [x] A valid local skill directory imports into the managed imports area.
- [x] A valid local skill Markdown file imports into the managed imports area.
- [x] Supporting files inside the selected skill are preserved.
- [x] Local path metadata records the original source location.
- [x] Invalid or ambiguous paths return user-facing errors without partial storage.
- [x] Collision behavior matches Markdown import behavior.

---

## Phase 5: URL Import Behind Injectable Fetching

**User stories**: 12, 15, 16, 17, 18, 19, 20, 31, 39, 41, 42, 45

### What to build

Support direct skill file URL imports while keeping fetching isolated from parsing and storage. Tests should provide deterministic fetched content through an injected boundary rather than relying on live network access.

### TDD checklist

- [ ] RED: Add one behavior test that imports from a fake URL fetcher and expects stored import metadata and content.
- [ ] GREEN: Implement the minimal direct-file URL import path.
- [ ] REFACTOR: Keep source acquisition, validation, and storage as separate concepts behind a simple public interface.

### Acceptance criteria

- [ ] URL import accepts direct skill Markdown content from an injectable fetcher.
- [ ] URL import uses the same validation and collision behavior as other import sources.
- [ ] Source metadata records the URL.
- [ ] Fetch failures are reported in user-facing language.
- [ ] Tests do not depend on live network access.

---

## Phase 6: Repository Import With Skill Selection

**User stories**: 13, 15, 16, 17, 18, 19, 20, 21, 22, 31, 39, 41, 42, 45

### What to build

Add repository import through an injectable repository provider. Repositories with one valid skill can import directly. Repositories with multiple valid skills produce a selection state that the TUI can present interactively before importing the chosen skill.

### TDD checklist

- [ ] RED: Add one behavior test for a repository provider that returns multiple valid skills and expects a selection result instead of an arbitrary import.
- [ ] GREEN: Implement minimal repository scanning and selection-result behavior.
- [ ] REFACTOR: Unify selected repository skill import with local path import so supporting files are preserved consistently.

### Acceptance criteria

- [ ] Repository import detects zero, one, and many valid skills.
- [ ] Repositories with zero valid skills return a clear error.
- [ ] Repositories with one valid skill can import it directly.
- [ ] Repositories with multiple valid skills return an interactive selection state.
- [ ] Importing the selected repository skill preserves supporting files.
- [ ] Repository tests use local fixtures or injectable providers, not live remote repositories.

---

## Phase 7: Enable And Disable Per Agent

**User stories**: 25, 26, 27, 28, 29, 30, 31, 39, 41, 44, 45

### What to build

Support enabling imported or canonical skills for Claude Code, Codex, or both by creating managed symlinks. Support disabling by removing only managed symlinks. This is the highest-risk filesystem mutation slice, so safety behavior should be driven by focused tests.

### TDD checklist

- [ ] RED: Add one behavior test that enables an imported skill for one agent in a temporary root and expects a managed symlink plus action report.
- [ ] GREEN: Implement the minimal enable path for one target agent.
- [ ] REFACTOR: Extend the same public operation to both agents and prepare safe disable behavior while green.

### Acceptance criteria

- [ ] Enabling creates missing managed agent roots when needed.
- [ ] A selected skill can be enabled for Claude Code only.
- [ ] A selected skill can be enabled for Codex only.
- [ ] A selected skill can be enabled for both agents.
- [ ] Disabling removes only managed symlinks.
- [ ] Disabling refuses to delete real directories.
- [ ] Replacement refuses to overwrite external symlinks or unsafe existing entries.
- [ ] Every filesystem action is reported.

---

## Phase 8: Promote Imported Skills Safely

**User stories**: 32, 33, 34, 35, 36, 39, 41, 45

### What to build

Support promoting an imported skill into the canonical local skill collection. Promotion preserves supporting files, refuses canonical name collisions, updates the import manifest, and relinks any enabled managed symlinks to the canonical skill.

### TDD checklist

- [ ] RED: Add one behavior test that imports, enables, promotes, and expects the canonical copy plus relinked managed symlink.
- [ ] GREEN: Implement the minimal promotion path for one imported skill.
- [ ] REFACTOR: Clarify promotion result reporting and collision handling while all filesystem tests stay green.

### Acceptance criteria

- [ ] Promotion copies the imported skill into the canonical local skill collection.
- [ ] Promotion preserves supporting files.
- [ ] Promotion refuses to overwrite an existing canonical skill.
- [ ] Promotion rewrites enabled managed symlinks to point at the promoted canonical skill.
- [ ] Promotion leaves documentation and installer scripts untouched.
- [ ] Promotion reports all filesystem actions and failure reasons.

---

## Phase 9: Delete Unpromoted Imports

**User stories**: 37, 38, 39, 41, 45

### What to build

Allow cleanup of rejected unpromoted imports while protecting canonical and promoted skills. Deletion should be unavailable for canonical skills and should refuse any operation that would affect enabled managed symlinks without an explicit safe flow.

### TDD checklist

- [ ] RED: Add one behavior test that deletes an unpromoted import and expects the import storage to be removed and the inventory updated.
- [ ] GREEN: Implement the minimal unpromoted import deletion path.
- [ ] REFACTOR: Reuse safety and action-reporting language from enable, disable, and promotion.

### Acceptance criteria

- [ ] Unpromoted imports can be deleted from the managed imports area.
- [ ] Canonical skills cannot be deleted through the import cleanup operation.
- [ ] Promoted imports cannot be deleted as unpromoted experiments.
- [ ] Deletion reports success or a clear failure reason.
- [ ] Inventory reflects deleted imports after the operation.

---

## Phase 10: Keyboard-First TUI Over Core State

**User stories**: 6, 7, 21, 23, 24, 25, 31, 39, 45

### What to build

Build the terminal UI on top of the already-tested core behavior. The TUI should show the merged inventory, filtering, selected skill details, active enablement target, action hints, import prompts, multi-skill repository selection, and visible success or failure results after each action.

### TDD checklist

- [ ] RED: Add one reducer-style state transition test for filtering, selection, target switching, or an action result.
- [ ] GREEN: Implement the minimal app state transition and render path needed for the tested behavior.
- [ ] REFACTOR: Keep rendering thin and leave filesystem behavior in the core operations already covered by tests.

### Acceptance criteria

- [ ] The main TUI screen shows the merged skill list.
- [ ] Filtering helps find a specific skill quickly.
- [ ] The selected skill view shows name, description, source status, and enablement state.
- [ ] The active enablement target can switch between Claude Code and Codex.
- [ ] Repository imports with multiple valid skills enter an interactive selection flow.
- [ ] Keyboard-first action hints are visible.
- [ ] Each action leaves a visible success or failure result.
- [ ] A terminal-backend smoke test verifies the main screen renders key sections without panics.
