// Thin typed client for the backend REST API. Base URL is empty in dev (the
// Vite proxy forwards /api to the Go server); set VITE_API_BASE_URL for prod.
const BASE_URL = import.meta.env.VITE_API_BASE_URL ?? "";

export interface ChatRequest {
  message: string;
  userId?: string;
  sessionId?: string;
}

export interface ChatResponse {
  reply: string;
}

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
};
