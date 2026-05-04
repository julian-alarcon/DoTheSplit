import type { APIRoute } from "astro";

const internalBase =
  process.env.API_BASE_URL_INTERNAL ?? "http://localhost:8080";

export const POST: APIRoute = async ({ request, url, redirect }) => {
  const groupID = url.searchParams.get("id");
  if (!groupID) return new Response("missing id", { status: 400 });
  const cookie = request.headers.get("cookie") ?? "";
  const form = await request.formData();
  await fetch(`${internalBase}/v1/groups/${groupID}/members`, {
    method: "POST",
    headers: { "Content-Type": "application/json", cookie },
    body: JSON.stringify({ email: form.get("email") }),
  });
  return redirect(`/groups/${groupID}`, 302);
};
