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
    <h1 class="mb-4 text-2xl font-semibold">Users</h1>

    <Alert v-if="okMsg" tone="success" class="mb-4">{{ okMsg }}</Alert>
    <Alert v-if="errMsg" tone="error" class="mb-4">{{ errMsg }}</Alert>

    <section class="mb-6 rounded-lg border border-border p-4">
      <h2 class="text-lg font-medium">Add user</h2>
      <p class="mb-3 text-xs text-muted-foreground">The user receives an email with a 6-digit code to set their own password.</p>
      <form class="grid gap-3" @submit.prevent="onCreate">
        <label class="field">
          <input v-model="form.email" type="email" required autocomplete="off" class="field-input" placeholder=" " />
          <span class="field-label" data-required>Email</span>
        </label>
        <label class="field">
          <input v-model="form.display_name" required maxlength="80" autocomplete="off" class="field-input" placeholder=" " />
          <span class="field-label" data-required>Display name</span>
        </label>
        <label class="field-select-row">
          <span>Role</span>
          <select v-model="form.role" class="field-select">
            <option value="user">user</option>
            <option value="admin">admin</option>
          </select>
        </label>
        <div class="flex justify-end"><button type="submit" class="btn-primary">Create user</button></div>
      </form>
    </section>

    <section>
      <div class="mb-3 flex items-center justify-between">
        <h2 class="text-lg font-medium">All users ({{ total }})</h2>
        <label class="toggle">
          <input v-model="includeDeleted" type="checkbox" class="toggle-input" />
          <span class="toggle-track" aria-hidden="true"></span>
          <span>Show deleted</span>
        </label>
      </div>
      <ul class="grid list-none gap-2">
        <li v-for="u in items" :key="u.id">
          <RouterLink
            :to="`/admin/users/${u.id}`"
            class="flex items-center gap-3 rounded-md border border-border bg-card p-3 transition-colors hover:bg-hover-surface"
          >
            <MemberAvatar :user-id="u.id" :display-name="u.display_name" :has-avatar="u.has_avatar" :size="32" />
            <div class="min-w-0 flex-1">
              <div class="flex flex-wrap items-baseline gap-2">
                <span class="truncate font-medium">{{ u.display_name }}</span>
                <span v-if="u.id === myId" class="text-xs text-muted-foreground">(you)</span>
              </div>
              <div class="truncate text-xs text-muted-foreground">{{ u.email ?? "(email scrubbed)" }}</div>
              <div class="mt-1 flex flex-wrap gap-2 text-xs text-muted-foreground">
                <span class="rounded-sm bg-muted px-1.5 py-px">{{ u.role }}</span>
                <span>created {{ fmtDate(u.created_at) }}</span>
                <span v-if="u.deleted_at" class="text-destructive">deleted {{ fmtDate(u.deleted_at) }}</span>
              </div>
            </div>
            <span class="ml-auto text-muted-foreground" aria-hidden="true">›</span>
          </RouterLink>
        </li>
      </ul>
      <nav class="mt-4 flex gap-4 text-sm">
        <button v-if="offset > 0" type="button" class="cursor-pointer text-inherit underline" @click="offset = Math.max(0, offset - ADMIN_PAGE)">← Previous</button>
        <button v-if="offset + ADMIN_PAGE < total" type="button" class="cursor-pointer text-inherit underline" @click="offset += ADMIN_PAGE">Next →</button>
      </nav>
    </section>
  </AppLayout>
</template>
