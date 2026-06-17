// Group data access. Thin wrappers over the typed client; no shared reactive
// state (each view owns its own loading/error refs), so this is a plain module
// of functions rather than a singleton store. Per the migration simplicity
// rules, we hand-roll fetching until it becomes painful.
import { api } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";

export type Group = components["schemas"]["Group"];
export type GroupMember = components["schemas"]["GroupMember"];

// Returns `error: true` on a network/5xx failure so callers can distinguish a
// genuine "no groups" empty state from a transient fetch failure.
export async function listGroups(): Promise<{ groups: Group[]; error: boolean }> {
  const { data, error } = await api.GET("/v1/groups");
  if (error || !data) return { groups: [], error: true };
  return { groups: data, error: false };
}

// There is no single-group GET endpoint; the list response embeds members, so
// the dashboard resolves its group from the list.
export async function getGroup(
  groupId: string,
): Promise<{ group: Group | null; error: boolean }> {
  const { groups, error } = await listGroups();
  return { group: groups.find((g) => g.id === groupId) ?? null, error };
}

export type Balance = components["schemas"]["Balance"];
export type SimplifiedDebt = components["schemas"]["SimplifiedDebt"];

export async function getBalances(
  groupId: string,
): Promise<{ net: Balance[]; simplified: SimplifiedDebt[]; error: boolean }> {
  const { data, error } = await api.GET("/v1/groups/{id}/balances", {
    params: { path: { id: groupId } },
  });
  if (error || !data) return { net: [], simplified: [], error: true };
  return { net: data.net, simplified: data.simplified, error: false };
}

// True when the group's ledger has at least one transaction (the currency is
// then locked, since changing it would reinterpret stored amount_cents).
export async function hasTransactions(groupId: string): Promise<boolean> {
  const { data } = await api.GET("/v1/groups/{id}/transactions", {
    params: { path: { id: groupId }, query: { limit: 1 } },
  });
  return (data?.items?.length ?? 0) > 0;
}

export async function addMember(groupId: string, email: string): Promise<{ ok: boolean }> {
  const { error } = await api.POST("/v1/groups/{id}/members", {
    params: { path: { id: groupId } },
    body: { email },
  });
  return { ok: !error };
}

export async function removeMember(
  groupId: string,
  userId: string,
): Promise<{ ok: boolean; message?: string }> {
  const { error } = await api.DELETE("/v1/groups/{id}/members/{userId}", {
    params: { path: { id: groupId, userId } },
  });
  return { ok: !error, message: error?.message };
}

// All group-settings mutations (rename, currency, default_split, ownership
// transfer) go through the single updateGroup PATCH.
type DefaultSplitEntry = components["schemas"]["DefaultSplitEntry"];
export async function updateGroup(
  groupId: string,
  patch: {
    name?: string;
    default_currency?: string;
    default_split?: DefaultSplitEntry[] | null;
    created_by?: string;
  },
): Promise<{ ok: boolean }> {
  const { error } = await api.PATCH("/v1/groups/{id}", {
    params: { path: { id: groupId } },
    body: patch,
  });
  return { ok: !error };
}

export async function deleteGroup(groupId: string): Promise<{ ok: boolean }> {
  const { error } = await api.DELETE("/v1/groups/{id}", { params: { path: { id: groupId } } });
  return { ok: !error };
}

// Export the group ledger as CSV. The endpoint is bearer-authed so we can't
// use a plain <a download> link; fetch the bytes through the typed client and
// hand back the blob + filename for the caller to trigger a download.
export async function exportCsv(
  groupId: string,
): Promise<{ ok: boolean; blob?: Blob; filename?: string }> {
  const { data, error, response } = await api.GET("/v1/groups/{id}/export.csv", {
    params: { path: { id: groupId } },
    parseAs: "blob",
  });
  if (error || !data) return { ok: false };
  // Prefer the server's Content-Disposition filename when present.
  const cd = response.headers.get("content-disposition") ?? "";
  const match = /filename="?([^"]+)"?/.exec(cd);
  const filename = match?.[1] ?? `dothesplit-${groupId}.csv`;
  return { ok: true, blob: data as Blob, filename };
}

export async function deleteRecurring(id: string): Promise<{ ok: boolean }> {
  const { error } = await api.DELETE("/v1/recurring-expenses/{id}", {
    params: { path: { id } },
  });
  return { ok: !error };
}

export async function listRecurring(
  groupId: string,
): Promise<components["schemas"]["RecurringExpense"][]> {
  const { data, error } = await api.GET("/v1/groups/{id}/recurring-expenses", {
    params: { path: { id: groupId } },
  });
  return error || !data ? [] : data;
}

export async function listCategories(): Promise<components["schemas"]["Category"][]> {
  const { data, error } = await api.GET("/v1/categories");
  return error || !data ? [] : data;
}

export type ActivityItem = components["schemas"]["ActivityItem"];

export async function listActivity(
  groupId: string,
  cursor?: string,
): Promise<{ items: ActivityItem[]; nextCursor: string }> {
  const { data, error } = await api.GET("/v1/groups/{id}/activity", {
    params: { path: { id: groupId }, query: cursor ? { cursor } : {} },
  });
  if (error || !data) return { items: [], nextCursor: "" };
  return { items: data.items, nextCursor: data.next_cursor ?? "" };
}

export async function createGroup(input: {
  name: string;
  default_currency?: string;
}): Promise<{ ok: boolean; group?: Group }> {
  const currency = (input.default_currency ?? "").trim().toUpperCase() || "EUR";
  const { data, error } = await api.POST("/v1/groups", {
    body: { name: input.name, default_currency: currency },
  });
  if (error || !data) return { ok: false };
  return { ok: true, group: data };
}

/**
 * Invite a batch of emails to a group. Returns how many failed (unregistered
 * emails 404; the caller surfaces a retry banner). Best-effort, in parallel.
 */
export async function inviteMembers(
  groupId: string,
  emails: string[],
): Promise<number> {
  const results = await Promise.all(
    emails.map((email) =>
      api
        .POST("/v1/groups/{id}/members", {
          params: { path: { id: groupId } },
          body: { email },
        })
        .then((r) => !r.error)
        .catch(() => false),
    ),
  );
  return results.filter((ok) => !ok).length;
}
