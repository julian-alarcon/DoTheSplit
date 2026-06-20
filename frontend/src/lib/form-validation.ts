// Submit-gated validation cues.
//
// We rely on native HTML constraint validation, but the visible cues (the red
// bottom-line + the sibling .field-error text) must surface only when the user
// attempts the action - not on blur, and not on a field a successful submit
// just cleared. Plain CSS :user-invalid can't express that: it matches on blur
// and keeps matching after the value is wiped.
//
// So the CSS gates every cue behind an ancestor `form[data-validated]` using
// live `:invalid`, and this module toggles that flag from two document-level
// listeners installed once at boot:
//
//   - `invalid` (capture, since it does not bubble): the browser fires it on
//     each invalid control when a submit is attempted while fields are bad.
//     Mark that control's form validated -> cues appear on the action, not on
//     blur (fixes the "shown too early" bug).
//   - `submit`: only fires once the form is valid. Clear the flag so the cues
//     are gone before any in-place resetForm() empties required fields (fixes
//     the "red after a successful add" bug).
//
// Forms unmount on SPA navigation, so each fresh form starts clean.

function onInvalidCapture(event: Event): void {
  const target = event.target as { form?: HTMLFormElement | null } | null;
  target?.form?.setAttribute("data-validated", "");
}

function onSubmit(event: Event): void {
  (event.target as HTMLFormElement | null)?.removeAttribute("data-validated");
}

/** Install the global validation listeners once, at app boot. */
export function installFormValidation(): void {
  document.addEventListener("invalid", onInvalidCapture, true);
  document.addEventListener("submit", onSubmit);
}
