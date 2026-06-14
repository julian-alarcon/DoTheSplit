// Theme state. Per-device only: the value lives in localStorage (key
// dts_theme) and never leaves the browser. The /theme-boot.js script applies
// the resolved theme before first paint; this composable mirrors that state
// reactively for the switcher and writes localStorage + the <html> dataset on
// change. Default is "dark" when nothing is stored.
import { ref } from "vue";

export type Theme = "dark" | "light" | "high-contrast";

const STORAGE_KEY = "dts_theme";

function isTheme(v: string | null): v is Theme {
  return v === "dark" || v === "light" || v === "high-contrast";
}

function readStored(): Theme {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (isTheme(stored)) return stored;
  } catch {
    // Storage may be blocked (private mode); fall through to the default.
  }
  return "dark";
}

// Module singleton so every <ThemeSwitcher> instance shares one source of
// truth. Seeded from whatever /theme-boot.js already wrote to <html>.
const current = ref<Theme>(
  isTheme(document.documentElement.dataset.theme ?? null)
    ? (document.documentElement.dataset.theme as Theme)
    : readStored(),
);

export function useTheme() {
  function setTheme(next: Theme) {
    current.value = next;
    document.documentElement.dataset.theme = next;
    try {
      localStorage.setItem(STORAGE_KEY, next);
    } catch {
      // Silently degrade; the in-session change still applies.
    }
  }
  return { theme: current, setTheme };
}
