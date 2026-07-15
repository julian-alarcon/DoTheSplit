// Group dashboard data access. Loads the group, balances, categories,
// recurring templates, and the first page of the transaction feed in parallel,
// and exposes create-expense + load-more helpers. Plain functions returning
// typed data; the view owns its
// own reactive state.
import { api } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import type { SplitPayload } from "@/components/SplitEditor.vue";

export type Category = components["schemas"]["Category"];
export type TransactionItem = components["schemas"]["TransactionItem"];
export type SimplifiedDebt = components["schemas"]["SimplifiedDebt"];
export type RecurringExpense = components["schemas"]["RecurringExpense"];
type Cadence = components["schemas"]["Cadence"];

export interface DashboardData {
  simplified: SimplifiedDebt[];
  categories: Category[];
  recurring: RecurringExpense[];
  transactions: TransactionItem[];
  nextCursor: string;
  // True if any of the four parallel reads failed, so the view can show an
  // error banner instead of a misleading empty dashboard.
  error: boolean;
}

export async function loadDashboard(groupId: string): Promise<DashboardData> {
  const [balancesRes, categoriesRes, recurringRes, transactionsRes] = await Promise.all([
    api.GET("/v1/groups/{id}/balances", { params: { path: { id: groupId } } }),
    api.GET("/v1/categories"),
    api.GET("/v1/groups/{id}/recurring-expenses", { params: { path: { id: groupId } } }),
    api.GET("/v1/groups/{id}/transactions", { params: { path: { id: groupId } } }),
  ]);
  return {
    simplified: balancesRes.data?.simplified ?? [],
    categories: categoriesRes.data ?? [],
    recurring: recurringRes.data ?? [],
    transactions: transactionsRes.data?.items ?? [],
    nextCursor: transactionsRes.data?.next_cursor ?? "",
    error: Boolean(
      balancesRes.error || categoriesRes.error || recurringRes.error || transactionsRes.error,
    ),
  };
}

export async function loadMoreTransactions(
  groupId: string,
  cursor: string,
): Promise<{ items: TransactionItem[]; nextCursor: string; error: boolean }> {
  const { data, error } = await api.GET("/v1/groups/{id}/transactions", {
    params: { path: { id: groupId }, query: { cursor } },
  });
  return {
    items: data?.items ?? [],
    nextCursor: data?.next_cursor ?? "",
    error: Boolean(error),
  };
}

function isValidCadence(c: string): c is Cadence {
  return ["daily", "weekly", "biweekly", "monthly", "yearly"].includes(c);
}

// Mirrors server/internal/service/recurring.go advanceCadence so we can anchor the
// recurring template at the *next* occurrence without an extra round-trip.
function advanceCadence(fromISO: string, cadence: Cadence): string {
  const d = new Date(fromISO);
  switch (cadence) {
    case "daily":
      d.setUTCDate(d.getUTCDate() + 1);
      break;
    case "weekly":
      d.setUTCDate(d.getUTCDate() + 7);
      break;
    case "biweekly":
      d.setUTCDate(d.getUTCDate() + 14);
      break;
    case "monthly":
      d.setUTCMonth(d.getUTCMonth() + 1);
      break;
    case "yearly":
      d.setUTCFullYear(d.getUTCFullYear() + 1);
      break;
  }
  return d.toISOString();
}

export interface CreateExpenseInput {
  description: string;
  amountCents: number;
  payerId: string;
  categoryId?: string;
  notes?: string;
  incurredAt: string; // YYYY-MM-DD
  cadence?: string;
  split: SplitPayload;
}

/**
 * Create an expense and, when a cadence is chosen, a matching recurring
 * template anchored at the next occurrence. Returns a status code on partial
 * failure.
 */
export async function createExpense(
  groupId: string,
  input: CreateExpenseInput,
): Promise<{ ok: boolean; code?: string }> {
  let incurredAtISO: string | null = null;
  if (/^\d{4}-\d{2}-\d{2}$/.test(input.incurredAt)) {
    incurredAtISO = `${input.incurredAt}T12:00:00Z`;
  }

  const body: components["schemas"]["CreateExpenseRequest"] = {
    description: input.description,
    amount_cents: input.amountCents,
    payer_id: input.payerId,
    mode: input.split.mode,
    splits: input.split.splits,
  };
  if (input.categoryId) body.category_id = input.categoryId;
  if (input.notes) body.notes = input.notes;
  if (incurredAtISO) body.incurred_at = incurredAtISO;

  const { error } = await api.POST("/v1/groups/{id}/expenses", {
    params: { path: { id: groupId } },
    body,
  });
  if (error) return { ok: false, code: "expense_create" };

  const cadence = (input.cadence ?? "").trim();
  if (isValidCadence(cadence) && incurredAtISO) {
    const recurringBody: components["schemas"]["CreateRecurringExpenseRequest"] = {
      description: input.description,
      amount_cents: input.amountCents,
      payer_id: input.payerId,
      mode: input.split.mode,
      splits: input.split.splits,
      cadence,
      next_run_at: advanceCadence(incurredAtISO, cadence),
    };
    if (input.categoryId) recurringBody.category_id = input.categoryId;
    const { error: recErr } = await api.POST("/v1/groups/{id}/recurring-expenses", {
      params: { path: { id: groupId } },
      body: recurringBody,
    });
    if (recErr) return { ok: false, code: "recurring_create" };
  }

  return { ok: true };
}
