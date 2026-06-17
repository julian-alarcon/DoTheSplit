<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { RouterLink } from "vue-router";
import { useAuth } from "@/composables/useAuth";
import { listGroups, type Group } from "@/composables/useGroups";
import AppLayout from "@/components/AppLayout.vue";
import Alert from "@/components/Alert.vue";

const { state } = useAuth();
const myId = computed(() => state.user?.id ?? "");

const groups = ref<Group[]>([]);
const loaded = ref(false);
const loadError = ref(false);

function ownerName(g: Group): string {
  const owner = g.members.find((m) => m.user_id === g.created_by);
  return owner?.display_name ?? "Unknown";
}

onMounted(async () => {
  const res = await listGroups();
  groups.value = res.groups;
  loadError.value = res.error;
  loaded.value = true;
});
</script>

<template>
  <AppLayout>
    <img
      src="/logo.svg"
      alt=""
      aria-hidden="true"
      class="pointer-events-none fixed left-1/2 top-1/2 -z-10 w-[min(90vw,640px)] -translate-x-1/2 -translate-y-1/2 opacity-40 dark:opacity-35"
    />
    <div class="mb-4 flex items-center justify-between gap-3">
      <h1 class="min-w-0 flex-1 truncate text-2xl font-semibold">Your groups</h1>
      <RouterLink to="/groups/new" class="btn-primary btn-sm flex-shrink-0">New group</RouterLink>
    </div>

    <Alert v-if="loaded && loadError" tone="error">
      Couldn't load your groups. Check your connection and try again.
    </Alert>

    <p v-else-if="loaded && groups.length === 0" class="text-muted-foreground">
      No groups yet. <RouterLink to="/groups/new" class="underline">Create one</RouterLink>.
    </p>

    <ul v-else class="flex list-none flex-col gap-1">
      <li v-for="g in groups" :key="g.id">
        <RouterLink
          :to="`/groups/${g.id}`"
          class="flex items-center justify-between gap-3 rounded-md border border-border bg-card px-3 py-2 transition-colors hover:bg-hover-surface focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
        >
          <span class="flex min-w-0 flex-col">
            <span class="truncate font-medium">{{ g.name }}</span>
            <span class="truncate text-xs text-muted-foreground">
              Created by {{ g.created_by === myId ? "you" : ownerName(g) }}
            </span>
          </span>
          <span class="flex-shrink-0 text-sm text-muted-foreground">
            {{ g.members.length }} member{{ g.members.length === 1 ? "" : "s" }} ·
            {{ g.default_currency }}
          </span>
        </RouterLink>
      </li>
    </ul>

    <div class="mt-8 flex justify-center">
      <RouterLink to="/import" class="btn-secondary btn-sm">Import group</RouterLink>
    </div>
  </AppLayout>
</template>
