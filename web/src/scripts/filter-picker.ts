// Generic single-select picker for chip-style filter triggers.
//
// Expected markup inside each [data-filter-picker]:
//   <button data-filter-open>
//     <span data-filter-icon>(optional)</span>
//     <span data-filter-label>All</span>
//   </button>
//   <input type="hidden" name="..." value="">
//   <dialog data-filter-dialog>
//     <button data-filter-cancel>...</button>
//     <button data-filter-option data-value="<id-or-empty>" data-label="...">
//       <svg>(optional - cloned into trigger icon slot on selection)</svg>
//       ...
//     </button>
//   </dialog>
//
// Selecting an option writes data-value to the hidden input, swaps the trigger
// label/icon, and closes the dialog. An empty data-value means "all" and is
// represented in the URL by an empty (and therefore filtered-out) param value.

function setupFilterPicker(root: HTMLElement) {
  const dialog = root.querySelector<HTMLDialogElement>("[data-filter-dialog]");
  const openBtn = root.querySelector<HTMLElement>("[data-filter-open]");
  const labelEl = root.querySelector<HTMLElement>("[data-filter-label]");
  const iconEl = root.querySelector<HTMLElement>("[data-filter-icon]");
  const hiddenInput = root.querySelector<HTMLInputElement>("input[type=hidden]");
  const cancelBtns = root.querySelectorAll<HTMLButtonElement>("[data-filter-cancel]");
  if (!dialog || !openBtn || !labelEl || !hiddenInput) return;

  openBtn.addEventListener("click", (e) => {
    e.preventDefault();
    dialog.showModal();
  });

  cancelBtns.forEach((btn) => {
    btn.addEventListener("click", (e) => {
      e.preventDefault();
      dialog.close();
    });
  });

  root.querySelectorAll<HTMLButtonElement>("[data-filter-option]").forEach((btn) => {
    btn.addEventListener("click", (e) => {
      e.preventDefault();
      const value = btn.dataset.value ?? "";
      const label = btn.dataset.label ?? "";
      hiddenInput.value = value;
      hiddenInput.dispatchEvent(new Event("input", { bubbles: true }));
      hiddenInput.dispatchEvent(new Event("change", { bubbles: true }));
      labelEl.textContent = label;
      if (iconEl) {
        const sourceSvg = btn.querySelector("svg");
        if (sourceSvg) {
          iconEl.replaceChildren(sourceSvg.cloneNode(true));
        } else {
          iconEl.replaceChildren();
        }
      }
      dialog.close();
      // If this picker is inside a form marked for autosubmit, re-run the
      // search immediately so the user doesn't have to click Search again.
      if (root.closest("[data-filter-autosubmit]")) {
        const form = root.closest("form");
        form?.requestSubmit();
      }
    });
  });
}

document
  .querySelectorAll<HTMLElement>("[data-filter-picker]")
  .forEach(setupFilterPicker);

// Strip empty filter inputs on submit so the resulting URL stays clean
// (e.g. /search?q=foo instead of /search?q=foo&group_id=&category_id=).
document.querySelectorAll<HTMLFormElement>("[data-filter-form]").forEach((form) => {
  form.addEventListener("submit", () => {
    form.querySelectorAll<HTMLInputElement>("input[type=hidden][data-filter-strip-empty]").forEach((input) => {
      if (input.value === "") input.disabled = true;
    });
  });
});

export {};
