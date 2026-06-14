# Business Rules for Cash Loan Funnel Interpretation

Use these rules to interpret metric movements. Do NOT assert a cause for a metric
change unless a corresponding ticket/MR (from query_jira / query_gitlab) or one of
these rules supports it.

## Funnel structure (per partner)
Whitelist (s10) → Traffic (s20) → Demand Rate (s20s30) → Pass Rule Rate (s30s40)
→ Filling Application Rate (s40s70) → Submission Rate (s20s70) → Approval Rate (BE)
→ Signing Rate (s100s120) → E2E Rate (s20s120).

E2E Rate is the headline conversion: of users who arrived (Traffic), how many ended
with a signed/disbursed loan. Report deltas in percentage points (pp) for rates.

## Interpretation rules
- A **no-score segment launch** brings in users with no credit history → it raises
  funnel volume but **lowers Approval Rate** over the following days/weeks. Expect E2E
  to dip before it recovers.
- A **rule migration** that moves the valid-rule check from the init step to the
  submit step **inflates Pass Rule Rate and Submission Rate** (fewer users filtered
  early) but the effect is **offset at Approval Rate** — net E2E impact is usually small.
- A **product shutdown** (e.g. Payday) removes a demand-generating segment → expect a
  **Demand Rate decline** concentrated on and after the shutdown date.
- A **full embedded-flow rollout** broadens exposure → **Traffic surge** plus a new user
  segment; demand metrics may temporarily inflate then normalize over 1–2 months.
- **eKYC failures** above 5% are incident-level; below 5% is background noise. eKYC
  issues hurt the Filling Application Rate (s40s70).
- Passive traffic sources fluctuate day to day; a single-day Traffic dip in one source
  (e.g. "Search bar") is not an anomaly unless it persists 3+ days.
- Signing Rate is normally stable (94–99%); movements >2pp warrant a note.
