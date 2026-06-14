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
    <h1 class="title">Admin</h1>
    <p class="lead">Instance-wide management. Destructive actions require re-entering your password.</p>
    <div class="grid">
      <RouterLink v-for="c in cards" :key="c.to" :to="c.to" class="card">
        <Icon :name="c.icon" :size="20" />
        <div>
          <div class="card-title">{{ c.title }}</div>
          <div class="card-desc">{{ c.desc }}</div>
        </div>
      </RouterLink>
    </div>
  </AppLayout>
</template>

<style scoped>
.title {
  margin-bottom: 0.25rem;
  font-size: 1.5rem;
  font-weight: 600;
}
.lead {
  margin-bottom: 1.5rem;
  font-size: 0.875rem;
  color: var(--muted-foreground);
}
.grid {
  display: grid;
  gap: 0.75rem;
}
.card {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  border-radius: 0.5rem;
  border: 1px solid var(--border);
  background: var(--card);
  padding: 0.75rem 1rem;
  transition: background-color 120ms ease;
}
.card:hover {
  background: var(--muted);
}
:root[data-theme="dark"] .card:hover,
:root[data-theme="high-contrast"] .card:hover {
  background: var(--accent);
}
.card-title {
  font-weight: 500;
}
.card-desc {
  font-size: 0.75rem;
  color: var(--muted-foreground);
}
</style>
