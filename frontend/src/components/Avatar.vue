<script setup lang="ts">
// Compact avatar block. Renders the server-stored 256x256 PNG (a
// nearest-neighbour upscale of the user's 8x8 client-side pixelation) when a
// resolved `src` is supplied, otherwise a square with 1-2 initials derived
// from the display name.
//
// Sizing comes from the .avatar-N classes in global.css (NOT inline style):
// the strict CSP would silently drop inline style attributes.
//
// The avatar endpoint (/v1/users/:id/avatar) is bearer-authed, so an <img>
// tag can't carry the token directly. Callers fetch the bytes with the typed
// client and pass a blob: URL via `src`; until then the initials fallback
// renders. (Wired in Phase D.)
import { computed } from "vue";

const props = withDefaults(
  defineProps<{
    displayName: string;
    /** Resolved blob: URL of the avatar PNG, when fetched. */
    src?: string | null;
    size?: 12 | 16 | 18 | 20 | 24 | 32 | 48 | 56 | 64;
    bordered?: boolean;
  }>(),
  { size: 24, bordered: false },
);

function initialsOf(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length >= 2)
    return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
  return "??";
}

const initials = computed(() => initialsOf(props.displayName));
const sizeClass = computed(() => `avatar-${props.size}`);
</script>

<template>
  <img
    v-if="src"
    :src="src"
    alt=""
    :width="size"
    :height="size"
    class="avatar avatar-img"
    :class="[sizeClass, { bordered }]"
  />
  <span
    v-else
    role="img"
    :aria-label="displayName"
    class="avatar avatar-initials"
    :class="[sizeClass, { bordered }]"
  >
    {{ initials }}
  </span>
</template>

<style scoped>
.avatar {
  border-radius: 0.25rem;
  vertical-align: middle;
  flex-shrink: 0;
}
.avatar-img {
  display: inline-block;
}
.avatar-initials {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-weight: 600;
  line-height: 1;
}
.bordered {
  border: 1px solid var(--border);
}
</style>
