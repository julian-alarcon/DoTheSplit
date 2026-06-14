// Search across the caller's groups. The category filter implies
// "expenses only" server-side (settlements have no category).
import { api } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";

export type SearchResponse = components["schemas"]["SearchResponse"];
export type SearchGroupRef = components["schemas"]["SearchGroupRef"];

export async function search(
  q: string,
  opts: { groupId?: string; categoryId?: string } = {},
): Promise<{ data: SearchResponse | null; status: number }> {
  const query: { q: string; group_id?: string[]; category_id?: string } = { q };
  if (opts.groupId) query.group_id = [opts.groupId];
  if (opts.categoryId) query.category_id = opts.categoryId;
  const { data, error, response } = await api.GET("/v1/search", { params: { query } });
  return { data: error || !data ? null : data, status: response.status };
}
