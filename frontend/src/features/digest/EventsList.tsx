import type { DigestEvent } from "../../lib/api";
import { Badge, Card } from "../../components/ui";
import { formatDate } from "../../lib/format";

/** Tickets and merge requests in the digest window that may explain movements. */
export function EventsList({ events }: { events: DigestEvent[] }) {
  return (
    <Card title="Events in window" subtitle={`${events.length} tickets / MRs`}>
      {events.length === 0 ? (
        <p className="text-sm text-slate-400">No events linked to this digest.</p>
      ) : (
        <ul className="divide-y divide-slate-100">
          {events.map((e) => (
            <li key={`${e.source}-${e.id}`} className="flex items-start gap-3 py-2.5">
              <div className="mt-0.5 flex shrink-0 gap-1">
                <Badge tone={e.source}>{e.source}</Badge>
              </div>
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="font-mono text-xs font-semibold text-blue-700">
                    {e.id}
                  </span>
                  <Badge tone={e.type}>{e.type}</Badge>
                  <span className="text-xs text-slate-400">{e.status}</span>
                </div>
                <p className="mt-0.5 text-sm text-slate-700">{e.title}</p>
                <div className="mt-0.5 flex flex-wrap items-center gap-2 text-xs text-slate-400">
                  <span>{formatDate(e.occurred_at)}</span>
                  {e.linked_tickets && e.linked_tickets.length > 0 && (
                    <span>· linked: {e.linked_tickets.join(", ")}</span>
                  )}
                </div>
              </div>
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}
