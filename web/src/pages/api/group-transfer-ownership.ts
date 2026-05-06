import type { APIRoute } from "astro";

const internalBase =
  process.env.API_BASE_URL_INTERNAL ?? "http://localhost:8080";

export const POST: APIRoute = async ({ request, url, redirect }) => {
  const groupID = url.searchParams.get("id");
  if (!groupID) return new Response("missing id", { status: 400 });
  const cookie = request.headers.get("cookie") ?? "";
  const form = await request.formData();
  const newOwnerID = String(form.get("new_owner_id") ?? "").trim();
  if (!newOwnerID) {
    return redirect(`/groups/${groupID}/settings?error=1`, 302);
  }

  const res = await fetch(`${internalBase}/v1/groups/${groupID}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json", cookie },
    body: JSON.stringify({ created_by: newOwnerID }),
  });
  if (!res.ok) {
    let message = "transfer_failed";
    try {
      const body = (await res.json()) as { message?: string };
      if (body?.message) message = body.message;
    } catch {
      // ignore
    }
    return redirect(
      `/groups/${groupID}/settings?error=1&reason=${encodeURIComponent(message)}`,
      302,
    );
  }
  return redirect(`/groups/${groupID}/settings`, 302);
};
