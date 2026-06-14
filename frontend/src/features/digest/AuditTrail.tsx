import { useState } from "react";

import type { Source } from "../../lib/api";
import { Badge, Card } from "../../components/ui";

function pretty(v: unknown): string {
  if (v == null) return "";
  if (typeof v === "string") return v;
  try {
    return JSON.stringify(v, null, 2);
  } catch {
    return String(v);
  }
}

/** The automatic audit trail: every tool call the agent loop made to build the
 *  digest, so each number is traceable to its origin. Collapsed by default. */
export function AuditTrail({ sources }: { sources: Source[] }) {
  const [open, setOpen] = useState<number | null>(null);

  return (
    <Card title="Audit trail" subtitle={`${sources.length} tool calls`}>
      {sources.length === 0 ? (
        <p className="text-sm text-slate-400">No tool calls recorded.</p>
      ) : (
        <ul className="space-y-1.5">
          {sources.map((s, i) => (
            <li key={i} className="rounded-lg border border-slate-100">
              <button
                onClick={() => setOpen(open === i ? null : i)}
                className="flex w-full items-center justify-between gap-2 px-3 py-2 text-left text-sm hover:bg-slate-50"
              >
                <span className="flex items-center gap-2">
                  <span className="font-mono text-xs text-slate-400">#{i + 1}</span>
                  <Badge>{s.tool_name}</Badge>
                </span>
                <span className="text-xs text-slate-400">{open === i ? "Hide" : "Inspect"}</span>
              </button>
              {open === i && (
                <div className="grid gap-2 border-t border-slate-100 px-3 py-2 sm:grid-cols-2">
                  <div>
                    <p className="mb-1 text-xs font-medium text-slate-500">input</p>
                    <pre className="max-h-48 overflow-auto rounded-md bg-slate-50 p-2 text-xs text-slate-600">
                      {pretty(s.input)}
                    </pre>
                  </div>
                  <div>
                    <p className="mb-1 text-xs font-medium text-slate-500">output</p>
                    <pre className="max-h-48 overflow-auto rounded-md bg-slate-50 p-2 text-xs text-slate-600">
                      {pretty(s.output)}
                    </pre>
                  </div>
                </div>
              )}
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}
