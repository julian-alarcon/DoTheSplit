// Resolve a bearer-authed avatar PNG into an object URL.
//
// The avatar endpoint (/v1/users/:id/avatar) requires the Authorization
// header, so a plain <img src> can't load it (img requests don't carry the
// bearer token). We fetch the bytes through the typed client and hand back a
// blob: URL, revoking it on cleanup. A module-level cache keyed by
// userId+version dedupes the many avatars that repeat across a transaction
// feed and survives component remounts within a session.
import { onUnmounted, ref, watch, type Ref } from "vue";
import { api } from "@/lib/api/client";

const cache = new Map<string, string>();

async function fetchAvatar(userId: string, version: string): Promise<string | null> {
  const key = `${userId}@${version}`;
  const cached = cache.get(key);
  if (cached) return cached;
  const { data, error } = await api.GET("/v1/users/{id}/avatar", {
    // Pass the version as a query param so a changed avatar yields a distinct
    // URL. The server sends `Cache-Control: private, max-age=...`, so without
    // this the browser would serve the stale cached bytes after an update.
    params: { path: { id: userId }, query: version ? { v: version } : {} },
    parseAs: "blob",
  });
  if (error || !data) return null;
  const url = URL.createObjectURL(data as Blob);
  cache.set(key, url);
  return url;
}

/**
 * Reactively resolve an avatar URL. Pass reactive refs so the URL updates when
 * the member or their avatar version changes. Returns null while loading or
 * when the user has no avatar.
 */
export function useAvatarUrl(
  userId: Ref<string | undefined>,
  hasAvatar: Ref<boolean | undefined>,
  version: Ref<string | undefined>,
) {
  const url = ref<string | null>(null);

  watch(
    [userId, hasAvatar, version],
    async ([id, has, ver]) => {
      if (!id || !has) {
        url.value = null;
        return;
      }
      url.value = await fetchAvatar(id, ver ?? "");
    },
    { immediate: true },
  );

  // Cached blob URLs are intentionally not revoked here: they're shared across
  // many rows and the cache lives for the session. The browser reclaims them
  // on navigation away from the SPA origin.
  onUnmounted(() => {});

  return { url };
}
