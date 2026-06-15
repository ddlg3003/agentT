import { useEffect, useRef, useState, type FormEvent } from "react";

import { MarkdownView, Spinner } from "../../components/ui";
import { useAskFollowup, useFollowupHistory } from "./queries";

const SUGGESTIONS = [
  "Why did SHB's E2E rate move?",
  "Which event best explains the biggest drop?",
  "Summarise the day in two sentences.",
];

/** Conversational follow-up grounded in the selected digest. The agent may
 *  apply a PO correction via update_digest; queries invalidate on success. */
export function FollowupPanel({ date }: { date: string }) {
  const [input, setInput] = useState("");
  const [optimistic, setOptimistic] = useState<{ question: string; answer: string | null } | null>(null);
  const bottomRef = useRef<HTMLDivElement>(null);

  const history = useFollowupHistory(date);
  const ask = useAskFollowup(date);

  // Auto-scroll to bottom when history loads or a new turn is added.
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [history.data?.length, optimistic]);

  function submit(question: string) {
    const q = question.trim();
    if (!q || ask.isPending) return;
    setInput("");
    setOptimistic({ question: q, answer: null });
    ask.mutate(q, {
      onSuccess: () => setOptimistic(null),
      onError: (err) => {
        setOptimistic({
          question: q,
          answer: `⚠️ ${err instanceof Error ? err.message : "request failed"}`,
        });
      },
    });
  }

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    submit(input);
  }

  const turns = history.data ?? [];
  const isEmpty = turns.length === 0 && !optimistic && !history.isLoading;

  return (
    <div className="flex h-full flex-col rounded-xl border border-slate-200 bg-white shadow-sm">
      <header className="border-b border-slate-100 px-4 py-3">
        <h2 className="text-sm font-semibold text-slate-800">Ask the agent</h2>
        <p className="text-xs text-slate-400">Grounded in this digest · can apply PO corrections</p>
      </header>

      <div className="flex-1 space-y-3 overflow-y-auto px-4 py-4">
        {history.isLoading && <Spinner label="Loading history…" />}

        {isEmpty && (
          <div className="space-y-2">
            <p className="text-sm text-slate-400">Ask why a number moved, or request a correction.</p>
            <div className="flex flex-col gap-1.5">
              {SUGGESTIONS.map((s) => (
                <button
                  key={s}
                  onClick={() => submit(s)}
                  className="rounded-lg border border-blue-100 bg-blue-50/50 px-3 py-1.5 text-left text-xs text-blue-700 hover:bg-blue-50"
                >
                  {s}
                </button>
              ))}
            </div>
          </div>
        )}

        {turns.map((t, i) =>
          t.role === "user" ? (
            <div key={i} className="text-right">
              <span className="inline-block max-w-[85%] rounded-2xl rounded-br-sm bg-blue-600 px-3.5 py-2 text-sm text-white">
                {t.content}
              </span>
            </div>
          ) : (
            <div key={i} className="rounded-2xl rounded-bl-sm bg-slate-50 px-3.5 py-2">
              <MarkdownView>{t.content}</MarkdownView>
            </div>
          ),
        )}

        {/* Optimistic question bubble while waiting for answer */}
        {optimistic && (
          <>
            <div className="text-right">
              <span className="inline-block max-w-[85%] rounded-2xl rounded-br-sm bg-blue-600 px-3.5 py-2 text-sm text-white">
                {optimistic.question}
              </span>
            </div>
            {optimistic.answer === null ? (
              <Spinner label="Thinking…" />
            ) : (
              <div className="rounded-2xl rounded-bl-sm bg-slate-50 px-3.5 py-2">
                <MarkdownView>{optimistic.answer}</MarkdownView>
              </div>
            )}
          </>
        )}

        <div ref={bottomRef} />
      </div>

      <form onSubmit={onSubmit} className="flex gap-2 border-t border-slate-100 p-3">
        <input
          value={input}
          onChange={(e) => setInput(e.target.value)}
          placeholder="Ask a follow-up…"
          className="flex-1 rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
        />
        <button
          type="submit"
          disabled={ask.isPending || !input.trim()}
          className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white transition hover:bg-blue-700 disabled:opacity-40"
        >
          Send
        </button>
      </form>
    </div>
  );
}
