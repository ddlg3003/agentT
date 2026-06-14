import { useState } from "react";
import { NavLink, useNavigate } from "react-router-dom";

import { formatDate } from "../../lib/format";
import { Spinner } from "../../components/ui";
import { useDigestDates, useRunDaily } from "./queries";

export function Sidebar() {
  const navigate = useNavigate();
  const { data: dates, isLoading } = useDigestDates();
  const runDaily = useRunDaily();
  const [date, setDate] = useState("");

  function generate() {
    if (!date) return;
    runDaily.mutate(date, { onSuccess: (d) => navigate(`/digests/${d.date}`) });
  }

  const linkBase =
    "block rounded-lg px-3 py-2 text-sm transition";
  const active = "bg-blue-600 text-white shadow-sm";
  const idle = "text-slate-600 hover:bg-blue-50 hover:text-blue-700";

  return (
    <aside className="flex w-72 shrink-0 flex-col gap-4 border-r border-slate-200 bg-slate-50/60 p-4">
      <div>
        <h1 className="flex items-center gap-2 text-lg font-bold text-slate-900">
          <span className="grid h-7 w-7 place-items-center rounded-lg bg-gradient-to-br from-blue-500 to-blue-700 text-sm text-white">
            ₵
          </span>
          agentT
        </h1>
        <p className="text-xs text-slate-400">Sales / PO Intelligence</p>
      </div>

      <div className="rounded-xl border border-blue-100 bg-blue-50/60 p-3">
        <label className="mb-1 block text-xs font-medium text-blue-800">
          Generate digest
        </label>
        <div className="flex gap-2">
          <input
            type="date"
            value={date}
            onChange={(e) => setDate(e.target.value)}
            className="min-w-0 flex-1 rounded-md border border-blue-200 bg-white px-2 py-1.5 text-sm focus:border-blue-500 focus:outline-none"
          />
          <button
            onClick={generate}
            disabled={!date || runDaily.isPending}
            className="rounded-md bg-blue-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-40"
          >
            Run
          </button>
        </div>
        {runDaily.isPending && (
          <div className="mt-2">
            <Spinner label="Running agent loop…" />
          </div>
        )}
        {runDaily.isError && (
          <p className="mt-2 text-xs text-rose-600">
            {runDaily.error instanceof Error ? runDaily.error.message : "failed"}
          </p>
        )}
      </div>

      <nav className="min-h-0 flex-1 overflow-y-auto">
        <p className="px-1 pb-1 text-xs font-semibold uppercase tracking-wide text-slate-400">
          Daily digests
        </p>
        {isLoading ? (
          <Spinner />
        ) : !dates || dates.length === 0 ? (
          <p className="px-1 text-xs text-slate-400">No digests yet — generate one above.</p>
        ) : (
          <ul className="space-y-1">
            {dates.map((d) => (
              <li key={d}>
                <NavLink
                  to={`/digests/${d}`}
                  className={({ isActive }) => `${linkBase} ${isActive ? active : idle}`}
                >
                  {formatDate(d)}
                </NavLink>
              </li>
            ))}
          </ul>
        )}
      </nav>

      <div className="border-t border-slate-200 pt-3">
        <NavLink
          to="/report"
          className={({ isActive }) => `${linkBase} ${isActive ? active : idle}`}
        >
          📊 Monthly report
        </NavLink>
      </div>
    </aside>
  );
}
