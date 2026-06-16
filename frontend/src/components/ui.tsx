import { useEffect, useState, type ReactNode } from "react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";

import { formatDelta, trendOf, type Trend } from "../lib/format";

/** A small pill showing a signed delta, coloured by trend. */
export function DeltaPill({
  delta,
  unit,
  label,
}: {
  delta: number;
  unit: string;
  label?: string;
}) {
  const trend = trendOf(delta);
  const cls: Record<Trend, string> = {
    up: "bg-emerald-50 text-emerald-700 ring-emerald-600/20",
    down: "bg-rose-50 text-rose-700 ring-rose-600/20",
    flat: "bg-slate-100 text-slate-500 ring-slate-400/20",
  };
  const arrow: Record<Trend, string> = { up: "▲", down: "▼", flat: "■" };
  return (
    <span
      className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium ring-1 ring-inset ${cls[trend]}`}
      title={label}
    >
      <span className="text-[0.6rem] leading-none">{arrow[trend]}</span>
      {formatDelta(delta, unit)}
      {label && <span className="font-normal opacity-70">{label}</span>}
    </span>
  );
}

const BADGE_TONES: Record<string, string> = {
  jira: "bg-indigo-50 text-indigo-700 ring-indigo-600/20",
  gitlab: "bg-orange-50 text-orange-700 ring-orange-600/20",
  bug: "bg-rose-50 text-rose-700 ring-rose-600/20",
  incident: "bg-rose-50 text-rose-700 ring-rose-600/20",
  feature: "bg-blue-50 text-blue-700 ring-blue-600/20",
  mr: "bg-violet-50 text-violet-700 ring-violet-600/20",
  default: "bg-slate-100 text-slate-600 ring-slate-400/20",
};

export function Badge({ children, tone }: { children: ReactNode; tone?: string }) {
  const cls = (tone && BADGE_TONES[tone]) ?? BADGE_TONES.default;
  return (
    <span
      className={`inline-flex items-center rounded-md px-1.5 py-0.5 text-xs font-medium ring-1 ring-inset ${cls}`}
    >
      {children}
    </span>
  );
}

const QUIPS = [
  "Querying BI data…",
  "Tracing the conversion funnel…",
  "Cross-checking Jira tickets…",
  "Reading through Gitlab MRs…",
  "Crunching per-partner metrics…",
  "Thinking really hard right now…",
  "Almost done writing the narrative…",
  "Consulting the knowledge base…",
  "Just a little longer, promise…",
  "Analyzing month-over-month trends…",
  "Hmm, this number looks off…",
  "Synthesizing insights…",
  "Asking the agent one more thing…",
  "Double-checking the data…",
  "Connecting the dots…",
  "Making sure nothing is missing…",
];

export function Spinner({ label, timed }: { label?: string; timed?: boolean }) {
  const [secs, setSecs] = useState(0);
  const [startIdx] = useState(() => Math.floor(Math.random() * QUIPS.length));

  useEffect(() => {
    if (!timed) return;
    const t = setInterval(() => setSecs((s) => s + 1), 1000);
    return () => clearInterval(t);
  }, [timed]);

  const quip = QUIPS[(startIdx + Math.floor(secs / 4)) % QUIPS.length];

  return (
    <div className="flex items-center gap-2 text-sm text-slate-500">
      <span className="h-4 w-4 animate-spin rounded-full border-2 border-blue-500 border-t-transparent" />
      {timed ? (
        <span>
          {quip}
          <span className="ml-1.5 font-mono text-xs tabular-nums text-slate-400">{secs}s</span>
        </span>
      ) : (
        <span>{label ?? "Loading…"}</span>
      )}
    </div>
  );
}

export function Card({
  title,
  subtitle,
  action,
  children,
}: {
  title?: ReactNode;
  subtitle?: ReactNode;
  action?: ReactNode;
  children: ReactNode;
}) {
  return (
    <section className="rounded-xl border border-slate-200 bg-white shadow-sm">
      {(title || action) && (
        <header className="flex items-center justify-between border-b border-slate-100 px-5 py-3">
          <div>
            {title && <h2 className="text-sm font-semibold text-slate-800">{title}</h2>}
            {subtitle && <p className="text-xs text-slate-400">{subtitle}</p>}
          </div>
          {action}
        </header>
      )}
      <div className="px-5 py-4">{children}</div>
    </section>
  );
}

function stripThinkBlocks(text: string): string {
  return text.replace(/<think>[\s\S]*?<\/think>/gi, "").trim();
}

/** Markdown renderer with prose styling tuned for the blue theme. */
export function MarkdownView({ children }: { children: string }) {
  return (
    <div className="prose-blue max-w-none text-sm leading-relaxed text-slate-700">
      <Markdown remarkPlugins={[remarkGfm]}>{stripThinkBlocks(children)}</Markdown>
    </div>
  );
}
