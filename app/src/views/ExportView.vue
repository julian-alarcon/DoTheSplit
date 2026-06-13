<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { exportCsv, getGroup, type Group } from "@/composables/useGroups";
import AppLayout from "@/components/AppLayout.vue";
import Alert from "@/components/Alert.vue";
import Icon from "@/components/Icon.vue";

const route = useRoute();
const router = useRouter();
const groupId = computed(() => String(route.params.id));

const group = ref<Group | null>(null);
const busy = ref(false);
const error = ref(false);

async function onExport() {
  error.value = false;
  busy.value = true;
  const res = await exportCsv(groupId.value);
  busy.value = false;
  if (!res.ok || !res.blob) {
    error.value = true;
    return;
  }
  // Trigger a client-side download from the fetched blob.
  const url = URL.createObjectURL(res.blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = res.filename ?? "export.csv";
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

onMounted(async () => {
  const g = await getGroup(groupId.value);
  if (!g) {
    await router.replace("/groups");
    return;
  }
  group.value = g;
});
</script>

<template>
  <AppLayout v-if="group" :back="{ to: `/groups/${groupId}/settings`, label: 'Settings' }">
    <h1 class="title">Export CSV</h1>
    <p class="lead">
      Download every expense and settlement in <span class="strong">{{ group.name }}</span> as a CSV file. The
      format is a superset of Splitwise's export, so you can re-import it into either tool.
    </p>
    <Alert v-if="error" tone="error" class="banner">Could not export the group. Try again.</Alert>
    <div class="right">
      <button type="button" class="btn-primary" :disabled="busy" @click="onExport">
        <Icon name="download" /><span>{{ busy ? "Preparing…" : "Export CSV" }}</span>
      </button>
    </div>
  </AppLayout>
</template>

<style scoped>
.title {
  margin-bottom: 0.5rem;
  font-size: 1.5rem;
  font-weight: 600;
}
.lead {
  margin-bottom: 1rem;
  font-size: 0.875rem;
  color: var(--muted-foreground);
}
.strong {
  font-weight: 500;
  color: var(--foreground);
}
.banner {
  margin-bottom: 1rem;
}
.right {
  display: flex;
  justify-content: flex-end;
}
</style>
