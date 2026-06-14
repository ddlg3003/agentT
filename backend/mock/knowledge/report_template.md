# PO Report Template — [Cashloan][PO][Report] {MONTH}.{YEAR}

The monthly report is a per-partner funnel review, month-over-month. Follow this
structure exactly.

## Performance Snapshot
One short paragraph: overall E2E conversion across partners, naming the MoM move per
partner (e.g. "modest MoM gains at SHB (+0.3pp) and TNEX (+0.3pp) offset by a slight
decline at CAKE (−0.3pp)").

Then, **one block per partner** (SHB, CAKE, TNEX, VP …), each containing:

### {PARTNER} — E2E: {value}% ({±X.Xpp MoM})
A 2–4 sentence narrative: what moved and why, tying each driver to a ticket/MR or a
business rule. State hypotheses as hypotheses, not facts.

**Summary** (compared to previous month) — bullet list, one per funnel step that moved:
- {Metric} {±Xpp} ({from}→{to}) → {explanation tied to evidence}

Then a funnel table across the last 3 months:

| Step | Metric | {M-2} | {M-1} | {M} |
|------|--------|-------|-------|-----|
| s10 | Whitelist (#) | ... | ... | ... |
| s20 | Traffic (#) | ... | ... | ... |
| s20s30 | Demand Rate (%) | ... | ... | ... |
| s30s40 | Pass Rule Rate (%) | ... | ... | ... |
| s40s70 | Filling Application Rate (%) | ... | ... | ... |
| s20s70 | Submission Rate (%) | ... | ... | ... |
| (BE) | Approval Rate (%) | ... | ... | ... |
| s100s120 | Signing Rate (%) | ... | ... | ... |
| s20s120 | E2E Rate (%) | ... | ... | ... |

(Show MoM deltas inline next to each cell, e.g. "93k (-5%)".)

## Top 3 priorities in next months
Numbered list of the 3 most important initiatives, each with a short rationale.

## Progress from AP
- Planning links / roadmap references (if available in the digests)
- Biggest risk / bottleneck (or N/A)
