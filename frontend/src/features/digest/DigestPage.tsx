import { useState } from "react";
import { useParams } from "react-router-dom";

import { Card, MarkdownView, Spinner } from "../../components/ui";
import { formatDateTime } from "../../lib/format";
import { useDigest, useFlagDigest } from "./queries";
import { FunnelMetrics } from "./FunnelMetrics";
import { EventsList } from "./EventsList";
import { AuditTrail } from "./AuditTrail";
import { FollowupPanel } from "./FollowupPanel";
import type { Correction } from "../../lib/api";

function FlagButton({ date, flagged }: { date: string; flagged: boolean }) {
  const flag = useFlagDigest(date);
  const [open, setOpen] = useState(false);
  const [note, setNote] = useState("");

  if (!open) {
    return (
      <button
        onClick={() => setOpen(true)}
        className={
          "rounded-lg px-3 py-1.5 text-sm font-medium ring-1 ring-inset transition " +
          (flagged
            ? "bg-amber-50 text-amber-700 ring-amber-600/30"
            : "bg-white text-slate-600 ring-slate-300 hover:bg-slate-50")
        }
      >
        {flagged ? "⚑ Flagged — re-flag" : "⚑ Flag incorrect"}
      </button>
    );
  }
  return (
    <div className="flex items-center gap-2">
      <input
        autoFocus
        value={note}
        onChange={(e) => setNote(e.target.value)}
        placeholder="What's wrong?"
        className="rounded-md border border-slate-300 px-2 py-1.5 text-sm focus:border-blue-500 focus:outline-none"
      />
      <button
        onClick={() =>
          flag.mutate(note, { onSuccess: () => (setOpen(false), setNote("")) })
        }
        disabled={flag.isPending}
        className="rounded-md bg-amber-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-amber-700 disabled:opacity-40"
      >
        Submit
      </button>
      <button
        onClick={() => setOpen(false)}
        className="text-sm text-slate-400 hover:text-slate-600"
      >
        Cancel
      </button>
    </div>
  );
}

function CorrectionsCard({ corrections }: { corrections: Correction[] }) {
  return (
    <Card title="PO corrections" subtitle={`${corrections.length} applied`}>
      <ul className="space-y-3">
        {corrections.map((c, i) => (
          <li key={i} className="rounded-lg border border-slate-100 p-3 text-sm">
            <div className="flex items-center justify-between">
              <span className="font-mono text-xs text-blue-700">{c.field}</span>
              <span className="text-xs text-slate-400">
                {c.by} · {formatDateTime(c.at)}
              </span>
            </div>
            <div className="mt-1.5 grid gap-1 sm:grid-cols-2">
              <p className="rounded bg-rose-50 px-2 py-1 text-rose-700 line-through">
                {c.old_value || "—"}
              </p>
              <p className="rounded bg-emerald-50 px-2 py-1 text-emerald-700">
                {c.new_value || "—"}
              </p>
            </div>
            {c.note && <p className="mt-1.5 text-xs text-slate-500">“{c.note}”</p>}
          </li>
        ))}
      </ul>
    </Card>
  );
}

export function DigestPage() {
  const { date } = useParams<{ date: string }>();
  const { data: digest, isLoading, isError, error } = useDigest(date);

  if (!date) {
    return (
      <Placeholder>Select a digest from the left, or generate a new one.</Placeholder>
    );
  }
  if (isLoading) {
    return (
      <Placeholder>
        <Spinner label={`Loading ${date}…`} />
      </Placeholder>
    );
  }
  if (isError || !digest) {
    return (
      <Placeholder>
        <p className="text-rose-600">
          {error instanceof Error ? error.message : "Failed to load digest."}
        </p>
      </Placeholder>
    );
  }

  return (
    <div className="grid h-full grid-cols-1 gap-5 overflow-hidden xl:grid-cols-[1fr_22rem]">
      <div className="min-h-0 space-y-5 overflow-y-auto pb-6 pr-1">
        <header className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h1 className="text-2xl font-bold text-slate-900">Daily Digest</h1>
            <p className="text-sm text-slate-400">
              {date} · generated {formatDateTime(digest.generated_at)}
            </p>
          </div>
          <FlagButton date={date} flagged={digest.flagged} />
        </header>

        {digest.flagged && (
          <div className="rounded-lg border border-amber-200 bg-amber-50 px-4 py-2.5 text-sm text-amber-800">
            <strong>Flagged by PO.</strong> {digest.flag_note}
          </div>
        )}

        <Card title="What happened & why">
          {digest.reasoning ? (
            <MarkdownView>{digest.reasoning}</MarkdownView>
          ) : (
            <p className="text-sm text-slate-400">No narrative.</p>
          )}
        </Card>

        <FunnelMetrics metrics={digest.metrics} />
        <EventsList events={digest.events} />
        {digest.corrections && digest.corrections.length > 0 && (
          <CorrectionsCard corrections={digest.corrections} />
        )}
        <AuditTrail sources={digest.sources} />
      </div>

      <div className="hidden min-h-0 xl:block">
        <FollowupPanel date={date} />
      </div>
    </div>
  );
}

function Placeholder({ children }: { children: React.ReactNode }) {
  return (
    <div className="grid h-full place-items-center text-center text-sm text-slate-500">
      <div>{children}</div>
    </div>
  );
}
