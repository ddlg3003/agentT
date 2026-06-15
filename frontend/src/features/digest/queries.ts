import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { api, type DailyDigest, type FollowupTurn } from "../../lib/api";

export const digestKeys = {
  list: ["digests"] as const,
  detail: (date: string) => ["digests", date] as const,
  history: (date: string) => ["digests", date, "history"] as const,
};

export function useDigestDates() {
  return useQuery({ queryKey: digestKeys.list, queryFn: api.listDigests });
}

export function useDigest(date: string | undefined) {
  return useQuery({
    queryKey: digestKeys.detail(date ?? ""),
    queryFn: () => api.getDigest(date!),
    enabled: Boolean(date),
  });
}

/** Trigger an on-demand daily run, then refresh the list + that date. */
export function useRunDaily() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (date: string) => api.runDaily(date),
    onSuccess: (digest: DailyDigest) => {
      qc.setQueryData(digestKeys.detail(digest.date), digest);
      qc.invalidateQueries({ queryKey: digestKeys.list });
    },
  });
}

export function useFlagDigest(date: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (note: string) => api.flagDigest(date, note),
    onSuccess: (digest: DailyDigest) =>
      qc.setQueryData(digestKeys.detail(date), digest),
  });
}

export function useFollowupHistory(date: string) {
  return useQuery({
    queryKey: digestKeys.history(date),
    queryFn: () => api.getFollowupHistory(date),
    enabled: Boolean(date),
    staleTime: Infinity,
    refetchOnWindowFocus: false,
  });
}

/** Ask a follow-up. On success we refetch the digest because the agent may
 *  have applied a correction via the update_digest tool. */
export function useAskFollowup(date: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (question: string) => api.askFollowup(date, question),
    onSuccess: (res, question) => {
      qc.invalidateQueries({ queryKey: digestKeys.detail(date), exact: true });
      // Append directly to cache instead of refetching — avoids the
      // eventual-consistency window where GreenNode may not yet return the
      // turn we just wrote.
      qc.setQueryData(digestKeys.history(date), (old: FollowupTurn[] | undefined) => [
        ...(old ?? []),
        { role: "user" as const, content: question },
        { role: "assistant" as const, content: res.answer },
      ]);
    },
  });
}
