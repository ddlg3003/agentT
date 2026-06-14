---
name: agentic-harness
description: >
  High-level design principles and mindset for building agentic systems — any system
  where an AI agent loops, acts, and coordinates. Use this skill when the user is
  designing, reviewing, or troubleshooting an agentic system of any kind: data analysis
  agents, coding agents, monitoring agents, workflow automation, multi-agent pipelines,
  or anything involving an LLM that calls tools and decides what to do next. Also trigger
  when the user asks about: agent loops, tool orchestration, context management, agent
  coordination, permission design, or why their agent spins / crashes / does too much itself.
---

# Agentic Harness

A design mindset for agentic systems, distilled from 18 production-validated patterns.
Domain-agnostic: the same principles apply whether you're building a data analysis agent,
a coding assistant, a monitoring bot, or a multi-robot coordinator.

---

## The Core Insight

An agentic system is not a chatbot with tools bolted on. It's a **loop with a brain**.

```
while task not done:
    think → act → observe → think again
```

Everything else — concurrency, memory, permissions, multi-agent — is infrastructure
that keeps this loop alive, safe, and fast. Design the loop first. Add infrastructure
as problems emerge.

---

## 6 Layers of an Agentic OS

Think of any agentic system as having 6 nested concerns, from outermost to innermost:

```
┌─────────────────────────────────┐
│  1. Interface (UI / API / CLI)  │  ← how humans observe and intervene
│  ┌───────────────────────────┐  │
│  │  2. Query Loop (the heart)│  │  ← think → act → loop
│  │  ┌─────────────────────┐  │  │
│  │  │  3. Tool Execution  │  │  │  ← what the agent actually does
│  │  │  ┌───────────────┐  │  │  │
│  │  │  │  4. Agents    │  │  │  │  ← spawning other agents
│  │  │  │  ┌─────────┐  │  │  │  │
│  │  │  │  │ 5. Mem  │  │  │  │  │  ← keeping context alive
│  │  │  │  │ ┌─────┐ │  │  │  │  │
│  │  │  │  │ │6.Perm│ │  │  │  │  │  ← controlling what's allowed
│  │  │  │  │ └─────┘ │  │  │  │  │
│  │  │  │  └─────────┘  │  │  │  │
│  │  │  └───────────────┘  │  │  │
│  │  └─────────────────────┘  │  │
│  └───────────────────────────┘  │
└─────────────────────────────────┘
```

You don't need all 6 layers on day one. Start with 2 (the loop) and add layers only
when you hit actual problems.

---

## The 5 Design Principles

### 1. Observe content, not metadata

The loop should continue or stop based on **what the agent actually did**, not on
status codes or API signals. API metadata can lie; content cannot.

> If the agent called a tool → keep looping.
> If the agent only produced text → stop.

Derive your own continuation flag from the response content. Never trust `stop_reason`
or equivalent signals from the LLM provider directly.

---

### 2. Default to safe, escalate deliberately

Every decision in an agentic system should have a safe default:

- Tools are **exclusive** (serial) by default — only opt into concurrency when you're
  sure it's safe for that specific invocation
- Permissions **deny** by default — explicitly allow, never implicitly permit
- Recovery **retries conservatively** — same strategy at most a few times, then
  escalate to a different strategy, then surface to human

The pattern is always: cheap/safe first → escalate only if needed → human as last resort.
Each escalation level runs **at most once** — never retry the same failed strategy.

---

### 3. Constrain by code, not by instruction

If you don't want an agent to do something, **remove the tool**, don't just tell it not to.

LLMs take the shortest path. A coordinator agent told "don't write code yourself,
delegate to workers" will still write code when it seems faster. A coordinator agent
with no execution tools *cannot* write code — it is forced to delegate.

> Instruction = hint. Tool restriction = guarantee.

This applies everywhere: read-only agents get no write tools. Batch jobs get no
interactive-approval tools. Planning agents get no execution tools.

---

### 4. Memory is layered, forgetting is controlled

Context windows are finite. Long-running agents will fill them. Design for this upfront:

```
Result too large?      → truncate, persist to disk, keep a pointer
Session getting long?  → summarize old turns, keep a boundary marker
Hit the limit mid-run? → emergency summarize, retry
```

The key insight: **forgetting should be intentional**. A "compact boundary" — a marker
in conversation history — tells the system "drop everything before this, only keep the
summary." The agent forgets the conversation but retains the lessons.

Memory across sessions works the same way: extract key facts at session end, inject
relevant ones at session start. The agent doesn't remember everything — it remembers
what matters.

---

### 5. Humans stay in the loop at the architecture level

Human oversight is not a UI feature — it's a structural constraint. Design it in:

- **Coordinator agents** cannot execute; they can only delegate. Humans approve at
  the coordinator level.
- **Permissions bubble up**: subagents cannot self-approve dangerous actions.
  Approvals flow to the surface where a human can see them.
- **Background tasks** report via structured output that humans can read and act on.
- **Cost and turn limits** are hard stops, not soft warnings.

The autonomy dial goes from "ask every time" to "never ask." Neither extreme is right.
Set the dial per action type based on reversibility and risk — not globally.

---

## Pattern → Problem Mapping

When something goes wrong, trace it to the right layer:

| Symptom | Layer | Fix |
|---------|-------|-----|
| Agent loops forever / never stops | Query Loop | Check continuation flag derivation (#2) |
| Agent crashes on long sessions | Context | Add compaction layers (#9) |
| Tool calls are slow | Tool Execution | Partition safe tools to run in parallel (#4) |
| Orchestrator does everything itself | Multi-Agent | Remove execution tools from coordinator (#7) |
| Parallel agents corrupt shared state | Multi-Agent | Isolate workspaces, merge after (#8) |
| Errors cause infinite retries | Query Loop | Escalating recovery with circuit breaker (#3) |
| Users approve everything blindly | Permission | Allowlist safe tools, track denial patterns (#10) |
| Same instructions repeated constantly | Extension | Promote to a reusable skill, auto-activate by context (#11–13) |
| Agent "forgets" early context | Context | Compact boundary + session memory (#9) |

---

## What to Design First

When starting any agentic system, answer these in order:

1. **What is the loop condition?** When does the agent stop? (Derive from content, not metadata)
2. **What tools exist, and which are safe to parallelize?** (Default: none. Opt in explicitly)
3. **How long can a session run?** (Design compaction strategy before you need it)
4. **Who approves what?** (Map actions to risk levels before writing permission code)
5. **Do you need multiple agents?** (Only if tasks are truly independent and parallel — otherwise one agent is simpler)

---

## What NOT to Do

- Don't add multi-agent complexity until a single agent is too slow or too limited
- Don't trust API stop signals — observe what the agent actually produced
- Don't retry the same failed strategy — escalate to a different one
- Don't give a coordinator execution tools — it will use them
- Don't treat context limits as edge cases — plan for them from day one
- Don't make permission binary — classify by risk, not allow/deny