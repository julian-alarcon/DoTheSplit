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
    <div class="mx-auto flex max-w-xl flex-col gap-3">
      <h1 class="text-2xl font-semibold">Notifications</h1>
      <p class="text-sm text-muted-foreground">Pick what should reach you. Defaults are off: opt in to anything you want to hear about.</p>

      <Alert v-if="saved" tone="success">Preferences saved.</Alert>
      <Alert v-if="error" tone="error">Could not save your preferences. Try again.</Alert>

      <form
        v-if="loaded"
        class="flex flex-col gap-4 rounded-md border border-border bg-card p-3"
        @submit.prevent="onSubmit"
      >
        <h2 class="text-sm font-medium uppercase tracking-wider text-muted-foreground">Email</h2>
        <label v-for="t in toggles" :key="t.key" class="toggle">
          <input v-model="prefs[t.key]" type="checkbox" class="toggle-input" />
          <span class="toggle-track" aria-hidden="true"></span>
          <span>{{ t.label }}</span>
        </label>
        <div class="flex justify-end pt-2">
          <button type="submit" class="btn-primary">Save preferences</button>
        </div>
      </form>
    </div>
  </AppLayout>
</template>
