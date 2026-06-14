<script setup lang="ts">
// Theme picker. Native radio group styled as a 3-state segmented switch
// (see .theme-seg in global.css). Per-device only; value persists via the
// useTheme composable (localStorage key dts_theme).
import { useTheme, type Theme } from "@/composables/useTheme";
import Icon from "@/components/Icon.vue";

const { theme, setTheme } = useTheme();

const options: ReadonlyArray<{ value: Theme; label: string; icon: string }> = [
  { value: "dark", label: "Dark", icon: "moon" },
  { value: "light", label: "Light", icon: "sun" },
  { value: "high-contrast", label: "High contrast", icon: "circle-half-stroke" },
];
</script>

<template>
  <fieldset class="theme-seg">
    <legend class="sr-only">Theme</legend>
    <label
      v-for="opt in options"
      :key="opt.value"
      class="theme-seg-opt"
      :title="opt.label"
    >
      <input
        type="radio"
        name="theme"
        :value="opt.value"
        :checked="theme === opt.value"
        @change="setTheme(opt.value)"
      />
      <span class="theme-seg-icon"><Icon :name="opt.icon" /></span>
      <span class="sr-only">{{ opt.label }}</span>
    </label>
  </fieldset>
</template>
