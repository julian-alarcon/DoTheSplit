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
import { useNetworkStatus } from "@/composables/useNetworkStatus";
import Alert from "@/components/Alert.vue";
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
const { offline } = useNetworkStatus();
const router = useRouter();
const user = computed(() => state.user);

// Build identity, baked in at image-build time via Vite define (see
// vite.config.ts). Falls back to "dev" for local dev.
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
  <div class="flex min-h-[100dvh] flex-col">
    <a href="#main" class="skip-link">Skip to content</a>
    <header class="relative z-50 border-b border-border bg-[color-mix(in_oklch,var(--card)_70%,transparent)] backdrop-blur-[8px]">
      <div class="mx-auto flex max-w-6xl items-center justify-between pt-[max(0.75rem,env(safe-area-inset-top))] pr-[max(1rem,env(safe-area-inset-right))] pb-3 pl-[max(1rem,env(safe-area-inset-left))]">
        <RouterLink :to="user ? '/groups' : '/'" class="flex items-center gap-2 text-lg font-semibold">
          <img src="/logo.svg" alt="" aria-hidden="true" width="40" height="40" class="-my-1.5 h-10 w-10" />
          <span>DoTheSplit</span>
        </RouterLink>

        <div v-if="user" class="flex items-center gap-2 sm:gap-3">
          <RouterLink
            v-if="user.is_admin"
            to="/admin"
            class="inline-flex items-center gap-1.5 rounded-md border border-amber-400/60 bg-amber-50 px-2 py-1.5 text-xs font-medium text-amber-900 hover:bg-amber-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-amber-500 dark:border-amber-500/60 dark:bg-amber-950/40 dark:text-amber-200 dark:hover:bg-amber-900/40"
            aria-label="Admin"
          >
            <Icon name="shield-halved" />
            <span class="hidden sm:inline">Admin</span>
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
              <span class="hidden text-sm sm:inline">{{ user.display_name }}</span>
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

        <nav v-else class="flex gap-4 text-sm [&_a:hover]:underline">
          <RouterLink to="/login">Log in</RouterLink>
          <RouterLink to="/register">Register</RouterLink>
        </nav>
      </div>
    </header>

    <div v-if="back" class="mx-auto w-full max-w-6xl px-2 pt-3">
      <RouterLink :to="back.to" class="btn-secondary btn-sm self-start">
        <Icon name="arrow-left-long" />
        <span>Back to {{ back.label }}</span>
      </RouterLink>
    </div>

    <div v-if="offline" class="mx-auto w-full px-2 pt-3" :class="wide ? 'max-w-6xl' : 'max-w-3xl'">
      <Alert tone="info">
        You're offline. Showing saved data; changes can't be saved right now.
      </Alert>
    </div>

    <main id="main" class="mx-auto w-full flex-1 p-2" :class="wide ? 'max-w-6xl' : 'max-w-3xl'">
      <slot />
    </main>

    <footer class="border-t border-border bg-card pt-3 pb-[max(0.75rem,env(safe-area-inset-bottom))] text-xs text-muted-foreground">
      <div class="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-x-4 gap-y-2 pl-[max(1rem,env(safe-area-inset-left))] pr-[max(1rem,env(safe-area-inset-right))]">
        <span>
          DoTheSplit &middot;
          <a
            v-if="isReleasedVersion"
            :href="`https://github.com/julian-alarcon/dothesplit/releases/tag/v${buildVersion}`"
            class="[font-family:var(--font-mono)] hover:underline"
            rel="noopener noreferrer"
            target="_blank"
            >v{{ buildVersion }}</a
          >
          <code v-else class="[font-family:var(--font-mono)]">{{ buildVersion }}</code>
        </span>
        <ThemeSwitcher />
      </div>
    </footer>
  </div>
</template>
