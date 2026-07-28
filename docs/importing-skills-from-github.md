# Importing Skills from GitHub

The interactive installer can discover and import portable skills from a
GitHub repository. Importing adds selected source to this repository under
`third-party/`; it does not immediately install the new skills into Codex or
Claude Code.

GitHub import is available only in the interactive TUI. There is no import CLI
flag or environment-variable equivalent.

## Import workflow

1. Run `./install.sh`.
2. Press `i` from the main skill list.
3. Choose a saved repository or **Paste a new repository URL**, then press
   `enter` to clone and scan it.
4. Review the discovered candidates. Valid, non-conflicting skills start
   selected; invalid candidates remain visible but disabled with a reason.
5. Use `↑`/`↓` or `j`/`k` to move, `space` to toggle, `a` to select all valid
   candidates, `n` to select none, and `enter` to import the selected batch.
6. Back on the main list, review the newly imported rows and press `enter` to
   apply installation.

`esc` backs out of a screen and requests cancellation of an active scan or
import. Cancellation is best-effort: if publication completes before the
request takes effect, the import succeeds. `ctrl-c` exits the TUI.

A successful import:

- Copies each selected directory to `third-party/<name>/`.
- Adds a provenance row to `third-party/ATTRIBUTION.md`.
- Reloads the main list with the imported rows selected and moves the cursor
  to the first imported skill.
- Preserves the pending selections of existing rows.

Import does not refresh `~/.skill-symlinks/` or create runtime links. The
normal installer apply remains a separate, explicit step.

## Accepted repository URLs and authentication

The importer accepts only HTTPS GitHub repository URLs in this form:

```text
https://github.com/<owner>/<repository>
```

An optional `.git` suffix, one trailing slash, or both are accepted. Owner and
repository names are normalized to lowercase, and the suffix and trailing
slash are removed before the URL is saved.

The importer rejects:

- HTTP and SSH URLs
- Embedded credentials or explicit ports
- Escaped paths, queries, and fragments
- Branch and subpath URLs
- GitHub Enterprise hosts and other forges

Clone runs without a shell using `--depth 1` and `--no-tags`. It inherits
existing Git authentication while setting `GIT_TERMINAL_PROMPT=0` and
`GCM_INTERACTIVE=Never`. Public repositories work directly; private
repositories require credentials that already work non-interactively.

## Saved repository history

Repository history is stored at:

```text
<user-config-dir>/agent-skills/import-repositories.json
```

On macOS, `<user-config-dir>` is normally `~/Library/Application Support`.
The installer creates its dedicated `agent-skills` directory with mode `0700`
and keeps the history and lock files at `0600`.

A repository is added or refreshed only after a successful scan finds at
least one valid, non-conflicting skill. Records appear in most-recently-used
order; rescanning a saved URL moves it to the front without creating a
duplicate.

To forget a saved URL, place the cursor on it, press `d`, and confirm with `y`.
`N`, `enter`, or `esc` cancels. Deleting history does not delete imported
skills, edit attribution, remove staged copies, or change installed runtime
links.

## Candidate discovery and validation

Scanning reads repository content but never executes it. It walks the checkout
root and nested directories, including hidden compatibility roots, while
skipping `.git`.

Each candidate must:

- Have a real regular `SKILL.md` file.
- Include YAML frontmatter with non-empty `name` and `description` values.
- Use a safe install name that does not collide case-insensitively with another
  candidate, a first-party skill, or an existing third-party entry.

Once a valid skill root is accepted, scanning prunes that directory;
descendant `SKILL.md` files are not offered as separate candidates.

## Publication and rollback guarantees

Import holds a repository-wide import lock, then revalidates every selected
candidate and destination before publication. It stages the complete batch
under `third-party/`, excludes `.git`, preserves regular-file modes, and
rejects symlinks and special files.

Skill destinations are published with atomic no-overwrite operations. Any
validation, copy, publication, or attribution failure stops the transaction.
If skill directories were already published, the importer attempts to roll
them back before returning the error. Cleanup failures are reported alongside
the original failure and can prevent complete rollback.

Cancellation observed during checkout, scanning, or tree staging publishes no
skills. If publication wins the race with a cancellation request, the
completed import is returned as successful.

`third-party/ATTRIBUTION.md` is the expected mutable repository file. Its
updated content is staged with the batch and replaces the previous file last.
The import lock serializes importer transactions but not arbitrary editors, so
do not edit attribution concurrently with an import.

## Attribution and licensing

Each attribution row links to the scanned commit and candidate subpath:

```text
https://github.com/<owner>/<repository>/tree/<commit>/<candidate-subpath>
```

The commit root is used for a root-level skill. Automatic imports do not
verify licensing; new rows record the license as `Unknown (unverified)` until
it is reviewed manually.
