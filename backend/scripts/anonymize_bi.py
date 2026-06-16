#!/usr/bin/env python3
"""
anonymize_bi.py — Anonymize the Cash Loan production event-tracking CSV so it can
be used as safe mock data (no PII) while preserving structure + cardinality.

Two jobs:
  1. PII (user_id, zalo_id) -> fake numeric of the SAME LENGTH, deterministic:
     a given value A always maps to the same fake value F (even on re-runs or on
     a different row). This keeps the distinct-user count intact (injective map).
  2. metadata JSON -> keep only the fields the funnel query actually uses
     (partner_code, partner, status); drop everything else (application_id,
     contract_id, due_amount, due_date, utm_source, cta, product_line, ...).

Input : original CSV (same header as production).
Output: anonymized CSV, identical column schema, ready to use as mock BI data.

Usage:
    python3 anonymize_bi.py input.csv output.csv
    python3 anonymize_bi.py input.csv output.csv --salt my-secret-salt

Streams row by row -> handles 1M+ row files without blowing up RAM.
"""

import argparse
import csv
import hashlib
import json
import sys

# metadata fields the funnel query uses — keep these, drop the rest.
KEEP_META_FIELDS = ("partner_code", "partner", "status")

# Columns holding PII to anonymize (preserve length, deterministic mapping).
PII_COLUMNS = ("user_id", "zalo_id")

# Allow very long metadata cells.
csv.field_size_limit(10 * 1024 * 1024)


def fake_numeric(value: str, salt: str) -> str:
    """Map an identifier to a fake numeric string of the SAME LENGTH, deterministic.

    Same (salt, value) -> always the same result. The original value cannot be
    recovered (one-way hash). The first character is always 1-9 so no leading
    zero is dropped.
    """
    if value == "" or value is None:
        return value  # keep empty cells empty (e.g. blank zalo_id)
    digest = hashlib.sha256((salt + "|" + value).encode("utf-8")).hexdigest()
    # int(hex) -> a large decimal; str() has no leading zero (first digit is 1-9).
    digits = str(int(digest, 16))
    target_len = len(value)
    while len(digits) < target_len:
        digits += digits
    return digits[:target_len]


def clean_metadata(raw: str) -> str:
    """Parse the metadata JSON, keep only KEEP_META_FIELDS, re-serialize compactly.

    Returns '{}' on empty / parse failure (counted separately for the final report).
    """
    if not raw or not raw.strip():
        return "{}", False
    try:
        obj = json.loads(raw)
    except (json.JSONDecodeError, ValueError):
        return "{}", True  # parse fail
    if not isinstance(obj, dict):
        return "{}", True
    kept = {k: obj[k] for k in KEEP_META_FIELDS if k in obj}
    # Compact separators; ensure_ascii=False to keep Unicode if present.
    return json.dumps(kept, ensure_ascii=False, separators=(",", ":")), False


def main() -> int:
    ap = argparse.ArgumentParser(description="Anonymize Cash Loan BI event CSV.")
    ap.add_argument("input", help="path to the original CSV")
    ap.add_argument("output", help="path to write the anonymized CSV")
    ap.add_argument(
        "--salt",
        default="agentT-bi-mock",
        help="salt for PII hashing (changing the salt changes the mapping). "
        "Fixed by default so re-runs produce identical output.",
    )
    args = ap.parse_args()

    rows = 0
    meta_parse_fail = 0
    distinct = {col: set() for col in PII_COLUMNS}

    with open(args.input, newline="", encoding="utf-8") as fin, open(
        args.output, "w", newline="", encoding="utf-8"
    ) as fout:
        reader = csv.DictReader(fin)
        if reader.fieldnames is None:
            print("ERROR: input CSV is empty or has no header", file=sys.stderr)
            return 1

        missing = [c for c in PII_COLUMNS if c not in reader.fieldnames]
        if "metadata" not in reader.fieldnames:
            missing.append("metadata")
        if missing:
            print(
                f"ERROR: required columns missing from CSV: {missing}\n"
                f"Header found: {reader.fieldnames}",
                file=sys.stderr,
            )
            return 1

        writer = csv.DictWriter(
            fout, fieldnames=reader.fieldnames, quoting=csv.QUOTE_MINIMAL
        )
        writer.writeheader()

        for row in reader:
            for col in PII_COLUMNS:
                original = row.get(col, "")
                if original:
                    distinct[col].add(original)
                row[col] = fake_numeric(original, args.salt)

            cleaned, failed = clean_metadata(row.get("metadata", ""))
            row["metadata"] = cleaned
            if failed:
                meta_parse_fail += 1

            writer.writerow(row)
            rows += 1

    print(f"OK: processed {rows:,} rows -> {args.output}")
    for col in PII_COLUMNS:
        print(f"  {col}: {len(distinct[col]):,} distinct values (cardinality preserved)")
    print(f"  metadata: kept {list(KEEP_META_FIELDS)}; parse fail = {meta_parse_fail:,}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
