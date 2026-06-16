import { useState } from "react";
import { useMutation } from "@tanstack/react-query";

import { api } from "../../lib/api";
import { Card, MarkdownView, Spinner } from "../../components/ui";

export function MonthlyPage() {
  const [month, setMonth] = useState("");
  const report = useMutation({ mutationFn: (ym: string) => api.monthlyReport(ym) });

  return (
    <div className="mx-auto h-full max-w-3xl space-y-5 overflow-y-auto pb-6">
      <header>
        <h1 className="text-2xl font-bold text-slate-900">Monthly report</h1>
        <p className="text-sm text-slate-400">
          Month-over-month synthesis across the month's daily digests.
        </p>
      </header>

      <Card>
        <div className="flex flex-wrap items-center gap-2">
          <input
            type="month"
            value={month}
            onChange={(e) => setMonth(e.target.value)}
            className="rounded-md border border-slate-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none"
          />
          <button
            onClick={() => month && report.mutate(month)}
            disabled={!month || report.isPending}
            className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-40"
          >
            Synthesize
          </button>
          {report.isPending && <Spinner timed />}
        </div>
        {report.isError && (
          <p className="mt-2 text-sm text-rose-600">
            {report.error instanceof Error ? report.error.message : "failed"}
          </p>
        )}
      </Card>

      {report.data && (
        <Card
          title={`Report · ${report.data.month}`}
          subtitle={`Synthesized from ${report.data.num_digests} daily digests`}
          action={
            <button
              onClick={() => {
                const blob = new Blob([report.data!.markdown], { type: "text/markdown" });
                const url = URL.createObjectURL(blob);
                const a = document.createElement("a");
                a.href = url;
                a.download = `monthly-report-${report.data!.month}.md`;
                a.click();
                URL.revokeObjectURL(url);
              }}
              className="rounded-md border border-slate-300 px-3 py-1.5 text-xs font-medium text-slate-600 hover:bg-slate-50"
            >
              Export as Markdown
            </button>
          }
        >
          <MarkdownView>{report.data.markdown}</MarkdownView>
        </Card>
      )}
    </div>
  );
}
