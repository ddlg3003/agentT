import type { Metric } from "./api";

/** Format a metric value respecting its unit ("%" → 21.0%, "#" → 97,000). */
export function formatValue(m: Pick<Metric, "value" | "unit">): string {
  if (m.unit === "%") return `${m.value.toFixed(1)}%`;
  return Math.round(m.value).toLocaleString("en-US");
}

/** Format a delta in the metric's unit, signed (+/−). Returns "" for ~0. */
export function formatDelta(delta: number, unit: string): string {
  if (Math.abs(delta) < 1e-9) return "0";
  const sign = delta > 0 ? "+" : "−";
  if (unit === "%") return `${sign}${Math.abs(delta).toFixed(1)}pp`;
  return `${sign}${Math.abs(Math.round(delta)).toLocaleString("en-US")}`;
}

export type Trend = "up" | "down" | "flat";

export function trendOf(delta: number): Trend {
  if (delta > 1e-9) return "up";
  if (delta < -1e-9) return "down";
  return "flat";
}

/** A YYYY-MM-DD date → "Fri, 13 Mar 2026". */
export function formatDate(date: string): string {
  const d = new Date(`${date}T00:00:00`);
  if (Number.isNaN(d.getTime())) return date;
  return d.toLocaleDateString("en-GB", {
    weekday: "short",
    day: "2-digit",
    month: "short",
    year: "numeric",
  });
}

/** An ISO timestamp → "13 Mar 2026, 08:30". */
export function formatDateTime(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString("en-GB", {
    day: "2-digit",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}
