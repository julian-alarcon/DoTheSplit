export {};

// Step-up password prompt wiring.
//
// Trigger buttons set `data-password-open` to the id of a
// <PasswordPromptDialog id="…">. The trigger MUST live inside the form that
// should be submitted on accept. We open the dialog, focus the password
// input, and on accept inject (or replace) a hidden `password` field on that
// form before requestSubmit().

function setup() {
  const triggers = document.querySelectorAll<HTMLElement>("[data-password-open]");
  for (const trigger of triggers) {
    const dialogId = trigger.dataset.passwordOpen;
    if (!dialogId) continue;
    const dialog = document.getElementById(dialogId) as HTMLDialogElement | null;
    if (!dialog) continue;

    trigger.addEventListener("click", (e) => {
      e.preventDefault();
      const form = trigger.closest("form");
      if (!form) return;
      type DialogWithForm = HTMLDialogElement & { _targetForm?: HTMLFormElement };
      (dialog as DialogWithForm)._targetForm = form;
      // Reset the input + error each open; the dialog is shared by N triggers.
      const input = dialog.querySelector<HTMLInputElement>("[data-password-input]");
      const err = dialog.querySelector<HTMLElement>("[data-password-error]");
      if (input) input.value = "";
      if (err) err.classList.add("hidden");
      dialog.showModal();
      queueMicrotask(() => input?.focus());
    });
  }

  const dialogs = document.querySelectorAll<HTMLDialogElement>("[data-password-dialog]");
  for (const dialog of dialogs) {
    const cancels = dialog.querySelectorAll<HTMLButtonElement>("[data-password-cancel]");
    for (const btn of cancels) {
      btn.addEventListener("click", (e) => {
        e.preventDefault();
        dialog.close();
      });
    }
    const input = dialog.querySelector<HTMLInputElement>("[data-password-input]");
    const err = dialog.querySelector<HTMLElement>("[data-password-error]");
    const accept = dialog.querySelector<HTMLButtonElement>("[data-password-accept]");
    const submit = () => {
      const value = input?.value ?? "";
      if (!value) {
        err?.classList.remove("hidden");
        input?.focus();
        return;
      }
      type DialogWithForm = HTMLDialogElement & { _targetForm?: HTMLFormElement };
      const form = (dialog as DialogWithForm)._targetForm;
      if (!form) {
        dialog.close();
        return;
      }
      // Inject (or replace) a hidden password field on the target form so
      // the existing SSR forwarder picks it up unchanged.
      let hidden = form.querySelector<HTMLInputElement>('input[type="hidden"][name="password"]');
      if (!hidden) {
        hidden = document.createElement("input");
        hidden.type = "hidden";
        hidden.name = "password";
        form.appendChild(hidden);
      }
      hidden.value = value;
      dialog.close();
      // Clear the dialog input so the password doesn't linger in DOM.
      if (input) input.value = "";
      form.requestSubmit();
    };
    accept?.addEventListener("click", (e) => {
      e.preventDefault();
      submit();
    });
    input?.addEventListener("keydown", (e) => {
      if (e.key === "Enter") {
        e.preventDefault();
        submit();
      }
    });
  }
}

setup();
