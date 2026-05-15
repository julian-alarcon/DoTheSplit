// Global background-scroll lock for native <dialog> modals.
//
// Plain `overflow:hidden` on <html>/<body> works in theory, but when the page
// is scrolled down it visibly jumps to the top while the dialog is open
// (the document loses its scrollable height). We use the standard
// "position: fixed; top: -<scrollY>" trick so the page appears frozen where
// the user left it; on close we restore styles and re-apply the scroll.
//
// A single MutationObserver watches the <body> subtree for any dialog whose
// `open` attribute toggles, so individual components don't need to opt in.

let lockedY = 0;
let lockedPaddingRight = "";
let lockCount = 0;

function scrollbarGutter(): number {
  return Math.max(0, window.innerWidth - document.documentElement.clientWidth);
}

function lock() {
  if (lockCount++ > 0) return;
  lockedY = window.scrollY;
  lockedPaddingRight = document.body.style.paddingRight;
  const gutter = scrollbarGutter();
  const body = document.body;
  body.style.position = "fixed";
  body.style.top = `-${lockedY}px`;
  body.style.left = "0";
  body.style.right = "0";
  body.style.width = "100%";
  if (gutter > 0) {
    // Compensate for the scrollbar disappearing so content doesn't shift.
    body.style.paddingRight = `${gutter}px`;
  }
}

function unlock() {
  if (--lockCount > 0) return;
  lockCount = 0;
  const body = document.body;
  body.style.position = "";
  body.style.top = "";
  body.style.left = "";
  body.style.right = "";
  body.style.width = "";
  body.style.paddingRight = lockedPaddingRight;
  window.scrollTo(0, lockedY);
}

function attach(dialog: HTMLDialogElement): void {
  if (dialog.dataset.scrollLockBound === "1") return;
  dialog.dataset.scrollLockBound = "1";
  if (dialog.open) lock();
  // `cancel` (Escape) is always followed by `close`, so only listen to close
  // to avoid double-decrementing the lock counter.
  dialog.addEventListener("close", unlock);
}

function scan(root: ParentNode): void {
  for (const d of root.querySelectorAll<HTMLDialogElement>("dialog")) attach(d);
}

function init(): void {
  scan(document);
  // Track dialogs added later (Astro pages with islands sometimes inject them).
  new MutationObserver((records) => {
    for (const r of records) {
      for (const node of r.addedNodes) {
        if (!(node instanceof Element)) continue;
        if (node.tagName === "DIALOG") attach(node as HTMLDialogElement);
        else scan(node);
      }
    }
  }).observe(document.body, { childList: true, subtree: true });
  // showModal()/show() set the `open` attribute, but the close event already
  // fires on close() so we only need a special path for the open transition.
  // Patch HTMLDialogElement.prototype to call lock() before showModal opens.
  const proto = HTMLDialogElement.prototype;
  const origShowModal = proto.showModal;
  proto.showModal = function patchedShowModal(this: HTMLDialogElement) {
    attach(this);
    lock();
    return origShowModal.call(this);
  };
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", init);
} else {
  init();
}
