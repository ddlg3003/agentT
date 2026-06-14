import { useState, type FormEvent } from "react";

import { MarkdownView, Spinner } from "../../components/ui";
import { useAskFollowup } from "./queries";

interface Turn {
  role: "po" | "agent";
  text: string;
}

const SUGGESTIONS = [
  "Why did SHB's E2E rate move?",
  "Which event best explains the biggest drop?",
  "Summarise the day in two sentences.",
];

/** Conversational follow-up grounded in the selected digest. The agent may
 *  apply a PO correction via update_digest; queries invalidate on success. */
export function FollowupPanel({ date }: { date: string }) {
  const [turns, setTurns] = useState<Turn[]>([]);
  const [input, setInput] = useState("");
  const ask = useAskFollowup(date);

  function submit(question: string) {
    const q = question.trim();
    if (!q || ask.isPending) return;
    setTurns((t) => [...t, { role: "po", text: q }]);
    setInput("");
    ask.mutate(q, {
      onSuccess: (res) =>
        setTurns((t) => [...t, { role: "agent", text: res.answer }]),
      onError: (err) =>
        setTurns((t) => [
          ...t,
          { role: "agent", text: `⚠️ ${err instanceof Error ? err.message : "request failed"}` },
        ]),
    });
  }

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    submit(input);
  }

  return (
    <div className="flex h-full flex-col rounded-xl border border-slate-200 bg-white shadow-sm">
      <header className="border-b border-slate-100 px-4 py-3">
        <h2 className="text-sm font-semibold text-slate-800">Ask the agent</h2>
        <p className="text-xs text-slate-400">Grounded in this digest · can apply PO corrections</p>
      </header>

      <div className="flex-1 space-y-3 overflow-y-auto px-4 py-4">
        {turns.length === 0 && (
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
          t.role === "po" ? (
            <div key={i} className="text-right">
              <span className="inline-block max-w-[85%] rounded-2xl rounded-br-sm bg-blue-600 px-3.5 py-2 text-sm text-white">
                {t.text}
              </span>
            </div>
          ) : (
            <div key={i} className="rounded-2xl rounded-bl-sm bg-slate-50 px-3.5 py-2">
              <MarkdownView>{t.text}</MarkdownView>
            </div>
          ),
        )}
        {ask.isPending && <Spinner label="Thinking…" />}
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
