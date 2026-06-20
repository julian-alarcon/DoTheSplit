// Theme boot. MUST run synchronously in <head> before first paint, otherwise
// the browser paints the light :root default and flips to the stored
// dark/high-contrast theme (flash-of-wrong-theme). The strict CSP forbids
// inline scripts (script-src 'self'), so this is a same-origin classic script
// served by the Go binary at /theme-boot.js and referenced without defer/async.
// Default is "dark" when nothing is stored.
(function () {
  try {
    var t = localStorage.getItem("dts_theme");
    document.documentElement.dataset.theme =
      t === "light" || t === "dark" || t === "high-contrast" ? t : "dark";
  } catch (e) {
    document.documentElement.dataset.theme = "dark";
  }
})();
