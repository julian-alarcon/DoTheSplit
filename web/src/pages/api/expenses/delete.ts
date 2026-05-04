import type { APIRoute } from "astro";

const internalBase =
  process.env.API_BASE_URL_INTERNAL ?? "http://localhost:8080";

export const POST: APIRoute = async ({ request, url, redirect }) => {
  const id = url.searchParams.get("id");
  const group = url.searchParams.get("group");
  if (!id || !group) return new Response("missing id", { status: 400 });
  const cookie = request.headers.get("cookie") ?? "";
  await fetch(`${internalBase}/v1/expenses/${id}`, {
    method: "DELETE",
    headers: { cookie },
  });
  return redirect(`/groups/${group}`, 302);
};
