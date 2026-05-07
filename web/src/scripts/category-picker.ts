// Sync the emoji on the <summary> with the checked category radio, and close
// the picker after a selection. Kept in a real module file so Astro bundles
// it as /_astro/*.js - covered by CSP `script-src 'self'`.
document.querySelectorAll<HTMLDetailsElement>("[data-category-picker]").forEach((picker) => {
  const summary = picker.querySelector<HTMLElement>("[data-category-emoji]");
  picker.querySelectorAll<HTMLInputElement>("[data-category-option]").forEach((input) => {
    input.addEventListener("change", () => {
      if (input.checked && summary) summary.textContent = input.dataset.emoji ?? "";
      picker.open = false;
    });
  });
});
