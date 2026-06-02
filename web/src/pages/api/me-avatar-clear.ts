import type { APIRoute } from "astro";

const internalBase =
  process.env.API_BASE_URL_INTERNAL ?? "http://localhost:8080";

export const POST: APIRoute = async ({ request, redirect }) => {
  const cookie = request.headers.get("cookie") ?? "";
  const res = await fetch(`${internalBase}/v1/me/avatar`, {
    method: "DELETE",
    headers: { cookie },
  });
  if (!res.ok && res.status !== 204) {
    return redirect("/settings?error=avatar", 302);
  }
  return redirect("/settings?ok=avatar_cleared", 302);
};
