// Single-expense data access: load (with revisions), update, delete, restore.
// The single-item GET deliberately returns soft-deleted rows so the detail
// page can render a read-only "deleted" view and offer Restore.
import { api } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import type { SplitPayload } from "@/components/SplitEditor.vue";

export type Expense = components["schemas"]["Expense"];
export type ExpenseRevision = components["schemas"]["ExpenseRevision"];

export async function getExpense(id: string): Promise<Expense | null> {
  const { data, error } = await api.GET("/v1/expenses/{id}", {
    params: { path: { id } },
  });
  return error || !data ? null : data;
}

export async function getExpenseRevisions(id: string): Promise<ExpenseRevision[]> {
  const { data, error } = await api.GET("/v1/expenses/{id}/revisions", {
    params: { path: { id } },
  });
  return error || !data ? [] : data;
}

export interface UpdateExpenseInput {
  description: string;
  amountCents: number;
  payerId: string;
  categoryId?: string;
  notes?: string;
  incurredAt: string; // YYYY-MM-DD
  // Only sent when the user touched the split editor; otherwise the backend
  // keeps/rescales the existing split.
  split?: SplitPayload | null;
}

export async function updateExpense(
  id: string,
  input: UpdateExpenseInput,
): Promise<{ ok: boolean }> {
  const body: components["schemas"]["UpdateExpenseRequest"] = {
    description: input.description,
    amount_cents: input.amountCents,
    payer_id: input.payerId,
  };
  if (input.categoryId) body.category_id = input.categoryId;
  body.notes = input.notes ?? "";
  if (/^\d{4}-\d{2}-\d{2}$/.test(input.incurredAt)) {
    body.incurred_at = `${input.incurredAt}T12:00:00Z`;
  }
  // mode + splits must be sent together; only when the editor was touched.
  if (input.split) {
    body.mode = input.split.mode;
    body.splits = input.split.splits;
  }
  const { error } = await api.PATCH("/v1/expenses/{id}", {
    params: { path: { id } },
    body,
  });
  return { ok: !error };
}

export async function deleteExpense(id: string): Promise<{ ok: boolean }> {
  const { error } = await api.DELETE("/v1/expenses/{id}", { params: { path: { id } } });
  return { ok: !error };
}

export async function restoreExpense(id: string): Promise<{ ok: boolean }> {
  const { error } = await api.POST("/v1/expenses/{id}/restore", {
    params: { path: { id } },
  });
  return { ok: !error };
}
