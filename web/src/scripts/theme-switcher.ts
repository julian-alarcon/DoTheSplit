// Footer theme picker. Per-device only: the value lives in localStorage
// (key: dts_theme) and never leaves the browser. The inline boot script in
// Base.astro applies the resolved theme before paint; this script checks the
// radio matching the stored value and writes localStorage on change.
//
// Default = "dark" when nothing is stored.

type Theme = "dark" | "light" | "high-contrast";

const STORAGE_KEY = "dts_theme";

function isTheme(v: string | null): v is Theme {
  return v === "dark" || v === "light" || v === "high-contrast";
}

const fieldset = document.querySelector<HTMLFieldSetElement>("[data-theme-picker]");
if (fieldset) {
  const radios = fieldset.querySelectorAll<HTMLInputElement>('input[name="theme"]');

  let stored: string | null = null;
  try {
    stored = localStorage.getItem(STORAGE_KEY);
  } catch {
    // Storage may be blocked (private mode); picker still works in-session.
  }
  const current: Theme = isTheme(stored) ? stored : "dark";

  radios.forEach((radio) => {
    radio.checked = radio.value === current;
    radio.addEventListener("change", () => {
      if (!radio.checked || !isTheme(radio.value)) return;
      try {
        localStorage.setItem(STORAGE_KEY, radio.value);
      } catch {
        // Silently degrade.
      }
      document.documentElement.dataset.theme = radio.value;
    });
  });
}
