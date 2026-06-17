<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useAuth } from "@/composables/useAuth";
import { getGroup, listActivity, type ActivityItem, type Group } from "@/composables/useGroups";
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
  const res = await listActivity(target);
  if (groupId.value !== target) return;
  items.value = res.items;
  nextCursor.value = res.nextCursor;
  loaded.value = true;
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
      />
    </ul>
    <div v-if="nextCursor" class="mt-4 flex justify-center">
      <button type="button" class="btn-secondary btn-sm" :disabled="loadingMore" @click="onLoadMore">
        Load more
      </button>
    </div>
  </AppLayout>
</template>
