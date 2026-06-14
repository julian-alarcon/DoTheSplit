<script setup lang="ts">
// Avatar that fetches the bearer-authed PNG and falls back to initials.
// Thin wrapper over Avatar.vue + useAvatarUrl, so callers just pass the member
// fields rather than wiring the blob fetch themselves.
import { toRef } from "vue";
import Avatar from "@/components/Avatar.vue";
import { useAvatarUrl } from "@/composables/useAvatarUrl";

const props = withDefaults(
  defineProps<{
    userId: string;
    displayName: string;
    hasAvatar?: boolean;
    avatarUpdatedAt?: string | null;
    size?: 12 | 16 | 18 | 20 | 24 | 32 | 48 | 56 | 64;
    bordered?: boolean;
  }>(),
  { size: 24, bordered: false },
);

const { url } = useAvatarUrl(
  toRef(props, "userId"),
  toRef(props, "hasAvatar"),
  toRef(() => props.avatarUpdatedAt ?? ""),
);
</script>

<template>
  <Avatar :display-name="displayName" :src="url" :size="size" :bordered="bordered" />
</template>
