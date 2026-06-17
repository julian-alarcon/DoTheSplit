<script setup lang="ts">
import { RouterLink } from "vue-router";
import AppLayout from "@/components/AppLayout.vue";
import Icon from "@/components/Icon.vue";

const cards = [
  { to: "/admin/users", icon: "users", title: "Users", desc: "List, add, remove, reset passwords." },
  { to: "/admin/groups", icon: "layer-group", title: "All groups", desc: "Inspect every group; remove unwanted ones." },
  { to: "/admin/smtp", icon: "envelope", title: "SMTP", desc: "Configure outbound email (password encrypted at rest)." },
  { to: "/admin/audit", icon: "clock-rotate-left", title: "Audit log", desc: "Every admin action and failed step-up attempt." },
];
</script>

<template>
  <AppLayout :back="{ to: '/groups', label: 'Groups' }">
    <h1 class="mb-1 text-2xl font-semibold">Admin</h1>
    <p class="mb-6 text-sm text-muted-foreground">Instance-wide management. Destructive actions require re-entering your password.</p>
    <div class="grid gap-3">
      <RouterLink
        v-for="c in cards"
        :key="c.to"
        :to="c.to"
        class="flex items-center gap-3 rounded-lg border border-border bg-card px-4 py-3 transition-colors hover:bg-hover-surface"
      >
        <Icon :name="c.icon" :size="20" />
        <div>
          <div class="font-medium">{{ c.title }}</div>
          <div class="text-xs text-muted-foreground">{{ c.desc }}</div>
        </div>
      </RouterLink>
    </div>
  </AppLayout>
</template>
