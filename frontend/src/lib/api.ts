// Thin typed client for the backend REST API. Base URL is empty in dev (the
// Vite proxy forwards /api to the Go server); set VITE_API_BASE_URL for prod.
const BASE_URL = import.meta.env.VITE_API_BASE_URL ?? "";

// ---------------------------------------------------------------------------
// Domain types — mirror backend/internal/domain/digest and usecase outputs.
// ---------------------------------------------------------------------------

export interface Metric {
  partner: string; // "SHB" | "CAKE" | "TNEX" | "VP" | "ALL"
  step: string; // funnel step code, e.g. "s20s120"
  name: string; // human label, e.g. "E2E Rate"
  unit: string; // "%" | "#"
  value: number;
  delta_day: number; // change vs previous day
  delta_mom: number; // change vs same day last month
}

export interface DigestEvent {
  source: string; // "jira" | "gitlab"
  id: string; // "LOAN-451" | "!1234"
  title: string;
  type: string; // bug | feature | incident | mr
  status: string;
  occurred_at: string;
  linked_tickets?: string[];
}

export interface Source {
  tool_name: string;
  input: unknown;
  output: unknown;
}

export interface Correction {
  field: string;
  old_value: string;
  new_value: string;
  note: string;
  by: string;
  at: string;
}

export interface DailyDigest {
  date: string; // YYYY-MM-DD
  generated_at: string;
  metrics: Metric[];
  events: DigestEvent[];
  reasoning: string;
  sources: Source[];
  flagged: boolean;
  flag_note?: string;
  corrections?: Correction[];
}

export interface MonthlyReport {
  month: string; // "2026-03"
  markdown: string;
  num_digests: number;
}

export interface FollowupTurn {
  role: "user" | "assistant";
  content: string;
}

export interface ChatRequest {
  message: string;
  userId?: string;
  sessionId?: string;
}

export interface ChatResponse {
  reply: string;
}

// ---------------------------------------------------------------------------
// Transport
// ---------------------------------------------------------------------------

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE_URL}${path}`, {
    headers: { "Content-Type": "application/json" },
    ...init,
  });
  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as { error?: string } | null;
    throw new Error(body?.error ?? `request failed: ${res.status}`);
  }
  return res.json() as Promise<T>;
}

export const api = {
  chat: (req: ChatRequest) =>
    request<ChatResponse>("/api/v1/chat", {
      method: "POST",
      body: JSON.stringify(req),
    }),

  // Digest product -----------------------------------------------------------
  listDigests: () =>
    request<{ dates: string[] }>("/api/v1/digests").then((r) => r.dates ?? []),

  getDigest: (date: string) => request<DailyDigest>(`/api/v1/digests/${date}`),

  getFollowupHistory: (date: string, userId = "demo-po") =>
    request<{ turns: FollowupTurn[] }>(`/api/v1/digests/${date}/history?userId=${encodeURIComponent(userId)}`).then(
      (r) => r.turns ?? [],
    ),

  askFollowup: (date: string, question: string, userId = "demo-po") =>
    request<{ answer: string }>(`/api/v1/digests/${date}/ask`, {
      method: "POST",
      body: JSON.stringify({ question, userId }),
    }),

  flagDigest: (date: string, note: string, userId = "demo-po") =>
    request<DailyDigest>(`/api/v1/digests/${date}/flag`, {
      method: "PATCH",
      body: JSON.stringify({ note, userId }),
    }),

  runDaily: (date: string) =>
    request<DailyDigest>("/api/v1/jobs/daily", {
      method: "POST",
      body: JSON.stringify({ date }),
    }),

  monthlyReport: (ym: string) =>
    request<MonthlyReport>(`/api/v1/report/monthly/${ym}`),
};
