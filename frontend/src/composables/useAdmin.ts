// Admin surface data access: users, groups, SMTP, audit log. All endpoints
// are admin-gated server-side; the router guard only hides the UI. Destructive
// actions (delete user/group, role change, password reset) take a step-up
// password. Plain functions; views own their reactive state.
import { api } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";

export type AdminUser = components["schemas"]["AdminUser"];
export type AdminGroup = components["schemas"]["AdminGroup"];
export type SmtpConfig = components["schemas"]["SmtpConfig"];
export type SmtpTlsMode = components["schemas"]["SmtpTlsMode"];
export type AdminAuditEntry = components["schemas"]["AdminAuditEntry"];

const PAGE = 50;

export async function listUsers(offset: number, includeDeleted: boolean) {
  const { data, error } = await api.GET("/v1/admin/users", {
    params: { query: { limit: PAGE, offset, include_deleted: includeDeleted } },
  });
  if (error || !data) return { items: [] as AdminUser[], total: 0 };
  return { items: data.items, total: data.total };
}

export async function getUser(id: string): Promise<AdminUser | null> {
  const { data, error } = await api.GET("/v1/admin/users/{id}", {
    params: { path: { id } },
  });
  return error || !data ? null : data;
}

export async function createUser(input: {
  email: string;
  display_name: string;
  role: "user" | "admin";
}): Promise<{ ok: boolean; code?: string }> {
  const { error, response } = await api.POST("/v1/admin/users", { body: input });
  if (error) {
    // 400 here usually means SMTP isn't configured (the invite is emailed).
    if (response.status === 400) return { ok: false, code: "smtp" };
    return { ok: false, code: "exists" };
  }
  return { ok: true };
}

// Map the API status codes for step-up flows to banner codes
// (step_up / locked / last_admin / smtp).
function stepUpCode(status: number): string {
  if (status === 401) return "step_up";
  if (status === 423) return "locked";
  if (status === 409) return "last_admin";
  if (status === 400) return "smtp";
  return "error";
}

export async function setUserRole(
  id: string,
  role: "user" | "admin",
  password: string,
): Promise<{ ok: boolean; code?: string }> {
  const { error, response } = await api.PATCH("/v1/admin/users/{id}/role", {
    params: { path: { id } },
    body: { role, password },
  });
  return error ? { ok: false, code: stepUpCode(response.status) } : { ok: true };
}

export async function resetUserPassword(
  id: string,
  password: string,
): Promise<{ ok: boolean; code?: string }> {
  const { error, response } = await api.POST("/v1/admin/users/{id}/password", {
    params: { path: { id } },
    body: { password },
  });
  return error ? { ok: false, code: stepUpCode(response.status) } : { ok: true };
}

export async function deleteUser(
  id: string,
  password: string,
): Promise<{ ok: boolean; code?: string }> {
  const { error, response } = await api.DELETE("/v1/admin/users/{id}", {
    params: { path: { id } },
    body: { password },
  });
  return error ? { ok: false, code: stepUpCode(response.status) } : { ok: true };
}

export async function listGroups(offset: number) {
  const { data, error } = await api.GET("/v1/admin/groups", {
    params: { query: { limit: PAGE, offset } },
  });
  if (error || !data) return { items: [] as AdminGroup[], total: 0 };
  return { items: data.items, total: data.total };
}

export async function deleteGroup(
  id: string,
  password: string,
): Promise<{ ok: boolean; code?: string }> {
  const { error, response } = await api.DELETE("/v1/admin/groups/{id}", {
    params: { path: { id } },
    body: { password },
  });
  return error ? { ok: false, code: stepUpCode(response.status) } : { ok: true };
}

export async function getSmtp(): Promise<{ config: SmtpConfig | null; notConfigured: boolean }> {
  const { data, error, response } = await api.GET("/v1/admin/smtp");
  if (error || !data) return { config: null, notConfigured: response.status === 404 };
  return { config: data, notConfigured: false };
}

export async function revealSmtpPassword(): Promise<string> {
  const { data, error } = await api.GET("/v1/admin/smtp/password");
  return error || !data ? "" : data.password;
}

export interface SmtpUpdateInput {
  host: string;
  port: number;
  username?: string | null;
  from_address: string;
  tls_mode: SmtpTlsMode;
  // null = keep existing, "" = clear, non-empty = set new.
  smtp_password?: string | null;
  allow_plaintext_credentials: boolean;
}

export async function updateSmtp(input: SmtpUpdateInput): Promise<{ ok: boolean }> {
  const { error } = await api.PUT("/v1/admin/smtp", { body: input });
  return { ok: !error };
}

export async function testSmtp(): Promise<{ success: boolean; error?: string }> {
  const { data, error } = await api.POST("/v1/admin/smtp/test");
  if (error || !data) return { success: false, error: "error" };
  return { success: data.success, error: data.error };
}

export async function sendSmtpTestEmail(): Promise<{ success: boolean; error?: string }> {
  const { data, error } = await api.POST("/v1/admin/smtp/send-test");
  if (error || !data) return { success: false, error: "error" };
  return { success: data.success, error: data.error };
}

export async function listAudit(offset: number, action: string) {
  const { data, error } = await api.GET("/v1/admin/audit", {
    params: { query: { limit: PAGE, offset, action: action || undefined } },
  });
  if (error || !data) return { items: [] as AdminAuditEntry[], total: 0 };
  return { items: data.items, total: data.total };
}

export const ADMIN_PAGE = PAGE;
