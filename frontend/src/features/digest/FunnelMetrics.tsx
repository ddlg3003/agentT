import { useMemo, useState } from "react";

import type { Metric } from "../../lib/api";
import { formatValue } from "../../lib/format";
import { Card, DeltaPill } from "../../components/ui";

/** Funnel viz: pick a partner, see each funnel step's value and how it moved
 *  day-over-day and month-over-month. The bar width encodes the value for %
 *  steps so the conversion funnel reads at a glance. */
export function FunnelMetrics({ metrics }: { metrics: Metric[] }) {
  const partners = useMemo(() => {
    const seen: string[] = [];
    for (const m of metrics) if (!seen.includes(m.partner)) seen.push(m.partner);
    return seen;
  }, [metrics]);

  const [partner, setPartner] = useState(partners[0] ?? "");
  const active = partners.includes(partner) ? partner : partners[0] ?? "";
  const rows = metrics.filter((m) => m.partner === active);

  if (metrics.length === 0) {
    return (
      <Card title="Conversion funnel">
        <p className="text-sm text-slate-400">No metrics in this digest.</p>
      </Card>
    );
  }

  return (
    <Card
      title="Conversion funnel"
      subtitle={`${rows.length} steps · ${active}`}
      action={
        <div className="flex flex-wrap gap-1">
          {partners.map((p) => (
            <button
              key={p}
              onClick={() => setPartner(p)}
              className={
                "rounded-md px-2.5 py-1 text-xs font-medium transition " +
                (p === active
                  ? "bg-blue-600 text-white shadow-sm"
                  : "bg-slate-100 text-slate-600 hover:bg-slate-200")
              }
            >
              {p}
            </button>
          ))}
        </div>
      }
    >
      <ul className="space-y-2.5">
        {rows.map((m) => (
          <li key={`${m.partner}-${m.step}`}>
            <div className="flex items-baseline justify-between gap-3">
              <span className="text-sm text-slate-600">{m.name}</span>
              <div className="flex items-center gap-1.5">
                <span className="tabular-nums text-sm font-semibold text-slate-900">
                  {formatValue(m)}
                </span>
                <DeltaPill delta={m.delta_day} unit={m.unit} label="d/d" />
                <DeltaPill delta={m.delta_mom} unit={m.unit} label="MoM" />
              </div>
            </div>
            {m.unit === "%" && (
              <div className="mt-1 h-1.5 w-full overflow-hidden rounded-full bg-slate-100">
                <div
                  className="h-full rounded-full bg-gradient-to-r from-blue-400 to-blue-600"
                  style={{ width: `${Math.min(100, Math.max(2, m.value * 100))}%` }}
                />
              </div>
            )}
          </li>
        ))}
      </ul>
    </Card>
  );
}
