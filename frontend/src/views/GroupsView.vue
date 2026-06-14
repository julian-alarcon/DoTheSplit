<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { RouterLink } from "vue-router";
import { useAuth } from "@/composables/useAuth";
import { listGroups, type Group } from "@/composables/useGroups";
import AppLayout from "@/components/AppLayout.vue";

const { state } = useAuth();
const myId = computed(() => state.user?.id ?? "");

const groups = ref<Group[]>([]);
const loaded = ref(false);

function ownerName(g: Group): string {
  const owner = g.members.find((m) => m.user_id === g.created_by);
  return owner?.display_name ?? "Unknown";
}

onMounted(async () => {
  groups.value = await listGroups();
  loaded.value = true;
});
</script>

<template>
  <AppLayout>
    <img src="/logo.svg" alt="" aria-hidden="true" class="watermark" />
    <div class="head">
      <h1 class="title">Your groups</h1>
      <RouterLink to="/groups/new" class="btn-primary btn-sm new-btn">New group</RouterLink>
    </div>

    <p v-if="loaded && groups.length === 0" class="empty">
      No groups yet. <RouterLink to="/groups/new" class="link">Create one</RouterLink>.
    </p>

    <ul v-else class="list">
      <li v-for="g in groups" :key="g.id">
        <RouterLink :to="`/groups/${g.id}`" class="row">
          <span class="row-main">
            <span class="row-name">{{ g.name }}</span>
            <span class="row-sub">
              Created by {{ g.created_by === myId ? "you" : ownerName(g) }}
            </span>
          </span>
          <span class="row-meta">
            {{ g.members.length }} member{{ g.members.length === 1 ? "" : "s" }} ·
            {{ g.default_currency }}
          </span>
        </RouterLink>
      </li>
    </ul>

    <div class="import-row">
      <RouterLink to="/import" class="btn-secondary btn-sm">Import group</RouterLink>
    </div>
  </AppLayout>
</template>

<style scoped>
.watermark {
  pointer-events: none;
  position: fixed;
  left: 50%;
  top: 50%;
  z-index: -10;
  width: min(90vw, 640px);
  transform: translate(-50%, -50%);
  opacity: 0.4;
}
:root[data-theme="dark"] .watermark,
:root[data-theme="high-contrast"] .watermark {
  opacity: 0.35;
}
.head {
  margin-bottom: 1rem;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}
.title {
  min-width: 0;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 1.5rem;
  font-weight: 600;
}
.new-btn {
  flex-shrink: 0;
}
.empty {
  color: var(--muted-foreground);
}
.link {
  text-decoration: underline;
}
.list {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  list-style: none;
}
.row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  border-radius: 0.375rem;
  border: 1px solid var(--border);
  background: var(--card);
  padding: 0.5rem 0.75rem;
  transition: background-color 120ms ease;
}
.row:hover {
  background: var(--muted);
}
:root[data-theme="dark"] .row:hover,
:root[data-theme="high-contrast"] .row:hover {
  background: var(--accent);
}
.row:focus-visible {
  outline: 2px solid var(--ring);
  outline-offset: 2px;
}
.row-main {
  display: flex;
  min-width: 0;
  flex-direction: column;
}
.row-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 500;
}
.row-sub {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.75rem;
  color: var(--muted-foreground);
}
.row-meta {
  flex-shrink: 0;
  font-size: 0.875rem;
  color: var(--muted-foreground);
}
.import-row {
  margin-top: 2rem;
  display: flex;
  justify-content: center;
}
</style>
