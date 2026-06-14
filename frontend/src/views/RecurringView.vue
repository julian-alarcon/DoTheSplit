<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import {
  deleteRecurring,
  getGroup,
  listCategories,
  listRecurring,
  type Group,
} from "@/composables/useGroups";
import type { components } from "@/lib/api/schema";
import { formatMoney } from "@/lib/currencies";
import AppLayout from "@/components/AppLayout.vue";
import Alert from "@/components/Alert.vue";
import Icon from "@/components/Icon.vue";
import CategoryIcon from "@/components/CategoryIcon.vue";
import MemberAvatar from "@/components/MemberAvatar.vue";
import ConfirmDialog from "@/components/ConfirmDialog.vue";

type RecurringExpense = components["schemas"]["RecurringExpense"];
type Category = components["schemas"]["Category"];

const route = useRoute();
const router = useRouter();
const groupId = computed(() => String(route.params.id));

const group = ref<Group | null>(null);
const templates = ref<RecurringExpense[]>([]);
const categories = ref<Category[]>([]);
const loaded = ref(false);
const error = ref(false);
const confirmId = ref<string | null>(null);

const categoryByID = computed(() => new Map(categories.value.map((c) => [c.id, c])));
const memberByID = computed(
  () => new Map((group.value?.members ?? []).map((m) => [m.user_id, m])),
);
const nameByID = computed(
  () => new Map((group.value?.members ?? []).map((m) => [m.user_id, m.display_name])),
);

const cadenceLabels: Record<string, string> = {
  daily: "Daily",
  weekly: "Weekly",
  biweekly: "Every 2 weeks",
  monthly: "Monthly",
  yearly: "Yearly",
};
const monthFmt = new Intl.DateTimeFormat(undefined, { month: "short" });
const dayFmt = new Intl.DateTimeFormat(undefined, { day: "2-digit" });

async function onConfirmStop() {
  const id = confirmId.value;
  confirmId.value = null;
  if (!id) return;
  const res = await deleteRecurring(id);
  if (!res.ok) {
    error.value = true;
    return;
  }
  templates.value = templates.value.filter((t) => t.id !== id);
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
  const [tpls, cats] = await Promise.all([listRecurring(target), listCategories()]);
  if (groupId.value !== target) return;
  templates.value = tpls;
  categories.value = cats;
  loaded.value = true;
}

onMounted(load);
// vue-router reuses this instance when only :id changes; reload on id change.
watch(groupId, load);
</script>

<template>
  <AppLayout v-if="group" :back="{ to: `/groups/${groupId}`, label: group.name }">
    <h1 class="title">Recurring expenses</h1>

    <Alert v-if="error" tone="error" class="banner">Could not stop the template. Try again.</Alert>

    <div v-if="loaded && templates.length === 0" class="empty">
      <h2 class="empty-title">No recurring expenses yet</h2>
      <p class="muted">
        Add a regular expense in
        <RouterLink :to="`/groups/${groupId}`" class="link">{{ group.name }}</RouterLink>
        and pick a repeat cadence in the date picker to create one.
      </p>
    </div>

    <template v-else>
      <p class="lead">
        Stopping a template only cancels future runs. Expenses already created from it stay in place.
      </p>
      <ul class="list">
        <li v-for="t in templates" :key="t.id" class="card">
          <div class="row">
            <div class="row-left">
              <span class="when" :title="categoryByID.get(t.category_id)?.label ?? ''">
                <CategoryIcon
                  :slug="categoryByID.get(t.category_id)?.slug"
                  :group-label="categoryByID.get(t.category_id)?.group_label"
                  :size="28"
                />
                <span class="when-label">Next run</span>
                <span class="when-month">{{ monthFmt.format(new Date(t.next_run_at)) }}</span>
                <span class="when-day">{{ dayFmt.format(new Date(t.next_run_at)) }}</span>
              </span>
              <div class="info">
                <div class="info-title">
                  <span class="desc">{{ t.description }}</span>
                  <span class="cadence">
                    <Icon name="arrows-rotate" :size="11" />
                    {{ cadenceLabels[t.cadence] ?? t.cadence }}
                  </span>
                </div>
                <div class="info-sub">
                  <span>Paid by</span>
                  <MemberAvatar
                    :user-id="t.payer_id"
                    :display-name="nameByID.get(t.payer_id) ?? '?'"
                    :has-avatar="memberByID.get(t.payer_id)?.has_avatar"
                    :avatar-updated-at="memberByID.get(t.payer_id)?.avatar_updated_at"
                    :size="16"
                  />
                  <span>{{ nameByID.get(t.payer_id) ?? "?" }}</span>
                </div>
              </div>
            </div>
            <div class="row-right">
              <span class="amount">{{ formatMoney(t.amount_cents, t.currency) }}</span>
              <button type="button" class="btn-danger-outline btn-sm" @click="confirmId = t.id">
                <Icon name="circle-stop" />
                <span>Stop</span>
              </button>
            </div>
          </div>
        </li>
      </ul>
    </template>

    <ConfirmDialog
      :open="confirmId !== null"
      title="Stop this recurring template?"
      message="Past expenses already created from this template stay in place. Only future runs are cancelled."
      confirm-label="Stop template"
      confirm-icon="circle-stop"
      @update:open="(v) => { if (!v) confirmId = null; }"
      @confirm="onConfirmStop"
    />
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
.lead {
  margin-bottom: 0.75rem;
  font-size: 0.875rem;
  color: var(--muted-foreground);
}
.muted {
  color: var(--muted-foreground);
}
.link {
  text-decoration: underline;
}
.empty {
  border-radius: 0.375rem;
  border: 1px solid var(--border);
  background: var(--card);
  padding: 0.75rem;
  font-size: 0.875rem;
}
.empty-title {
  margin-bottom: 0.5rem;
  font-weight: 500;
}
.list {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  list-style: none;
}
.card {
  border-radius: 0.375rem;
  border: 1px solid var(--border);
  background: var(--card);
  padding: 0.75rem;
}
.row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
}
.row-left {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.75rem;
}
.when {
  flex-shrink: 0;
  display: inline-flex;
  width: 1.75rem;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  line-height: 1;
  text-align: center;
}
.when-label,
.when-month {
  margin-top: 0.125rem;
  font-size: 8px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--muted-foreground);
}
.when-day {
  font-size: 0.75rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  color: var(--muted-foreground);
}
.info {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 0.25rem;
}
.info-title {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
}
.desc {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 500;
}
.cadence {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  border-radius: 9999px;
  background: var(--muted);
  padding: 0.125rem 0.5rem;
  font-size: 0.75rem;
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--muted-foreground);
}
.info-sub {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.375rem;
  font-size: 0.75rem;
  color: var(--muted-foreground);
}
.row-right {
  display: flex;
  flex-shrink: 0;
  flex-direction: column;
  align-items: flex-end;
  gap: 0.375rem;
}
.amount {
  font-family: var(--font-mono);
  font-size: 0.875rem;
}
</style>
