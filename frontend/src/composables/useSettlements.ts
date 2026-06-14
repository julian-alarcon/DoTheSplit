// Settlement data access: create, get, update, delete, restore. The
// single-item GET returns soft-deleted rows so the detail page can render a
// read-only deleted view with Restore.
import { api } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";

export type Settlement = components["schemas"]["Settlement"];

export async function getSettlement(id: string): Promise<Settlement | null> {
  const { data, error } = await api.GET("/v1/settlements/{id}", {
    params: { path: { id } },
  });
  return error || !data ? null : data;
}

export interface CreateSettlementInput {
  fromUserId: string;
  toUserId: string;
  amountCents: number;
  note?: string;
  settledAt: string; // YYYY-MM-DD
}

export async function createSettlement(
  groupId: string,
  input: CreateSettlementInput,
): Promise<{ ok: boolean }> {
  const body: components["schemas"]["CreateSettlementRequest"] = {
    from_user_id: input.fromUserId,
    to_user_id: input.toUserId,
    amount_cents: input.amountCents,
  };
  if (input.note) body.note = input.note;
  if (/^\d{4}-\d{2}-\d{2}$/.test(input.settledAt)) {
    body.settled_at = `${input.settledAt}T12:00:00Z`;
  }
  const { error } = await api.POST("/v1/groups/{id}/settlements", {
    params: { path: { id: groupId } },
    body,
  });
  return { ok: !error };
}

export interface UpdateSettlementInput {
  fromUserId: string;
  toUserId: string;
  amountCents: number;
  note?: string;
  settledAt: string; // YYYY-MM-DD
}

export async function updateSettlement(
  id: string,
  input: UpdateSettlementInput,
): Promise<{ ok: boolean }> {
  const body: components["schemas"]["UpdateSettlementRequest"] = {
    from_user_id: input.fromUserId,
    to_user_id: input.toUserId,
    amount_cents: input.amountCents,
    note: input.note ?? "",
  };
  if (/^\d{4}-\d{2}-\d{2}$/.test(input.settledAt)) {
    body.settled_at = `${input.settledAt}T12:00:00Z`;
  }
  const { error } = await api.PATCH("/v1/settlements/{id}", {
    params: { path: { id } },
    body,
  });
  return { ok: !error };
}

export async function deleteSettlement(id: string): Promise<{ ok: boolean }> {
  const { error } = await api.DELETE("/v1/settlements/{id}", {
    params: { path: { id } },
  });
  return { ok: !error };
}

export async function restoreSettlement(id: string): Promise<{ ok: boolean }> {
  const { error } = await api.POST("/v1/settlements/{id}/restore", {
    params: { path: { id } },
  });
  return { ok: !error };
}
