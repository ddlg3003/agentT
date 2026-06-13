import { useState, type FormEvent } from "react";

import { useChat } from "./useChat";

export function ChatPage() {
  const { messages, send, isSending, error } = useChat();
  const [input, setInput] = useState("");

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    send(input);
    setInput("");
  }

  return (
    <div className="mx-auto flex h-screen max-w-2xl flex-col p-4">
      <header className="border-b border-gray-200 pb-3">
        <h1 className="text-xl font-semibold text-gray-900">AgentT</h1>
        <p className="text-sm text-gray-500">MVP agent skeleton · Go + GreenNode AgentBase</p>
      </header>

      <main className="flex-1 space-y-3 overflow-y-auto py-4">
        {messages.length === 0 && (
          <p className="text-center text-sm text-gray-400">Say hello to start the conversation.</p>
        )}
        {messages.map((m, i) => (
          <div key={i} className={m.role === "user" ? "text-right" : "text-left"}>
            <span
              className={
                "inline-block max-w-[80%] rounded-2xl px-4 py-2 text-sm " +
                (m.role === "user"
                  ? "bg-blue-600 text-white"
                  : "bg-gray-100 text-gray-900")
              }
            >
              {m.content}
            </span>
          </div>
        ))}
        {isSending && <p className="text-left text-sm text-gray-400">…thinking</p>}
        {error && <p className="text-left text-sm text-red-500">{error}</p>}
      </main>

      <form onSubmit={onSubmit} className="flex gap-2 border-t border-gray-200 pt-3">
        <input
          value={input}
          onChange={(e) => setInput(e.target.value)}
          placeholder="Type a message…"
          className="flex-1 rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none"
        />
        <button
          type="submit"
          disabled={isSending || !input.trim()}
          className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white disabled:opacity-40"
        >
          Send
        </button>
      </form>
    </div>
  );
}
