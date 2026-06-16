<script setup lang="ts">
// App chrome: header (brand + admin/search/user menu), optional "Back to X"
// row, the width-capped <main> slot, and the footer (build info + theme
// switcher).
//
// Layout caps (CLAUDE.md): default single-column at 768px; opt-in `wide`
// switches to 1152px for the group-dashboard triptych. Below the wide
// breakpoint everything stacks single-column.
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { RouterLink, useRouter } from "vue-router";
import { useAuth } from "@/composables/useAuth";
import MemberAvatar from "@/components/MemberAvatar.vue";
import Icon from "@/components/Icon.vue";
import ThemeSwitcher from "@/components/ThemeSwitcher.vue";

withDefaults(
  defineProps<{
    wide?: boolean;
    back?: { to: string; label: string };
  }>(),
  { wide: false },
);

const { state, logout } = useAuth();
const router = useRouter();
const user = computed(() => state.user);

// Build identity, baked in at image-build time via Vite define (see
// vite.config.ts). Falls back to "dev" for local dev.
const buildCommit = (import.meta.env.VITE_BUILD_COMMIT ?? "dev").slice(0, 12);
const buildVersion = import.meta.env.VITE_BUILD_VERSION ?? "dev";
const isReleasedVersion =
  buildVersion !== "dev" && !buildVersion.includes("-dev");

const menuOpen = ref(false);
const menuRoot = ref<HTMLElement | null>(null);

function toggleMenu() {
  menuOpen.value = !menuOpen.value;
}
function closeMenu() {
  menuOpen.value = false;
}
async function onLogout() {
  closeMenu();
  await logout();
  router.push("/login");
}

function onDocClick(e: MouseEvent) {
  if (menuOpen.value && menuRoot.value && !menuRoot.value.contains(e.target as Node)) {
    closeMenu();
  }
}
function onKeydown(e: KeyboardEvent) {
  if (e.key === "Escape") closeMenu();
}

onMounted(() => {
  document.addEventListener("click", onDocClick);
  document.addEventListener("keydown", onKeydown);
});
onBeforeUnmount(() => {
  document.removeEventListener("click", onDocClick);
  document.removeEventListener("keydown", onKeydown);
});
</script>

<template>
  <div class="shell">
    <a href="#main" class="skip-link">Skip to content</a>
    <header class="hdr">
      <div class="hdr-inner">
        <RouterLink :to="user ? '/groups' : '/'" class="brand">
          <img src="/logo.svg" alt="" aria-hidden="true" width="40" height="40" class="brand-logo" />
          <span>DoTheSplit</span>
        </RouterLink>

        <div v-if="user" class="hdr-actions">
          <RouterLink v-if="user.is_admin" to="/admin" class="admin-link" aria-label="Admin">
            <Icon name="shield-halved" />
            <span class="admin-label">Admin</span>
          </RouterLink>

          <RouterLink to="/search" aria-label="Search" title="Search" class="user-menu-icon-btn">
            <Icon name="magnifying-glass" />
          </RouterLink>

          <div ref="menuRoot" class="user-menu">
            <button
              type="button"
              class="user-menu-trigger"
              aria-haspopup="menu"
              :aria-expanded="menuOpen"
              aria-controls="user-menu-panel"
              @click="toggleMenu"
            >
              <MemberAvatar
                :user-id="user.id"
                :display-name="user.display_name"
                :has-avatar="user.has_avatar"
                :avatar-updated-at="user.avatar_updated_at"
                :size="24"
                bordered
              />
              <span class="user-name">{{ user.display_name }}</span>
              <Icon name="chevron-down" :size="12" />
            </button>
            <div
              v-show="menuOpen"
              id="user-menu-panel"
              class="user-menu-panel"
              role="menu"
              aria-label="Account menu"
            >
              <RouterLink to="/settings" role="menuitem" class="user-menu-item" @click="closeMenu">
                <Icon name="gear" />
                <span>Settings</span>
              </RouterLink>
              <RouterLink to="/about" role="menuitem" class="user-menu-item" @click="closeMenu">
                <Icon name="circle-info" />
                <span>About</span>
              </RouterLink>
              <button type="button" role="menuitem" class="user-menu-item" @click="onLogout">
                <Icon name="right-from-bracket" />
                <span>Logout</span>
              </button>
            </div>
          </div>
        </div>

        <nav v-else class="guest-nav">
          <RouterLink to="/login">Log in</RouterLink>
          <RouterLink to="/register">Register</RouterLink>
        </nav>
      </div>
    </header>

    <div v-if="back" class="back-row">
      <RouterLink :to="back.to" class="btn-secondary btn-sm">
        <Icon name="arrow-left-long" />
        <span>Back to {{ back.label }}</span>
      </RouterLink>
    </div>

    <main id="main" class="main" :class="wide ? 'main-wide' : 'main-default'">
      <slot />
    </main>

    <footer class="ftr">
      <div class="ftr-inner">
        <span>
          DoTheSplit &middot;
          <a
            v-if="isReleasedVersion"
            :href="`https://github.com/julian-alarcon/dothesplit/releases/tag/v${buildVersion}`"
            class="mono link"
            rel="noopener noreferrer"
            target="_blank"
            >v{{ buildVersion }}</a
          >
          <code v-else class="mono">{{ buildVersion }}</code>
          &middot;
          <span v-if="buildCommit === 'dev'">build <code class="mono">dev</code></span>
          <span v-else>
            build
            <a
              :href="`https://github.com/julian-alarcon/dothesplit/commit/${buildCommit}`"
              class="mono link"
              rel="noopener noreferrer"
              target="_blank"
              >{{ buildCommit }}</a
            >
          </span>
        </span>
        <ThemeSwitcher />
      </div>
    </footer>
  </div>
</template>

<style scoped>
.shell {
  display: flex;
  min-height: 100dvh;
  flex-direction: column;
}

.hdr {
  border-bottom: 1px solid var(--border);
  background: color-mix(in oklch, var(--card) 70%, transparent);
  backdrop-filter: blur(8px);
}
.hdr-inner {
  margin-inline: auto;
  display: flex;
  max-width: 72rem;
  align-items: center;
  justify-content: space-between;
  padding: max(0.75rem, env(safe-area-inset-top)) max(1rem, env(safe-area-inset-right))
    0.75rem max(1rem, env(safe-area-inset-left));
}
.brand {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 1.125rem;
  font-weight: 600;
}
.brand-logo {
  height: 2.5rem;
  width: 2.5rem;
  margin-block: -0.375rem;
}
.hdr-actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}
@media (min-width: 640px) {
  .hdr-actions {
    gap: 0.75rem;
  }
}
.admin-link {
  display: inline-flex;
  align-items: center;
  gap: 0.375rem;
  border-radius: 0.375rem;
  border: 1px solid oklch(82.8% 0.189 84.429 / 0.6); /* amber-400/60 */
  background: oklch(98.7% 0.022 95.277); /* amber-50 */
  padding: 0.375rem 0.5rem;
  font-size: 0.75rem;
  font-weight: 500;
  color: oklch(41.4% 0.112 45.904); /* amber-900 */
}
.admin-link:hover {
  background: oklch(96.2% 0.059 95.617); /* amber-100 */
}
.admin-link:focus-visible {
  outline: none;
  box-shadow: 0 0 0 2px oklch(76.9% 0.188 70.08); /* ring-amber-500 */
}
:root[data-theme="dark"] .admin-link,
:root[data-theme="high-contrast"] .admin-link {
  border-color: oklch(76.9% 0.188 70.08 / 0.6); /* amber-500/60 */
  background: oklch(27.9% 0.077 45.635 / 0.4); /* amber-950/40 */
  color: oklch(92.4% 0.12 95.746); /* amber-200 */
}
:root[data-theme="dark"] .admin-link:hover,
:root[data-theme="high-contrast"] .admin-link:hover {
  background: oklch(41.4% 0.112 45.904 / 0.4); /* amber-900/40 */
}
.admin-label {
  display: none;
}
.user-name {
  display: none;
  font-size: 0.875rem;
}
@media (min-width: 640px) {
  .admin-label,
  .user-name {
    display: inline;
  }
}
.guest-nav {
  display: flex;
  gap: 1rem;
  font-size: 0.875rem;
}
.guest-nav a:hover {
  text-decoration: underline;
}

.back-row {
  margin-inline: auto;
  width: 100%;
  max-width: 72rem;
  padding: 0.75rem 0.5rem 0;
}
.back-row .btn-secondary {
  align-self: flex-start;
}

.main {
  margin-inline: auto;
  width: 100%;
  flex: 1;
  padding: 0.5rem;
}
.main-default {
  max-width: 48rem;
}
.main-wide {
  max-width: 72rem;
}

.ftr {
  border-top: 1px solid var(--border);
  background: color-mix(in oklch, var(--card) 70%, transparent);
  padding-top: 0.75rem;
  padding-bottom: max(0.75rem, env(safe-area-inset-bottom));
  font-size: 0.75rem;
  color: var(--muted-foreground);
}
.ftr-inner {
  margin-inline: auto;
  display: flex;
  max-width: 72rem;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem 1rem;
  padding-inline: max(1rem, env(safe-area-inset-left)) max(1rem, env(safe-area-inset-right));
}
.mono {
  font-family: var(--font-mono);
}
.link:hover {
  text-decoration: underline;
}
</style>
