import { useState } from "react";
import { useMutation } from "@tanstack/react-query";

import { api } from "../../lib/api";

export interface ChatMessage {
  role: "user" | "assistant";
  content: string;
}

// A stable session id for this browser tab; good enough for the MVP demo.
const sessionId = crypto.randomUUID();

/**
 * useChat owns the local transcript and the send mutation. Keeping transport in
 * TanStack Query gives us loading/error state for free.
 */
export function useChat(userId = "demo-user") {
  const [messages, setMessages] = useState<ChatMessage[]>([]);

  const mutation = useMutation({
    mutationFn: (message: string) => api.chat({ message, userId, sessionId }),
    onSuccess: (res) => {
      setMessages((prev) => [...prev, { role: "assistant", content: res.reply }]);
    },
  });

  function send(message: string) {
    const trimmed = message.trim();
    if (!trimmed) return;
    setMessages((prev) => [...prev, { role: "user", content: trimmed }]);
    mutation.mutate(trimmed);
  }

  return {
    messages,
    send,
    isSending: mutation.isPending,
    error: mutation.error instanceof Error ? mutation.error.message : null,
  };
}
