// First-run setup state. Mirrors the Astro middleware's setup probe: the
// instance is "locked" once the install ceremony has produced the first admin.
// While unlocked, the router funnels every route to /setup. Defaults to locked
// if the API is unreachable so a network blip never exposes the install flow.
import { reactive, readonly } from "vue";
import { api } from "@/lib/api/client";

const state = reactive({
  locked: true,
  // Flips true once the first probe settles, so the guard only probes once.
  checked: false,
});

async function ensureChecked(): Promise<void> {
  if (state.checked) return;
  try {
    const { data } = await api.GET("/v1/setup/status");
    state.locked = data ? data.locked : true;
  } catch {
    state.locked = true;
  } finally {
    state.checked = true;
  }
}

// After completing setup or signing in, the ceremony is closed; mark locked so
// the guard stops funnelling to /setup without another round-trip.
function markLocked(): void {
  state.locked = true;
  state.checked = true;
}

export function useSetup() {
  return { state: readonly(state), ensureChecked, markLocked };
}
