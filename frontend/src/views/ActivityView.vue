<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useAuth } from "@/composables/useAuth";
import {
  getGroup,
  listActivity,
  markActivityRead,
  type ActivityItem,
  type Group,
} from "@/composables/useGroups";
import AppLayout from "@/components/AppLayout.vue";
import ActivityRow from "@/components/ActivityRow.vue";

const route = useRoute();
const router = useRouter();
const { state } = useAuth();
const groupId = computed(() => String(route.params.id));
const viewerId = computed(() => state.user?.id ?? "");

const group = ref<Group | null>(null);
const items = ref<ActivityItem[]>([]);
const nextCursor = ref("");
const loaded = ref(false);
const loadingMore = ref(false);

// Unread highlight. Captured from the group's unread_count *before* we mark the
// log read (opening it clears the marker server-side). The count excludes the
// viewer's own actions, so we walk the feed newest-first and flag that many
// non-viewer items rather than just the first N rows (own items are interspersed).
const unreadCount = ref(0);
const unreadIds = computed(() => {
  const ids = new Set<string>();
  let remaining = unreadCount.value;
  for (const item of items.value) {
    if (remaining <= 0) break;
    if (item.actor?.user_id === viewerId.value) continue;
    ids.add(item.id);
    remaining -= 1;
  }
  return ids;
});

async function onLoadMore() {
  if (!nextCursor.value || loadingMore.value) return;
  loadingMore.value = true;
  const res = await listActivity(groupId.value, nextCursor.value);
  items.value = [...items.value, ...res.items];
  nextCursor.value = res.nextCursor;
  loadingMore.value = false;
}

async function load() {
  loaded.value = false;
  const target = groupId.value;
  const { group: g } = await getGroup(target);
  if (groupId.value !== target) return;
  if (!g) {
    await router.replace("/groups");
    return;
  }
  group.value = g;
  unreadCount.value = g.unread_count ?? 0;
  const res = await listActivity(target);
  if (groupId.value !== target) return;
  items.value = res.items;
  nextCursor.value = res.nextCursor;
  loaded.value = true;
  // Opening the log clears the unread badge. Optimistically zero the local
  // group's count so the dashboard reflects it immediately on the way back
  // (the markActivityRead call below is the durable source of truth).
  if (group.value) group.value.unread_count = 0;
  void markActivityRead(target);
}

onMounted(load);
// vue-router reuses this instance when only :id changes; reload on id change.
watch(groupId, load);
</script>

<template>
  <AppLayout v-if="group" :back="{ to: `/groups/${groupId}`, label: group.name }">
    <h1 class="mb-4 text-2xl font-semibold">Activity</h1>
    <p v-if="loaded && items.length === 0" class="text-sm text-muted-foreground">No activity yet.</p>
    <ul v-else class="list-none divide-y divide-border">
      <ActivityRow
        v-for="item in items"
        :key="item.id"
        :item="item"
        :group-id="groupId"
        :viewer-id="viewerId"
        :members="group.members"
        :unread="unreadIds.has(item.id)"
      />
    </ul>
    <div v-if="nextCursor" class="mt-4 flex justify-center">
      <button type="button" class="btn-secondary btn-sm" :disabled="loadingMore" @click="onLoadMore">
        Load more
      </button>
    </div>
  </AppLayout>
</template>
