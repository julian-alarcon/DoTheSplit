import type { APIRoute } from "astro";
import { apiFetch, cookieFrom } from "@/lib/api/forward";

// POST /api/admin/group-delete: admin-cascade-delete any group.
export const POST: APIRoute = async ({ request, redirect }) => {
  const form = await request.formData();
  const cookie = cookieFrom(request);
  const id = (form.get("id") ?? "").toString();
  if (!id) return redirect("/admin/groups?delete_error=missing_id", 302);
  const res = await apiFetch(`/v1/admin/groups/${id}`, {
    method: "DELETE",
    cookie,
    json: { password: (form.get("password") ?? "").toString() },
  });
  if (res.status === 204) return redirect("/admin/groups?deleted=1", 302);
  if (res.status === 401) return redirect("/admin/groups?delete_error=step_up", 302);
  if (res.status === 423) return redirect("/admin/groups?delete_error=locked", 302);
  return redirect("/admin/groups?delete_error=1", 302);
};
