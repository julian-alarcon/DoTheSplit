// CSV import data access. Two shapes:
//   - Group-expenses: append rows to an existing group (no member mapping).
//   - Full-group (Splitwise / DoTheSplit): create a new group, mapping each
//     CSV name column to a registered email. Both endpoints share a two-phase
//     dry_run preview then commit.
import { api } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";

export type GroupExpensesResponse = components["schemas"]["ImportGroupExpensesResponse"];
export type SplitwiseResponse = components["schemas"]["ImportSplitwiseResponse"];
export type SplitwiseMember = components["schemas"]["ImportSplitwiseMember"];
type ApiError = components["schemas"]["Error"];

const CSV_MAX = 262144; // 256 KiB, matches the server cap.

/**
 * Pull a human-readable reason out of an openapi-fetch error. The API returns
 * `{ code, message }` on every 4xx; surfacing `message` tells the user (and us)
 * *why* an import was rejected instead of a generic "it failed". Falls back to
 * the HTTP status when the body isn't the expected shape.
 */
function errMessage(error: unknown, status: number): string {
  const msg = (error as ApiError | undefined)?.message;
  return typeof msg === "string" && msg.trim() ? msg : `Request failed (HTTP ${status}).`;
}

export function csvTooLarge(text: string): boolean {
  return new Blob([text]).size > CSV_MAX;
}

// The full-group CSV header is
// `Date,Description,Category,Cost,Currency,[<optional dothesplit cols>,]<u1>,<u2>,...`.
// The user columns after the fixed prefix (and any optional metadata columns)
// become the members to map.
const HEADER_PREFIX = ["Date", "Description", "Category", "Cost", "Currency"];

// Optional metadata columns the DoTheSplit exporter inserts between Currency
// and the per-member block. Mirrors the server's `dothesplitExtraColumns`
// whitelist (api/internal/csvimport/splitwise.go); matched case-insensitively.
// Without skipping these, a DoTheSplit-shaped CSV would parse Time/Payer/etc.
// as members and the count would mismatch the server's, failing the import.
const OPTIONAL_COLUMNS = new Set(["time", "payer", "notes", "created", "createdby"]);

/** Member column names from the CSV header (the names the user maps to emails). */
export function memberNamesFromCsv(csv: string): string[] {
  const firstLine = csv.split(/\r?\n/, 1)[0] ?? "";
  const cols = firstLine.split(",").map((c) => c.trim());
  if (cols.length < HEADER_PREFIX.length + 2) return [];
  for (let i = 0; i < HEADER_PREFIX.length; i++) {
    if (cols[i] !== HEADER_PREFIX[i]) return [];
  }
  // Skip any contiguous run of optional metadata columns directly after the
  // fixed prefix; everything from there on is a member name.
  let start = HEADER_PREFIX.length;
  while (start < cols.length && OPTIONAL_COLUMNS.has(cols[start].toLowerCase())) {
    start++;
  }
  return cols.slice(start);
}

/**
 * Derive a sensible default group name from an uploaded file name. DoTheSplit
 * exports are named `<slug>_<YYYY-MM-DD>_export.csv` (see the export handler in
 * api/internal/handlers/groups.go); strip that trailing `_<date>_export` and the
 * `.csv`, then tidy separators so "prost_2026-06-10_export.csv" becomes "prost".
 * Falls back to the bare stem for arbitrary Splitwise file names. group_name is
 * required server-side, so pre-filling it stops the import from failing on a
 * forgotten field.
 */
export function groupNameFromFilename(filename: string): string {
  const stem = filename.replace(/\.csv$/i, "");
  const trimmed = stem.replace(/_\d{4}-\d{2}-\d{2}_export$/i, "");
  return trimmed.replace(/[_-]+/g, " ").trim();
}

/** First valid ISO-4217 code in the Currency column, defaulting to EUR. */
export function firstCsvCurrency(csv: string): string {
  const lines = csv.split(/\r?\n/);
  for (let i = 1; i < lines.length; i++) {
    const cur = (lines[i].split(",")[4] ?? "").trim().toUpperCase();
    if (/^[A-Z]{3}$/.test(cur)) return cur;
  }
  return "EUR";
}

// --- Group-expenses (append to existing group) ------------------------------

export async function previewGroupExpenses(
  groupId: string,
  csv: string,
): Promise<{ data: GroupExpensesResponse | null; message?: string }> {
  const { data, error, response } = await api.POST("/v1/groups/{id}/imports/expenses", {
    params: { path: { id: groupId } },
    body: { csv, dry_run: true },
  });
  if (error || !data) return { data: null, message: errMessage(error, response.status) };
  return { data };
}

export async function commitGroupExpenses(
  groupId: string,
  csv: string,
): Promise<{ ok: boolean; message?: string }> {
  const { error, response } = await api.POST("/v1/groups/{id}/imports/expenses", {
    params: { path: { id: groupId } },
    body: { csv, dry_run: false },
  });
  return error ? { ok: false, message: errMessage(error, response.status) } : { ok: true };
}

// --- Full-group import (Splitwise + DoTheSplit share the same shape) --------

type FullImportPath = "/v1/imports/splitwise" | "/v1/imports/dothesplit";

interface FullImportBody {
  csv: string;
  group_name: string;
  default_currency: string;
  members: SplitwiseMember[];
  dry_run: boolean;
}

/**
 * First-pass preview with minimal inputs: the server parses the CSV header and
 * returns the member columns to map. We send placeholder emails so validation
 * passes; the real mapping happens on the second preview/commit.
 */
export async function previewFullImport(
  source: "splitwise" | "dothesplit",
  body: FullImportBody,
): Promise<{ data: SplitwiseResponse | null; message?: string }> {
  const path: FullImportPath = source === "splitwise" ? "/v1/imports/splitwise" : "/v1/imports/dothesplit";
  const { data, error, response } = await api.POST(path, { body: { ...body, dry_run: true } });
  if (error || !data) return { data: null, message: errMessage(error, response.status) };
  return { data };
}

export async function commitFullImport(
  source: "splitwise" | "dothesplit",
  body: FullImportBody,
): Promise<{ ok: boolean; groupId?: string; message?: string }> {
  const path: FullImportPath = source === "splitwise" ? "/v1/imports/splitwise" : "/v1/imports/dothesplit";
  const { data, error, response } = await api.POST(path, { body: { ...body, dry_run: false } });
  if (error || !data) return { ok: false, message: errMessage(error, response.status) };
  return { ok: true, groupId: data.group_id };
}
