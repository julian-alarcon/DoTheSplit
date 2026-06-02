// Click-to-toggle dropdown for the header user menu. Native HTML markup
// (button + role="menu" panel) so SSR is keyboard- and screen-reader-friendly
// before this script boots; this only adds open/close + click-outside +
// Escape handling.

function bind(root: HTMLElement) {
  const trigger = root.querySelector<HTMLButtonElement>("[data-user-menu-trigger]");
  const panel = root.querySelector<HTMLElement>(".user-menu-panel");
  if (!trigger || !panel) return;

  const close = () => {
    if (panel.hidden) return;
    panel.hidden = true;
    trigger.setAttribute("aria-expanded", "false");
  };
  const open = () => {
    if (!panel.hidden) return;
    panel.hidden = false;
    trigger.setAttribute("aria-expanded", "true");
  };

  trigger.addEventListener("click", (e) => {
    e.stopPropagation();
    panel.hidden ? open() : close();
  });

  document.addEventListener("click", (e) => {
    if (panel.hidden) return;
    if (!root.contains(e.target as Node)) close();
  });

  document.addEventListener("keydown", (e) => {
    if (panel.hidden) return;
    if (e.key === "Escape") {
      close();
      trigger.focus();
    }
  });
}

document.querySelectorAll<HTMLElement>("[data-user-menu]").forEach(bind);
