export {};

// Admin reset-password modal wiring: two fields (new password + admin's own
// password) injected as hidden inputs onto the trigger's form, then submit.

function setup() {
  const triggers = document.querySelectorAll<HTMLElement>("[data-reset-open]");
  for (const trigger of triggers) {
    const dialogId = trigger.dataset.resetOpen;
    if (!dialogId) continue;
    const dialog = document.getElementById(dialogId) as HTMLDialogElement | null;
    if (!dialog) continue;
    trigger.addEventListener("click", (e) => {
      e.preventDefault();
      const form = trigger.closest("form");
      if (!form) return;
      type DialogWithForm = HTMLDialogElement & { _targetForm?: HTMLFormElement };
      (dialog as DialogWithForm)._targetForm = form;
      const newInput = dialog.querySelector<HTMLInputElement>("[data-reset-new]");
      const adminInput = dialog.querySelector<HTMLInputElement>("[data-reset-admin]");
      const err = dialog.querySelector<HTMLElement>("[data-reset-error]");
      if (newInput) newInput.value = "";
      if (adminInput) adminInput.value = "";
      if (err) {
        err.textContent = "";
        err.classList.add("hidden");
      }
      dialog.showModal();
      queueMicrotask(() => newInput?.focus());
    });
  }

  const dialogs = document.querySelectorAll<HTMLDialogElement>("[data-reset-dialog]");
  for (const dialog of dialogs) {
    const cancels = dialog.querySelectorAll<HTMLButtonElement>("[data-reset-cancel]");
    for (const btn of cancels) {
      btn.addEventListener("click", (e) => {
        e.preventDefault();
        dialog.close();
      });
    }
    const newInput = dialog.querySelector<HTMLInputElement>("[data-reset-new]");
    const adminInput = dialog.querySelector<HTMLInputElement>("[data-reset-admin]");
    const err = dialog.querySelector<HTMLElement>("[data-reset-error]");
    const accept = dialog.querySelector<HTMLButtonElement>("[data-reset-accept]");
    const submit = () => {
      const newVal = newInput?.value ?? "";
      const adminVal = adminInput?.value ?? "";
      if (newVal.length < 10) {
        if (err) {
          err.textContent = "New password must be at least 10 characters.";
          err.classList.remove("hidden");
        }
        newInput?.focus();
        return;
      }
      if (!adminVal) {
        if (err) {
          err.textContent = "Your password is required.";
          err.classList.remove("hidden");
        }
        adminInput?.focus();
        return;
      }
      type DialogWithForm = HTMLDialogElement & { _targetForm?: HTMLFormElement };
      const form = (dialog as DialogWithForm)._targetForm;
      if (!form) {
        dialog.close();
        return;
      }
      ensureHidden(form, "new_password").value = newVal;
      ensureHidden(form, "password").value = adminVal;
      dialog.close();
      if (newInput) newInput.value = "";
      if (adminInput) adminInput.value = "";
      form.requestSubmit();
    };
    accept?.addEventListener("click", (e) => {
      e.preventDefault();
      submit();
    });
    for (const inp of [newInput, adminInput]) {
      inp?.addEventListener("keydown", (e) => {
        if (e.key === "Enter") {
          e.preventDefault();
          submit();
        }
      });
    }
  }
}

function ensureHidden(form: HTMLFormElement, name: string): HTMLInputElement {
  let el = form.querySelector<HTMLInputElement>(`input[type="hidden"][name="${name}"]`);
  if (!el) {
    el = document.createElement("input");
    el.type = "hidden";
    el.name = name;
    form.appendChild(el);
  }
  return el;
}

setup();
