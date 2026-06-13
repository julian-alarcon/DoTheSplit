<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { RouterLink } from "vue-router";
import { useAuth } from "@/composables/useAuth";
import { ADMIN_PAGE, createUser, listUsers, type AdminUser } from "@/composables/useAdmin";
import AppLayout from "@/components/AppLayout.vue";
import Alert from "@/components/Alert.vue";
import MemberAvatar from "@/components/MemberAvatar.vue";

const { state } = useAuth();
const myId = computed(() => state.user?.id ?? "");

const offset = ref(0);
const includeDeleted = ref(false);
const items = ref<AdminUser[]>([]);
const total = ref(0);

const okMsg = ref<string | null>(null);
const errMsg = ref<string | null>(null);

const form = ref({ email: "", display_name: "", role: "user" as "user" | "admin" });

function fmtDate(s: string) {
  return new Date(s).toISOString().slice(0, 10);
}

async function load() {
  const res = await listUsers(offset.value, includeDeleted.value);
  items.value = res.items;
  total.value = res.total;
}

async function onCreate() {
  okMsg.value = null;
  errMsg.value = null;
  const res = await createUser(form.value);
  if (res.ok) {
    okMsg.value = "User created.";
    form.value = { email: "", display_name: "", role: "user" };
    await load();
  } else {
    errMsg.value =
      res.code === "smtp"
        ? "Configure SMTP first: invitations are sent by email so the user can set their own password."
        : "Could not create user (email may already be registered).";
  }
}

watch([offset, includeDeleted], load);
onMounted(load);
</script>

<template>
  <AppLayout :back="{ to: '/admin', label: 'Admin' }">
    <h1 class="title">Users</h1>

    <Alert v-if="okMsg" tone="success" class="banner">{{ okMsg }}</Alert>
    <Alert v-if="errMsg" tone="error" class="banner">{{ errMsg }}</Alert>

    <section class="panel">
      <h2 class="panel-title">Add user</h2>
      <p class="hint mb">The user receives an email with a 6-digit code to set their own password.</p>
      <form class="form" @submit.prevent="onCreate">
        <label class="field">
          <input v-model="form.email" type="email" required class="field-input" placeholder=" " />
          <span class="field-label" data-required>Email</span>
        </label>
        <label class="field">
          <input v-model="form.display_name" required maxlength="80" class="field-input" placeholder=" " />
          <span class="field-label" data-required>Display name</span>
        </label>
        <label class="field-select-row">
          <span>Role</span>
          <select v-model="form.role" class="field-select">
            <option value="user">user</option>
            <option value="admin">admin</option>
          </select>
        </label>
        <div class="right"><button type="submit" class="btn-primary">Create user</button></div>
      </form>
    </section>

    <section>
      <div class="list-head">
        <h2 class="panel-title">All users ({{ total }})</h2>
        <label class="toggle">
          <input v-model="includeDeleted" type="checkbox" class="toggle-input" />
          <span class="toggle-track" aria-hidden="true"></span>
          <span>Show deleted</span>
        </label>
      </div>
      <ul class="users">
        <li v-for="u in items" :key="u.id">
          <RouterLink :to="`/admin/users/${u.id}`" class="user-row">
            <MemberAvatar :user-id="u.id" :display-name="u.display_name" :has-avatar="u.has_avatar" :size="32" />
            <div class="user-main">
              <div class="user-name-row">
                <span class="user-name">{{ u.display_name }}</span>
                <span v-if="u.id === myId" class="muted small">(you)</span>
              </div>
              <div class="user-email">{{ u.email ?? "(email scrubbed)" }}</div>
              <div class="user-meta">
                <span class="role-badge">{{ u.role }}</span>
                <span>created {{ fmtDate(u.created_at) }}</span>
                <span v-if="u.deleted_at" class="deleted">deleted {{ fmtDate(u.deleted_at) }}</span>
              </div>
            </div>
            <span class="chevron" aria-hidden="true">›</span>
          </RouterLink>
        </li>
      </ul>
      <nav class="pager">
        <button v-if="offset > 0" type="button" class="link" @click="offset = Math.max(0, offset - ADMIN_PAGE)">← Previous</button>
        <button v-if="offset + ADMIN_PAGE < total" type="button" class="link" @click="offset += ADMIN_PAGE">Next →</button>
      </nav>
    </section>
  </AppLayout>
</template>

<style scoped>
.title {
  margin-bottom: 1rem;
  font-size: 1.5rem;
  font-weight: 600;
}
.banner {
  margin-bottom: 1rem;
}
.panel {
  margin-bottom: 1.5rem;
  border-radius: 0.5rem;
  border: 1px solid var(--border);
  padding: 1rem;
}
.panel-title {
  font-size: 1.125rem;
  font-weight: 500;
}
.form {
  display: grid;
  gap: 0.75rem;
}
.hint {
  font-size: 0.75rem;
  color: var(--muted-foreground);
}
.mb {
  margin-bottom: 0.75rem;
}
.right {
  display: flex;
  justify-content: flex-end;
}
.list-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 0.75rem;
}
.users {
  display: grid;
  gap: 0.5rem;
  list-style: none;
}
.user-row {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  border-radius: 0.375rem;
  border: 1px solid var(--border);
  background: var(--card);
  padding: 0.75rem;
  transition: background-color 120ms ease;
}
.user-row:hover {
  background: var(--muted);
}
:root[data-theme="dark"] .user-row:hover,
:root[data-theme="high-contrast"] .user-row:hover {
  background: var(--accent);
}
.user-main {
  min-width: 0;
  flex: 1;
}
.user-name-row {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 0.5rem;
}
.user-name {
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.user-email {
  font-size: 0.75rem;
  color: var(--muted-foreground);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.user-meta {
  margin-top: 0.25rem;
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  font-size: 0.75rem;
  color: var(--muted-foreground);
}
.role-badge {
  border-radius: 0.25rem;
  background: var(--muted);
  padding: 0.0625rem 0.375rem;
}
.deleted {
  color: var(--destructive);
}
.muted {
  color: var(--muted-foreground);
}
.small {
  font-size: 0.75rem;
}
.chevron {
  margin-left: auto;
  color: var(--muted-foreground);
}
.pager {
  margin-top: 1rem;
  display: flex;
  gap: 1rem;
  font-size: 0.875rem;
}
.link {
  cursor: pointer;
  text-decoration: underline;
  color: inherit;
}
</style>
