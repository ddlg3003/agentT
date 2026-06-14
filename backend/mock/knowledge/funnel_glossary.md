# Funnel Glossary — Cash Loan

| Step code | Metric | Meaning |
|-----------|--------|---------|
| s10 | Whitelist (#) | Eligible user pool exposed to the offer |
| s20 | Traffic (#) | Users who actually arrived / accessed the flow |
| s20s30 | Demand Rate (%) | Of Traffic, share expressing borrowing demand |
| s30s40 | Pass Rule Rate (%) | Of demand, share passing eligibility rules |
| s40s70 | Filling Application Rate (%) | Share completing the application form |
| s20s70 | Submission Rate (%) | Of Traffic, share submitting an application |
| (BE) | Approval Rate (%) | Approved / Submitted (back-end credit decision) |
| s100s120 | Signing Rate (%) | Of approved, share signing the contract |
| s20s120 | E2E Rate (%) | End-to-end: signed / Traffic — the headline number |

Partners: SHB, CAKE, TNEX, VP. Each has its own funnel; never sum rates across
partners — compare them side by side.

Rates in the BI data are fractions (0.21 = 21%). Convert to percent in reports.
Deltas for rates are reported in percentage points (pp); deltas for counts (#) in %.
