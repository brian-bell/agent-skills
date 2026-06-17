---
name: chrome-reading-list
description: Extract Chrome's Reading List to CSV or JSON. Parses the Sync Data LevelDB directly (Chrome has no export UI). Use when the user asks to export, dump, back up, or list their Chrome reading list items.
user_invocable: true
---

# Chrome Reading List Exporter

Chrome doesn't expose the Reading List via Bookmarks export or any UI. Entries live in the Sync Data LevelDB with snappy-compressed SST blocks, prefix-compressed keys, and protobuf values. `extract.py` in this directory handles all of that.

## Steps

1. Parse the user's message for optional args:
   - Profile name (default `Default`)
   - Output path (default `~/Desktop/chrome-reading-list.csv`)
   - Format: `csv` | `json` | `both` (default `csv`)
   - Filters: `--unread-only` or `--read-only` (default: all)
2. Run the extractor:
   ```bash
   python3 ~/.claude/skills/chrome-reading-list/extract.py [--profile ...] [--out ...] [--format ...] [--unread-only|--read-only]
   ```
3. If it exits `1`, `python-snappy` is missing. Install and retry:
   ```bash
   pip3 install --user python-snappy && python3 ~/.claude/skills/chrome-reading-list/extract.py ...
   ```
4. Other exit codes: `2` = LevelDB directory not found (wrong profile name?), `3` = parse failure. Surface the stderr message to the user.
5. Chrome can be open during the read — reads are non-destructive. Closing Chrome first gives a fully consistent snapshot but it's rarely needed.
6. Report the output path and the entry count/status breakdown from the script's stdout. Offer to `open` the file for them.
