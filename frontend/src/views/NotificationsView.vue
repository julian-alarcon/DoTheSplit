<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useAuth } from "@/composables/useAuth";
import type { components } from "@/lib/api/schema";
import AppLayout from "@/components/AppLayout.vue";
import Alert from "@/components/Alert.vue";

type NotificationPrefs = components["schemas"]["NotificationPrefs"];

const { getNotifications, updateNotifications } = useAuth();

const prefs = ref<NotificationPrefs>({});
const loaded = ref(false);
const saved = ref(false);
const error = ref(false);

const toggles: Array<{ key: keyof NotificationPrefs; label: string }> = [
  { key: "notify_recurring_run", label: "A recurring expense is added to one of my groups" },
  { key: "notify_settlement", label: "A settlement is recorded in one of my groups" },
  { key: "notify_group_added", label: "I am added to a new group" },
];

async function onSubmit() {
  saved.value = false;
  error.value = false;
  const res = await updateNotifications(prefs.value);
  if (res.ok) saved.value = true;
  else error.value = true;
}

onMounted(async () => {
  prefs.value = await getNotifications();
  loaded.value = true;
});
</script>

<template>
  <AppLayout :back="{ to: '/settings', label: 'Settings' }">
    <div class="wrap">
      <h1 class="title">Notifications</h1>
      <p class="lead">Pick what should reach you. Defaults are off: opt in to anything you want to hear about.</p>

      <Alert v-if="saved" tone="success">Preferences saved.</Alert>
      <Alert v-if="error" tone="error">Could not save your preferences. Try again.</Alert>

      <form v-if="loaded" class="form" @submit.prevent="onSubmit">
        <h2 class="section">Email</h2>
        <label v-for="t in toggles" :key="t.key" class="toggle">
          <input v-model="prefs[t.key]" type="checkbox" class="toggle-input" />
          <span class="toggle-track" aria-hidden="true"></span>
          <span>{{ t.label }}</span>
        </label>
        <div class="right">
          <button type="submit" class="btn-primary">Save preferences</button>
        </div>
      </form>
    </div>
  </AppLayout>
</template>

<style scoped>
.wrap {
  margin-inline: auto;
  display: flex;
  max-width: 36rem;
  flex-direction: column;
  gap: 0.75rem;
}
.title {
  font-size: 1.5rem;
  font-weight: 600;
}
.lead {
  font-size: 0.875rem;
  color: var(--muted-foreground);
}
.form {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  border-radius: 0.375rem;
  border: 1px solid var(--border);
  background: var(--card);
  padding: 0.75rem;
}
.section {
  font-size: 0.875rem;
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--muted-foreground);
}
.right {
  display: flex;
  justify-content: flex-end;
  padding-top: 0.5rem;
}
</style>
