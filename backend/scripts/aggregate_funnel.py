#!/usr/bin/env python3
"""
aggregate_funnel.py — Run the Cash Loan funnel aggregation offline over the
anonymized event CSV and emit per-partner funnel metrics as JSON in the SAME
shape that internal/infra/tools/bi.go reads (metric_catalog + records).

This reproduces the production funnel SQL semantics in plain Python:
  - sub_event_id -> funnel step (with metadata.status splitting the UW step into
    success/fail).
  - USER-LEVEL dedup per period: a user counts once per step per period
    ("did this user EVER hit that step in the period").
  - Funnel population = users who loaded the demand page (step20). Every step
    count and every rate is computed within that population (the query's
    HAVING Step20 > 0).
  - Rates are stored as FRACTIONS (0.21 = 21%) to match the existing mock schema.

Timezone: event_time in the CSV is UTC ('...Z'); the production query works in
+07:00. We shift UTC -> +07:00 before taking the calendar day, so daily buckets
match the Vietnam business day used by the BI team.

Outputs:
  - <out_dir>/daily_metrics.from_events.json   (daily records, bi.go schema)
  - <out_dir>/monthly_metrics.json             (monthly records, re-deduped)
  - prints a monthly funnel overview table per partner to stdout.

Usage:
    python3 aggregate_funnel.py input_anon.csv out_dir
    python3 aggregate_funnel.py input_anon.csv out_dir --partners SHB,CAKE,TNEX
"""

import argparse
import csv
import json
import os
import sys
from datetime import datetime, timedelta

csv.field_size_limit(10 * 1024 * 1024)

VN_OFFSET = timedelta(hours=7)  # UTC -> Asia/Ho_Chi_Minh

# Funnel step -> bit position (compact per-user membership mask).
STEP_BITS = {
    "step20": 1 << 0,   # 6912.000  load demand page  (= Traffic)
    "step30": 1 << 1,   # 6912.001  click register
    "step40": 1 << 2,   # 6914.000  load personal info
    "step50": 1 << 3,   # 6914.001  confirm info
    "step60": 1 << 4,   # 6915.000  load summary
    "step70": 1 << 5,   # 6915.001  confirm summary (submit)
    "step71": 1 << 6,   # 6916.000  load UW screen
    "step72": 1 << 7,   # 6916.000 + status=underwriting_success_result
    "step73": 1 << 8,   # 6916.000 + status=underwriting_fail_result
    "step80": 1 << 9,   # 6917.000  load contract
    "step90": 1 << 10,  # 6917.003  agree contract
    "step100": 1 << 11, # 6918.000  load OTP
    "step110": 1 << 12, # 6918.001  verify OTP
    "step120": 1 << 13, # 6919.000  load waiting (= E2E success)
}

# Direct sub_event_id -> step (the UW step 6916.000 is handled separately).
SUB_EVENT_STEP = {
    "6912.000": "step20",
    "6912.001": "step30",
    "6914.000": "step40",
    "6914.001": "step50",
    "6915.000": "step60",
    "6915.001": "step70",
    "6917.000": "step80",
    "6917.003": "step90",
    "6918.000": "step100",
    "6918.001": "step110",
    "6919.000": "step120",
}

# Catalog mirrors the existing daily_metrics.json so bi.go reads it unchanged.
# whitelist is BE/external data (not in the event stream) -> intentionally omitted.
METRIC_CATALOG = [
    {"step": "s20", "key": "traffic", "name": "Traffic (#)", "unit": "#"},
    {"step": "s20s30", "key": "demand_rate", "name": "Demand Rate", "unit": "%"},
    {"step": "s30s40", "key": "pass_rule_rate", "name": "Pass Rule Rate", "unit": "%"},
    {"step": "s40s70", "key": "filling_rate", "name": "Filling Application Rate", "unit": "%"},
    {"step": "s20s70", "key": "submission_rate", "name": "Submission Rate", "unit": "%"},
    {"step": "be", "key": "approval_rate", "name": "Approval Rate", "unit": "%"},
    {"step": "s100s120", "key": "signing_rate", "name": "Signing Rate", "unit": "%"},
    {"step": "s20s120", "key": "e2e_rate", "name": "E2E Rate", "unit": "%"},
]


def steps_for(sub_event_id: str, status):
    """Return the list of funnel steps a row contributes to."""
    if sub_event_id == "6916.000":
        steps = ["step71"]
        if status == "underwriting_success_result":
            steps.append("step72")
        elif status == "underwriting_fail_result":
            steps.append("step73")
        return steps
    step = SUB_EVENT_STEP.get(sub_event_id)
    return [step] if step else []


def vn_day_month(event_time: str):
    """UTC ISO timestamp -> (YYYY-MM-DD, YYYY-MM) in +07:00. Returns (None, None) on parse error."""
    # Robust parse: take 'YYYY-MM-DDTHH:MM:SS', ignore fractional seconds / trailing Z.
    try:
        base = datetime.strptime(event_time[:19], "%Y-%m-%dT%H:%M:%S")
    except (ValueError, TypeError):
        return None, None
    vn = base + VN_OFFSET
    return vn.strftime("%Y-%m-%d"), vn.strftime("%Y-%m")


def safe_div(num: int, den: int):
    """num/den as a 4-decimal fraction, or None when the denominator is 0."""
    if den == 0:
        return None
    return round(num / den, 4)


def metrics_from_counts(c: dict) -> dict:
    """Build the catalog metric values from per-step distinct-user counts."""
    values = {}
    values["traffic"] = c.get("step20", 0)
    pairs = {
        "demand_rate": ("step30", "step20"),
        "pass_rule_rate": ("step40", "step30"),
        "filling_rate": ("step70", "step40"),   # S40S70 Application Filling
        "submission_rate": ("step70", "step20"),  # S20S70
        "signing_rate": ("step120", "step100"),  # S100S120
        "e2e_rate": ("step120", "step20"),        # S20S120
    }
    for key, (num, den) in pairs.items():
        r = safe_div(c.get(num, 0), c.get(den, 0))
        if r is not None:
            values[key] = r
    # Approval = step100 / (step73_fail + step100)  (the query's S72S100_Approval_rate)
    appr = safe_div(c.get("step100", 0), c.get("step73", 0) + c.get("step100", 0))
    if appr is not None:
        values["approval_rate"] = appr
    return values


def aggregate(path: str, partners_filter):
    """One pass over the CSV. Returns (daily, monthly) dicts:
    {period: {partner: {step: distinct_user_count}}}.
    """
    # period -> partner -> user -> step-bitmask, kept for daily and monthly separately.
    day_masks = {}
    month_masks = {}
    rows = total_skipped_partner = total_unmapped = 0

    with open(path, newline="", encoding="utf-8") as f:
        reader = csv.DictReader(f)
        for row in reader:
            rows += 1
            try:
                meta = json.loads(row.get("metadata") or "{}")
            except (json.JSONDecodeError, ValueError):
                meta = {}
            partner = meta.get("partner_code")
            if not partner:
                total_skipped_partner += 1
                continue
            if partners_filter and partner not in partners_filter:
                continue

            steps = steps_for(row.get("sub_event_id", ""), meta.get("status"))
            if not steps:
                total_unmapped += 1
                continue

            day, month = vn_day_month(row.get("event_time", ""))
            if day is None:
                continue
            user = row.get("user_id", "")
            bit = 0
            for s in steps:
                bit |= STEP_BITS[s]

            for store, period in ((day_masks, day), (month_masks, month)):
                pp = store.setdefault(period, {}).setdefault(partner, {})
                pp[user] = pp.get(user, 0) | bit

    def to_counts(masks):
        # Collapse per-user masks into per-step distinct-user counts, restricted
        # to the step20 population (HAVING Step20 > 0).
        out = {}
        step20 = STEP_BITS["step20"]
        for period, partners in masks.items():
            for partner, users in partners.items():
                counts = {}
                for mask in users.values():
                    if not (mask & step20):
                        continue  # not in funnel population
                    for step, bit in STEP_BITS.items():
                        if mask & bit:
                            counts[step] = counts.get(step, 0) + 1
                if counts:
                    out.setdefault(period, {})[partner] = counts
        return out

    stats = {
        "rows": rows,
        "skipped_no_partner": total_skipped_partner,
        "unmapped_sub_event": total_unmapped,
    }
    return to_counts(day_masks), to_counts(month_masks), stats


def to_records(counts: dict) -> list:
    """period->partner->stepcounts  ->  sorted [{date, partner, values}] records."""
    records = []
    for period in sorted(counts):
        for partner in sorted(counts[period]):
            records.append(
                {
                    "date": period,
                    "partner": partner,
                    "values": metrics_from_counts(counts[period][partner]),
                }
            )
    return records


def print_overview(monthly_counts: dict):
    """Print a per-partner monthly funnel table to stdout (eyeball check)."""
    rate_keys = [
        ("traffic", "Traffic (#)"),
        ("demand_rate", "Demand Rate"),
        ("pass_rule_rate", "Pass Rule Rate"),
        ("filling_rate", "Filling Rate"),
        ("submission_rate", "Submission Rate"),
        ("approval_rate", "Approval Rate"),
        ("signing_rate", "Signing Rate"),
        ("e2e_rate", "E2E Rate"),
    ]
    months = sorted(monthly_counts)
    partners = sorted({p for m in monthly_counts.values() for p in m})

    for partner in partners:
        print(f"\n=== {partner} — monthly funnel ===")
        header = f"{'Metric':<18}" + "".join(f"{m:>14}" for m in months)
        print(header)
        print("-" * len(header))
        for key, label in rate_keys:
            cells = []
            for m in months:
                counts = monthly_counts.get(m, {}).get(partner)
                if not counts:
                    cells.append(f"{'-':>14}")
                    continue
                vals = metrics_from_counts(counts)
                v = vals.get(key)
                if v is None:
                    cells.append(f"{'-':>14}")
                elif key == "traffic":
                    cells.append(f"{int(v):>14,}")
                else:
                    cells.append(f"{v * 100:>13.1f}%")
            print(f"{label:<18}" + "".join(cells))


# Step columns in the production query's final SELECT, in order.
VERIFY_STEP_COLS = [
    ("Step20_load_input_demand_page", "step20"),
    ("Step30_click_register_now", "step30"),
    ("Step40_load_personal_info", "step40"),
    ("Step50_click_confirm_info", "step50"),
    ("Step60_load_summary", "step60"),
    ("Step70_click_confirm_summary", "step70"),
    ("Step71_load_UW_screen", "step71"),
    ("Step72_load_underwriting_success", "step72"),
    ("Step73_load_underwriting_failed", "step73"),
    ("Step80_load_contract", "step80"),
    ("Step90_click_agree_contract", "step90"),
    ("Step100_load_otp", "step100"),
    ("Step110_verify_otp", "step110"),
    ("Step120_load_waitting_UW", "step120"),
]


def verify_rates(c: dict) -> dict:
    """Reproduce the production query's rate columns (as percentages ×100)."""
    def pct(num_step, den_steps):
        den = sum(c.get(s, 0) for s in den_steps)
        if den == 0:
            return ""
        num = c.get(num_step, 0)
        return round(100.0 * num / den, 2)

    return {
        "S20S30_Demand": pct("step30", ["step20"]),
        "S30S40_Pass_Rule": pct("step40", ["step30"]),
        "S40S50": pct("step50", ["step40"]),
        "S50S60": pct("step60", ["step50"]),
        "S40S70_Application_Filling": pct("step70", ["step40"]),
        "S60S70": pct("step70", ["step60"]),
        "S70S71_loadUW_from_submit": pct("step71", ["step70"]),
        "S70S100_Submission_success_rate": pct("step100", ["step70"]),
        "S72S100_Approval_rate": pct("step100", ["step73", "step100"]),
        "S73_Failed_rate": pct("step73", ["step73", "step72"]),
        "S20S70_Submission_rate": pct("step70", ["step20"]),
        "S100S120_Signing": pct("step120", ["step100"]),
        "S20S120_E2E": pct("step120", ["step20"]),
    }


def write_verify_csv(counts: dict, period_col: str, path: str):
    """Dump every step count + every query rate column per (period, partner) so the
    output can be diffed column-by-column against the production warehouse query."""
    rate_cols = list(verify_rates({}).keys())
    fieldnames = (
        [period_col, "partner"]
        + [name for name, _ in VERIFY_STEP_COLS]
        + rate_cols
    )
    with open(path, "w", newline="", encoding="utf-8") as f:
        w = csv.DictWriter(f, fieldnames=fieldnames)
        w.writeheader()
        for period in sorted(counts):
            for partner in sorted(counts[period]):
                c = counts[period][partner]
                row = {period_col: period, "partner": partner}
                for name, step in VERIFY_STEP_COLS:
                    row[name] = c.get(step, 0)
                row.update(verify_rates(c))
                w.writerow(row)


def main() -> int:
    ap = argparse.ArgumentParser(description="Offline Cash Loan funnel aggregation.")
    ap.add_argument("input", help="anonymized event CSV")
    ap.add_argument("out_dir", help="output directory for JSON files")
    ap.add_argument(
        "--partners",
        default="",
        help="optional comma-separated partner_code filter, e.g. SHB,CAKE,TNEX",
    )
    args = ap.parse_args()

    partners_filter = {p.strip() for p in args.partners.split(",") if p.strip()}

    daily_counts, monthly_counts, stats = aggregate(args.input, partners_filter)

    os.makedirs(args.out_dir, exist_ok=True)
    daily_path = os.path.join(args.out_dir, "daily_metrics.from_events.json")
    monthly_path = os.path.join(args.out_dir, "monthly_metrics.json")

    with open(daily_path, "w", encoding="utf-8") as f:
        json.dump(
            {"metric_catalog": METRIC_CATALOG, "records": to_records(daily_counts)},
            f,
            ensure_ascii=False,
            indent=2,
        )
    with open(monthly_path, "w", encoding="utf-8") as f:
        json.dump(
            {"metric_catalog": METRIC_CATALOG, "records": to_records(monthly_counts)},
            f,
            ensure_ascii=False,
            indent=2,
        )

    verify_monthly = os.path.join(args.out_dir, "funnel_verify_monthly.csv")
    verify_daily = os.path.join(args.out_dir, "funnel_verify_daily.csv")
    write_verify_csv(monthly_counts, "event_period", verify_monthly)
    write_verify_csv(daily_counts, "event_date", verify_daily)

    print(
        f"OK: {stats['rows']:,} rows | "
        f"skipped(no partner)={stats['skipped_no_partner']:,} | "
        f"unmapped sub_event={stats['unmapped_sub_event']:,}"
    )
    print(f"  daily   -> {daily_path}")
    print(f"  monthly -> {monthly_path}")
    print(f"  verify(monthly) -> {verify_monthly}")
    print(f"  verify(daily)   -> {verify_daily}")
    print_overview(monthly_counts)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
