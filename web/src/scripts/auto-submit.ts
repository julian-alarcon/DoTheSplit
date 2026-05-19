export {};

// Auto-submit a form when an input inside it changes. Marker: any element
// with `data-auto-submit` triggers a form submit on its `change` event. Used
// by filter toggles where we want the form-GET pattern (URL state, server
// renders the result) without an explicit "Apply" button.

function setup() {
  const triggers = document.querySelectorAll<HTMLInputElement>(
    "[data-auto-submit]",
  );
  for (const el of triggers) {
    el.addEventListener("change", () => {
      el.form?.requestSubmit();
    });
  }
}

setup();
