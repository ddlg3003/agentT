# AgentT — Sales / PO Intelligence Agent (MVP)

A daily-run **agentic system** that helps a Product Owner understand the Cash Loan
product. Every day it reads the numbers (BI), the work that shipped (Jira / Gitlab),
and the business rules, then reasons over them to produce a **Daily Digest**: what
moved in the conversion funnel, and *why* — with every number traceable to its source.

The PO can ask follow-up questions about a digest, correct it in conversation, and at
month end the agent synthesizes all digests into a **month-over-month report** per
partner.

> **MVP note:** all external data sources (BI / Jira / Gitlab / knowledge base) are
> mocked. They sit behind tool ports, so swapping in real clients later does **not**
> touch the agent loop or prompts.

---

## What it does

```
        ┌─────────────────────────── daily (auto @ 06:00, or PO clicks "create digest")
        ▼
   ┌─────────┐   query_bi / query_jira / query_gitlab / get_knowledge
   │  AGENT  │ ◀──────────────────────────────────────────────►  mock data
   │  LOOP   │   think → act → observe → repeat
   └─────────┘
        │  produces
        ▼
   ┌───────────────┐        ┌──────────────────┐        ┌────────────────────┐
   │  Daily Digest │  ───▶  │  Follow-up Q&A    │  ───▶  │  Monthly Rollup     │
   │  (per-partner │        │  + PO corrections │        │  (MoM report per    │
   │   funnel)     │        │  (can edit digest)│        │   partner)          │
   └───────────────┘        └──────────────────┘        └────────────────────┘
        │ persisted to SQLite (with full audit trail of every tool call)
```

### The domain: a per-partner conversion funnel

The product is a Cash Loan funnel, tracked per partner (SHB / CAKE / TNEX / VP …),
month-over-month. The agent reasons over these steps:

| Step | Metric | |
|------|--------|--|
| s10 | Whitelist (#) | eligible user pool |
| s20 | Traffic (#) | users who arrived |
| s20s30 | Demand Rate | expressed borrowing demand |
| s30s40 | Pass Rule Rate | passed eligibility rules |
| s40s70 | Filling Rate | completed the form |
| s20s70 | Submission Rate | submitted an application |
| (BE) | Approval Rate | approved / submitted |
| s100s120 | Signing Rate | signed the contract |
| **s20s120** | **E2E Rate** | **headline: signed / traffic** |

It correlates each movement with an event (a ticket or merge request) or a business
rule — and is constrained to **not invent causality** without that evidence.

### Three things that make the digest trustworthy

1. **Traceability** — the loop records every tool call (input + output) into the
   digest's `sources`. Any number is answerable in two clicks: "where did this come
   from?"
2. **PO corrections are audited** — when the PO corrects a number via Q&A, the
   original value is preserved in `corrections`; nothing is silently overwritten.
3. **Read-only by design** — the agent can only read external sources. The single
   write tool mutates *only* the digest store, and only in the follow-up loop.

---

## How it works (design)

One reusable **agent loop** (`internal/usecase/loop.go`), three invocations that differ
only in their prompt and tool set:

| Loop | Tools | Writes? |
|------|-------|---------|
| **Daily digest** | `query_bi`, `query_jira`, `query_gitlab`, `get_knowledge` | no (read-only) |
| **Follow-up Q&A** | the 4 read tools **+ `update_digest`** | digest store only |
| **Monthly rollup** | read tools (mainly the stored digests) | no |

Loop guarantees (from the agentic-harness design principles):
- stops on **content** (no tool calls), never the provider's `stop_reason`;
- **hard turn limit** → explicit error, never a silent partial digest;
- a failing tool returns `[DATA UNAVAILABLE: …]` and the loop continues;
- tools are sequential and read-only **by code**, not by instruction.

Architecture is hexagonal — `domain ← usecase ← adapter/infra`. The LLM, the digest
store, and the tools are all swappable behind ports. See [`CLAUDE.md`](./CLAUDE.md)
for the full layout and rules.

---

## Run it locally

**Prereqs:** Go 1.24+, Node 22+, pnpm (`corepack enable`).

The agent loops need a **tool-calling model**. Set an API key first — the `echo` stub
keeps the server up but cannot produce real digests.

```bash
cd backend
cp .env.example .env
# edit .env: set ANTHROPIC_API_KEY=sk-ant-...   (or OPENAI_API_KEY=...)
```

```bash
# backend → http://localhost:8080  (reads backend/.env automatically)
make be-run
```

The startup log confirms the wiring:

```
llm provider: anthropic
digest store: sqlite  path=./digests.db
daily scheduler started  at=06:00 local
server listening  addr=:8080
```

### Generate a digest and explore it

```bash
# 1) Create a daily digest (the PO "create digest" action; data exists for 2026-03-15)
curl -s -X POST localhost:8080/api/v1/jobs/daily \
  -H 'Content-Type: application/json' -d '{"date":"2026-03-15"}' | jq

# 2) List dates that have a digest
curl -s localhost:8080/api/v1/digests | jq

# 3) Read the full digest (metrics, events, reasoning, sources)
curl -s localhost:8080/api/v1/digests/2026-03-15 | jq

# 4) Ask a follow-up question
curl -s -X POST localhost:8080/api/v1/digests/2026-03-15/ask \
  -H 'Content-Type: application/json' \
  -d '{"userId":"po@vng","question":"Why did CAKE demand rate drop?"}' | jq

# 5) Correct a number (PO in the loop) — original value is kept in the audit trail
curl -s -X POST localhost:8080/api/v1/digests/2026-03-15/ask \
  -H 'Content-Type: application/json' \
  -d '{"userId":"po@vng","question":"The SHB E2E rate is wrong, it should be 4.5%. Please fix it."}' | jq

# 6) Generate the monthly report once you have several digests for a month
curl -s localhost:8080/api/v1/report/monthly/2026-03 | jq -r .markdown
```

> Generate a few March dates (`2026-03-13`, `-14`, `-15`) before the monthly rollup —
> it synthesizes whatever digests are stored for the month.

### Frontend

```bash
make fe-install
make fe-dev        # http://localhost:5173  (proxies /api → backend)
```

*(The digest UI is the next step; the current frontend ships the chat scaffold.)*

---

## API

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/api/v1/digests` | list dates that have a digest |
| `GET` | `/api/v1/digests/{date}` | full Daily Digest |
| `POST` | `/api/v1/digests/{date}/ask` | follow-up Q&A (may apply a PO correction) |
| `PATCH` | `/api/v1/digests/{date}/flag` | flag a digest as incorrect |
| `POST` | `/api/v1/jobs/daily` | run the daily job for a date |
| `POST` | `/api/v1/jobs/monthly` | run the monthly rollup |
| `GET` | `/api/v1/report/monthly/{ym}` | monthly report (`ym` = `2026-03`) |

The daily job also runs **automatically once per day** at a fixed local time
(`internal/scheduler`); `POST /jobs/daily` is the manual / on-demand trigger.

---

## Configuration

| Var | Purpose | Default |
|-----|---------|---------|
| `PORT` | backend port | `8080` |
| `LLM_PROVIDER` | `anthropic` / `openai` / `echo` | auto from API key |
| `ANTHROPIC_API_KEY` / `ANTHROPIC_MODEL` | Claude provider | — / `claude-opus-4-8` |
| `OPENAI_API_KEY` / `OPENAI_MODEL` | OpenAI-compatible provider | — / `gpt-4o` |
| `MOCK_DIR` | base dir for mock data tools | `./mock` |
| `DIGEST_DB_PATH` | SQLite digest store path | `./digests.db` |
| `MEMORY_BACKEND` | `memory` (in-process) / `greennode` | auto |

See [`backend/.env.example`](./backend/.env.example) for the full list, including the
GreenNode AgentBase memory backend.

---

## Project layout

```
backend/   Go 1.24, hexagonal architecture (module github.com/vngcloud/agentt)
  cmd/server/                HTTP entry point
  internal/app/              composition root — wires everything
  internal/usecase/          agent loop + daily / followup / monthly + embedded prompts
  internal/domain/           ports & entities: agent (LLM + tools), digest, memory
  internal/infra/            llm (anthropic/openai/echo), tools, digeststore (SQLite)
  internal/scheduler/        daily job scheduler
  mock/                      mock BI / Jira / Gitlab / knowledge data
  pkg/greennode/             vendor-isolated AgentBase SDK (deploy + memory)
frontend/  React 18 + Vite + TypeScript
```

## Develop

```bash
make be-test     # go test -race ./...
make be-vet      # go vet
make be-build    # build binary → backend/bin/server
make up          # docker demo stack
```

To add a real data source, reimplement a tool's `Run` in `internal/infra/tools/`.
To add an LLM provider, implement `agent.ToolCaller` in `internal/infra/llm/` and wire
it in `internal/app`. Nothing in the agent loop changes.

---

## Status

- ✅ Backend MVP: daily / follow-up / monthly loops, SQLite persistence, PO
  corrections with audit trail, daily scheduler + manual trigger, REST API.
- ✅ Tool-calling for Anthropic and OpenAI; tested with `-race`.
- ⏳ Frontend digest UI (list / view / ask / monthly report).
- ⏳ Swap mock tools for real BI / Jira / Gitlab clients.
- ⏳ Monthly rollup as a background job with status polling (currently synchronous).
