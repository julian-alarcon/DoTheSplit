<script setup lang="ts">
// Category glyph. Maps the category slug to a Font Awesome icon (via
// iconForSlug) and tints it by group via the .cat-group-* classes in
// global.css (slugified group_label). The icon inherits color through
// currentColor. The `data-icon-slug="other"` rotation override also lives in
// global.css.
import { computed } from "vue";
import Icon from "@/components/Icon.vue";
import { iconForSlug } from "@/lib/category-icons";

const props = withDefaults(
  defineProps<{
    slug?: string | null;
    groupLabel?: string;
    size?: number;
  }>(),
  { size: 20 },
);

const iconName = computed(() => iconForSlug(props.slug));
const groupClass = computed(() =>
  props.groupLabel
    ? `cat-group-${props.groupLabel
        .toLowerCase()
        .replace(/[^a-z0-9]+/g, "-")
        .replace(/^-|-$/g, "")}`
    : "",
);
</script>

<template>
  <Icon
    :name="iconName"
    :size="size"
    :class="groupClass"
    :data-icon-slug="slug ?? undefined"
  />
</template>
