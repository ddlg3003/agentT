# AgentT

MVP skeleton for an AI agent: a **Go backend** (clean architecture) and a
**Vite + React + TypeScript frontend**, in one monorepo.

The backend uses **GreenNode AgentBase** as deploy/memory infra for the demo,
but the vendor is kept behind a domain port — the business logic has zero
vendor coupling, so we can later self-host memory or move off AgentBase by
swapping a single adapter.

## Layout

```
agentT/
├── backend/                       # Go module (github.com/vngcloud/agentt)
│   ├── cmd/server/                # composition root (wiring + graceful shutdown)
│   ├── internal/
│   │   ├── domain/                # entities + PORTS (no deps): agent, memory
│   │   ├── usecase/               # application logic (ChatService)
│   │   ├── adapter/http/          # delivery: chi router, handlers, runtime mount
│   │   ├── infra/                 # adapters: greennode, llm (echo), memstore
│   │   └── config/                # env-based config
│   └── pkg/greennode/             # standalone, vendor-isolated AgentBase SDK
│       ├── (config, credentials, auth, restclient)
│       ├── memory/                # Memory API client + models
│       └── runtime/               # AgentBase /invocations + /health contract
├── frontend/                      # Vite + React + TS + Tailwind v4
│   └── src/
│       ├── lib/api.ts             # typed backend client
│       └── features/chat/         # chat UI (ChatPage, useChat)
├── greennode_agentbase-1.0.3/     # upstream Python SDK (reference only)
├── docker-compose.yml             # demo stack
└── Makefile
```

### Architecture (dependency rule)

Dependencies point inward only:

```
adapter/http ─┐
              ├─▶ usecase ─▶ domain (agent.LLMClient, memory.Repository)
infra/* ──────┘                       ▲
   │ implement the ports ─────────────┘
```

`internal/domain` imports nothing external. `internal/infra/greennode` is the
**only** package that imports the AgentBase SDK; it implements
`memory.Repository`. To drop the vendor, reimplement that port (e.g. Postgres +
pgvector) — nothing else changes.

## What was ported from the Python SDK

Only what the project needs (no CLI / MCP / deploy / identity tooling):

| Python (`greennode_agentbase`)        | Go (`pkg/greennode`)              |
| ------------------------------------- | --------------------------------- |
| `core/config.py` (env → file)         | `config.go`                       |
| `identity/credentials.py`             | `credentials.go`                  |
| OAuth2 client_credentials + caching   | `auth.go` (`TokenSource`)         |
| `core/http_client.py`                 | `restclient.go`                   |
| `memory/client.py` + `models.py`      | `memory/client.go` + `models.go`  |
| `runtime/context.py`, `runtime/app.py`| `runtime/context.go` + `server.go`|

## Run locally

Prereqs: Go 1.23+, Node 22+, pnpm (`corepack enable`).

```bash
# Terminal 1 — backend (defaults to in-process memory, no creds needed)
make be-run                       # http://localhost:8080

# Terminal 2 — frontend
make fe-install
make fe-dev                       # http://localhost:5173
```

Smoke test the backend directly:

```bash
curl -s localhost:8080/healthz
curl -s -X POST localhost:8080/api/v1/chat \
  -H 'Content-Type: application/json' \
  -d '{"message":"hello","userId":"u1","sessionId":"s1"}'
```

### Use GreenNode AgentBase memory

Set credentials (env or copy `.greennode.json.example` → `.greennode.json`) and:

```bash
export MEMORY_BACKEND=greennode
export GREENNODE_MEMORY_ID=<your-memory-id>
export GREENNODE_CLIENT_ID=...
export GREENNODE_CLIENT_SECRET=...
make be-run
```

### Demo stack (Docker)

```bash
make up      # frontend → http://localhost:8081, backend → :8080
```

## Deploying on AgentBase

The backend already serves the AgentBase runtime contract (`POST /invocations`,
`GET /health`) from the same binary, so the `backend/Dockerfile` image is
deployable as-is. The chat use case is reused for both the REST API and the
runtime entrypoint.

## Choosing the LLM provider

The LLM sits behind the `agent.LLMClient` port (`internal/domain/agent`). Two
implementations ship in `internal/infra/llm`:

| Provider    | `LLM_PROVIDER` | Notes                                                        |
| ----------- | -------------- | ------------------------------------------------------------ |
| Echo stub   | `echo`         | Default. No key needed — echoes the last user message.       |
| Claude      | `anthropic`    | Real Claude via the official `anthropic-sdk-go`.             |

It **auto-selects `anthropic`** when `ANTHROPIC_API_KEY` is set; otherwise it
falls back to the echo stub. To use Claude:

```bash
export ANTHROPIC_API_KEY=sk-ant-...
export ANTHROPIC_MODEL=claude-opus-4-8        # optional (this is the default)
make be-run
```

The startup log reports the active provider (`llm provider: anthropic`). Add
another provider by writing a sibling `agent.LLMClient` and a `case` in
`cmd/server/main.go`.

## Status / next steps

- Add tests under `backend/...` (`make be-test`) and auth on the REST API.
- Consider streaming the Claude response through to the frontend (SSE) for a
  typing effect.
