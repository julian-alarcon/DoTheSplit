// Disable the [data-dirty-submit] button while its [data-dirty-form] form
// matches the initial snapshot (no changes). Bundled by Astro as /_astro/*.js
// so CSP `script-src 'self'` covers it without per-page hashes.
document.querySelectorAll<HTMLFormElement>("[data-dirty-form]").forEach((form) => {
  const submit = form.querySelector<HTMLButtonElement>("[data-dirty-submit]");
  if (!submit) return;
  const snapshot = (f: HTMLFormElement) =>
    Array.from(new FormData(f).entries())
      .map(([k, v]) => `${k}=${v}`)
      .sort()
      .join("&");
  const initial = snapshot(form);
  const refresh = () => {
    submit.disabled = snapshot(form) === initial;
  };
  form.addEventListener("input", refresh);
  form.addEventListener("change", refresh);
  refresh();
});
