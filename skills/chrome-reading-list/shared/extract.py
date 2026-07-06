#!/usr/bin/env python3
"""Extract Chrome Reading List from the local Sync Data LevelDB.

Chrome has no built-in export for its Reading List. Entries are stored in
`~/Library/Application Support/Google/Chrome/<Profile>/Sync Data/LevelDB/`,
where SST data blocks are snappy-compressed, keys use prefix compression,
and values are `sync_pb::EntitySpecifics` protobufs wrapping a nested
`ReadingListSpecifics` message. This script walks every *.ldb and the
current *.log, decompresses, reconstructs keys, decodes the protos, and
writes CSV and/or JSON.
"""

import argparse
import csv
import datetime
import glob
import json
import os
import struct
import sys


CHROME_BASE = "~/Library/Application Support/Google/Chrome"


def varint(buf, pos):
    result = 0
    shift = 0
    while True:
        b = buf[pos]
        pos += 1
        result |= (b & 0x7F) << shift
        if not (b & 0x80):
            break
        shift += 7
    return result, pos


def parse_block(block):
    num_restarts = struct.unpack("<I", block[-4:])[0]
    records_end = len(block) - 4 - 4 * num_restarts
    entries = []
    pos = 0
    last_key = b""
    while pos < records_end:
        shared, pos = varint(block, pos)
        unshared, pos = varint(block, pos)
        vlen, pos = varint(block, pos)
        key = last_key[:shared] + block[pos : pos + unshared]
        pos += unshared
        value = block[pos : pos + vlen]
        pos += vlen
        entries.append((key, value))
        last_key = key
    return entries


def read_sst(path, snappy):
    with open(path, "rb") as f:
        data = f.read()
    footer = data[-48:]
    pos = 0
    _mi_off, pos = varint(footer, pos)
    _mi_size, pos = varint(footer, pos)
    idx_off, pos = varint(footer, pos)
    idx_size, pos = varint(footer, pos)

    def read_block(off, size):
        raw = data[off : off + size]
        ctype = data[off + size]
        if ctype == 0:
            return raw
        if ctype == 1:
            return snappy.decompress(raw)
        raise ValueError(f"unknown block compression type {ctype}")

    idx_entries = parse_block(read_block(idx_off, idx_size))
    all_entries = []
    for _, v in idx_entries:
        p = 0
        boff, p = varint(v, p)
        bsize, p = varint(v, p)
        try:
            all_entries.extend(parse_block(read_block(boff, bsize)))
        except Exception:
            continue
    return all_entries


def read_log(path, out):
    with open(path, "rb") as f:
        data = f.read()
    batches = []
    BLOCK = 32 * 1024
    frag = b""
    for bstart in range(0, len(data), BLOCK):
        block = data[bstart : bstart + BLOCK]
        pos = 0
        while pos + 7 <= len(block):
            length = struct.unpack("<H", block[pos + 4 : pos + 6])[0]
            rtype = block[pos + 6]
            if rtype == 0 and length == 0:
                break
            payload = block[pos + 7 : pos + 7 + length]
            pos += 7 + length
            if rtype in (1, 2):
                frag = payload
            elif rtype in (3, 4):
                frag += payload
            if rtype in (1, 4):
                batches.append(frag)
                frag = b""
    for batch in batches:
        if len(batch) < 12:
            continue
        count = struct.unpack("<I", batch[8:12])[0]
        p = 12
        for _ in range(count):
            if p >= len(batch):
                break
            tag = batch[p]
            p += 1
            if tag == 1:
                klen, p = varint(batch, p)
                k = batch[p : p + klen]
                p += klen
                vlen, p = varint(batch, p)
                v = batch[p : p + vlen]
                p += vlen
                out.append((k, v))
            elif tag == 0:
                klen, p = varint(batch, p)
                p += klen


def parse_proto(buf):
    fields = {}
    p = 0
    while p < len(buf):
        try:
            tag, p = varint(buf, p)
        except IndexError:
            break
        field = tag >> 3
        wire = tag & 7
        if wire == 0:
            val, p = varint(buf, p)
            fields.setdefault(field, []).append(val)
        elif wire == 2:
            ln, p = varint(buf, p)
            fields.setdefault(field, []).append(buf[p : p + ln])
            p += ln
        elif wire == 1:
            p += 8
        elif wire == 5:
            p += 4
        else:
            break
    return fields


STATUS = {0: "UNREAD", 1: "READ", 2: "UNSEEN"}


def extract_reading_list(leveldb_dir, snappy):
    all_entries = []
    for p in sorted(glob.glob(os.path.join(leveldb_dir, "*.ldb"))):
        all_entries.extend(read_sst(p, snappy))
    for p in sorted(glob.glob(os.path.join(leveldb_dir, "*.log"))):
        read_log(p, all_entries)

    kv = {}
    for k, v in all_entries:
        kv[k] = v

    rows = []
    prefix = b"reading_list-dt-"
    for k, v in kv.items():
        if not k.startswith(prefix):
            continue
        outer = parse_proto(v)
        inner = None
        for vals in outer.values():
            for val in vals:
                if isinstance(val, (bytes, bytearray)):
                    sub = parse_proto(val)
                    if 3 in sub and any(
                        isinstance(x, bytes) and b"http" in x for x in sub[3]
                    ):
                        inner = sub
                        break
            if inner:
                break
        if inner is None and 3 in outer and any(
            isinstance(x, bytes) and b"http" in x for x in outer[3]
        ):
            inner = outer
        if not inner:
            continue

        def getstr(f):
            for x in inner.get(f, []): # pyright: ignore[reportOptionalMemberAccess]
                if isinstance(x, bytes):
                    try:
                        return x.decode("utf-8")
                    except UnicodeDecodeError:
                        return x.decode("utf-8", "replace")
            return ""

        def getint(f):
            vals = inner.get(f, []) # pyright: ignore[reportOptionalMemberAccess]
            return vals[0] if vals and isinstance(vals[0], int) else None

        rows.append({
            "url": getstr(3),
            "title": getstr(2),
            "status": STATUS.get(getint(6) or 0, "?"),
            "created_us": getint(4),
            "updated_us": getint(5),
        })

    by_url = {}
    for r in rows:
        u = r["url"]
        if not u:
            continue
        if u not in by_url or (r.get("updated_us") or 0) > (by_url[u].get("updated_us") or 0):
            by_url[u] = r
    return sorted(by_url.values(), key=lambda r: -(r.get("updated_us") or 0))


def fmt_ts(us):
    if not us:
        return ""
    return datetime.datetime.fromtimestamp(us / 1_000_000).strftime("%Y-%m-%d %H:%M:%S")


def write_csv(rows, path):
    with open(path, "w", newline="") as f:
        w = csv.writer(f)
        w.writerow(["status", "created", "updated", "title", "url"])
        for r in rows:
            w.writerow([
                r["status"],
                fmt_ts(r.get("created_us")),
                fmt_ts(r.get("updated_us")),
                r["title"],
                r["url"],
            ])


def write_json(rows, path):
    payload = [
        {
            "status": r["status"],
            "created": fmt_ts(r.get("created_us")),
            "updated": fmt_ts(r.get("updated_us")),
            "created_us": r.get("created_us"),
            "updated_us": r.get("updated_us"),
            "title": r["title"],
            "url": r["url"],
        }
        for r in rows
    ]
    with open(path, "w") as f:
        json.dump(payload, f, indent=2)


def main(argv=None):
    ap = argparse.ArgumentParser(description="Extract Chrome Reading List to CSV/JSON.")
    ap.add_argument("--profile", default="Default", help="Chrome profile directory name (default: Default)")
    ap.add_argument("--out", default="~/Desktop/chrome-reading-list.csv", help="Output path")
    ap.add_argument("--format", choices=["csv", "json", "both"], default="csv")
    group = ap.add_mutually_exclusive_group()
    group.add_argument("--unread-only", action="store_true", help="Only UNREAD entries")
    group.add_argument("--read-only", action="store_true", help="Only READ entries")
    args = ap.parse_args(argv)

    try:
        import snappy
    except ImportError:
        print(
            "error: python-snappy not installed. Install with:\n"
            "  pip3 install --user python-snappy",
            file=sys.stderr,
        )
        return 1

    leveldb_dir = os.path.expanduser(os.path.join(CHROME_BASE, args.profile, "Sync Data/LevelDB"))
    if not os.path.isdir(leveldb_dir):
        print(f"error: LevelDB dir not found: {leveldb_dir}", file=sys.stderr)
        return 2

    try:
        rows = extract_reading_list(leveldb_dir, snappy)
    except Exception as e:
        print(f"error: parse failed: {e}", file=sys.stderr)
        return 3

    if args.unread_only:
        rows = [r for r in rows if r["status"] == "UNREAD"]
    elif args.read_only:
        rows = [r for r in rows if r["status"] == "READ"]

    out_path = os.path.expanduser(args.out)
    os.makedirs(os.path.dirname(out_path) or ".", exist_ok=True)

    if args.format == "csv":
        write_csv(rows, out_path)
        written = [out_path]
    elif args.format == "json":
        write_json(rows, out_path)
        written = [out_path]
    else:
        base, _ = os.path.splitext(out_path)
        csv_path = base + ".csv"
        json_path = base + ".json"
        write_csv(rows, csv_path)
        write_json(rows, json_path)
        written = [csv_path, json_path]

    counts = {}
    for r in rows:
        counts[r["status"]] = counts.get(r["status"], 0) + 1
    breakdown = ", ".join(f"{k}={v}" for k, v in sorted(counts.items()))
    print(f"wrote {len(rows)} entries ({breakdown}) to {', '.join(written)}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
