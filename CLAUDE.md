# agentT — Claude Agent Guide

## What this is

A Sales/PO Intelligence agent for the Cash Loan product. It runs daily agentic
loops over (mocked) BI / Jira / Gitlab / knowledge sources to produce a structured
**Daily Digest** explaining how the per-partner conversion funnel moved and why,
persists digests, supports follow-up Q&A (with PO corrections), and synthesizes a
**monthly month-over-month report**. All external data sources are mocked today;
swap the tool `Run` implementations for real clients later — the agent loop and
prompts stay identical.

## Project layout

```
backend/   Go 1.24, hexagonal architecture
  cmd/server/main.go          HTTP entry point (composition via internal/app)
  internal/app/               composition root — Build() wires everything
  internal/config/            env-based config (12-factor), reads .env via godotenv
  internal/domain/agent/      LLMClient + ToolCaller ports; Message/Tool/ToolCall types
  internal/domain/digest/     DailyDigest/Metric/Event/Source/Correction + Repository port
  internal/domain/memory/     Repository port (conversation + follow-up Q&A turns)
  internal/usecase/           core logic — no framework deps
    loop.go                   reusable think→act→observe engine (the agent loop)
    digest.go                 GenerateDaily (read-only tools)
    followup.go               AskFollowup  (read tools + update_digest)
    monthly.go                GenerateMonthly (monthly rollup)
    prompts/*.md              embedded system prompts (daily/followup/monthly)
  internal/adapter/http/      HTTP delivery (chi router): chat + digest handlers
  internal/infra/llm/         LLM providers: echo (stub), anthropic, openai
  internal/infra/tools/       agent tools: query_bi/jira/gitlab, get_knowledge, update_digest
  internal/infra/digeststore/ SQLite digest.Repository (modernc.org/sqlite, no CGO)
  internal/infra/memstore/    in-process memory store
  internal/infra/greennode/   AgentBase memory store
  internal/scheduler/         daily job scheduler (fixed local time, hardcoded)
  mock/                       mock data: bi/, jira/, gitlab/, knowledge/
  pkg/greennode/              VNG Cloud AgentBase SDK

frontend/  React 18 + Vite + TypeScript (chat UI; digest UI TBD)
```

## The agent loop (internal/usecase/loop.go)

One engine, three invocations. The only things that differ between the daily,
follow-up, and monthly loops are the system prompt, the seed message, and the tool
set. Design rules baked into the loop (see the agentic-harness skill):

- **Stop on content, not metadata.** The loop stops when the model returns a message
  with no tool calls — never on the provider's `stop_reason`.
- **Sequential tool dispatch.** No goroutines for tool calls in the MVP.
- **Hard turn limit.** `MAX_TURNS` (10 daily / 15 monthly) → returns `ErrMaxTurnsExceeded`,
  never a silent partial digest.
- **Failures don't kill the run.** A missing/failing tool returns a
  `[DATA UNAVAILABLE: …]` result to the model; the loop continues.
- **Audit trail is automatic.** Every successful tool call is recorded in
  `DailyDigest.Sources` by the loop, so every number is traceable without trusting
  the model to self-report.

## Tools & permission boundary

Read-only data tools (`query_bi`, `query_jira`, `query_gitlab`, `get_knowledge`) are
shared by all loops. The **only** write tool, `update_digest`, mutates the agent's own
digest store (never external sources) and is wired in **only** for the follow-up loop —
constrained by code, not instruction. It is built per request via a factory bound to a
specific digest (`date`) and actor (`userID`) so the model can never correct a different
digest or misattribute a change. Corrections preserve the original value in
`DailyDigest.Corrections`.

## Architecture rules

- **Dependency direction:** domain ← usecase ← adapter/infra. Never import infra from
  domain or usecase. (The write-tool factory is injected as a `func` so usecase depends
  only on the `agent.Tool` port.)
- **New LLM provider:** add a file in `internal/infra/llm/`, implement both `Complete`
  and `CompleteWithTools` (the `agent.ToolCaller` port), add `XxxConfig`/`XxxOptions`,
  wire in `internal/app` + `config.go`, document in `backend/.env.example`.
- **New tool:** implement `agent.Tool` in `internal/infra/tools/`, register it in the
  read-only set in `internal/app`. Read-only by default.
- **No fallback on error:** errors propagate; don't silently swallow and return partials.
- All errors in `usecase/` are logged with `slog` before returning (`log.ErrorContext`).

## API (frontend-facing, /api/v1)

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/digests` | list dates that have a digest |
| GET | `/digests/{date}` | full DailyDigest |
| POST | `/digests/{date}/ask` | follow-up Q&A (may apply a PO correction) |
| PATCH | `/digests/{date}/flag` | PO flags a digest incorrect |
| POST | `/jobs/daily` | trigger a daily run (PO "create digest" button) |
| POST | `/jobs/monthly` | run the monthly rollup |
| GET | `/report/monthly/{ym}` | monthly report (ym = `2026-03`) |

The daily job also runs automatically once per day at a hardcoded local time
(`internal/scheduler`); the PO can trigger an on-demand run via `POST /jobs/daily`.

## Running locally

```bash
make be-run      # backend — reads backend/.env automatically
make fe-dev      # frontend — proxies /api → backend
```

The agent loops need a tool-calling model: set `ANTHROPIC_API_KEY` or `OPENAI_API_KEY`.
The `echo` stub keeps the server runnable but cannot produce real digests.

## Key env vars

| Var | Purpose |
|-----|---------|
| `PORT` | backend port (default `8080`) |
| `LLM_PROVIDER` | `anthropic` / `openai` / `echo` (auto-detected from API key) |
| `ANTHROPIC_API_KEY` / `ANTHROPIC_BASE_URL` / `ANTHROPIC_MODEL` | Claude provider |
| `OPENAI_API_KEY` / `OPENAI_BASE_URL` / `OPENAI_MODEL` | OpenAI-compatible provider |
| `MOCK_DIR` | base dir for mock data tools (default `./mock`) |
| `DIGEST_DB_PATH` | SQLite digest store path (default `./digests.db`) |

See `backend/.env.example` for the full list.

## Makefile targets

`be-run`, `be-build`, `be-test`, `be-tidy`, `be-vet`, `fe-dev`, `fe-build`, `fe-lint`
