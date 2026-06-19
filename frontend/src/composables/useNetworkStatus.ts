// Network status. Module singleton (like useTheme) so every consumer shares one
// reactive source of truth, seeded from navigator.onLine and kept in sync by the
// window online/offline events. Read-only offline: the SW serves cached /v1 GETs
// while offline, the AppLayout shows a banner, and mutations are short-circuited
// in the API client (see lib/api/client.ts).
import { computed, ref } from "vue";

const online = ref(navigator.onLine);

window.addEventListener("online", () => {
  online.value = true;
});
window.addEventListener("offline", () => {
  online.value = false;
});

const offline = computed(() => !online.value);

export function useNetworkStatus() {
  return { online, offline };
}
