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
    <h1 class="mb-4 text-2xl font-semibold">Recurring expenses</h1>

    <Alert v-if="error" tone="error" class="mb-4">Could not stop the template. Try again.</Alert>

    <div v-if="loaded && templates.length === 0" class="rounded-md border border-border bg-card p-3 text-sm">
      <h2 class="mb-2 font-medium">No recurring expenses yet</h2>
      <p class="text-muted-foreground">
        Add a regular expense in
        <RouterLink :to="`/groups/${groupId}`" class="underline">{{ group.name }}</RouterLink>
        and pick a repeat cadence in the date picker to create one.
      </p>
    </div>

    <template v-else>
      <p class="mb-3 text-sm text-muted-foreground">
        Stopping a template only cancels future runs. Expenses already created from it stay in place.
      </p>
      <ul class="flex list-none flex-col gap-1">
        <li v-for="t in templates" :key="t.id" class="rounded-md border border-border bg-card p-3">
          <div class="flex items-start justify-between gap-3">
            <div class="flex min-w-0 items-center gap-3">
              <span class="flex w-7 flex-shrink-0 flex-col items-center justify-center text-center leading-none" :title="categoryByID.get(t.category_id)?.label ?? ''">
                <CategoryIcon
                  :slug="categoryByID.get(t.category_id)?.slug"
                  :group-label="categoryByID.get(t.category_id)?.group_label"
                  :size="28"
                />
                <span class="mt-0.5 text-[8px] font-semibold uppercase tracking-wider text-muted-foreground">Next run</span>
                <span class="mt-0.5 text-[8px] font-semibold uppercase tracking-wider text-muted-foreground">{{ monthFmt.format(new Date(t.next_run_at)) }}</span>
                <span class="text-xs font-semibold tabular-nums text-muted-foreground">{{ dayFmt.format(new Date(t.next_run_at)) }}</span>
              </span>
              <div class="flex min-w-0 flex-col gap-1">
                <div class="flex flex-wrap items-center gap-2">
                  <span class="truncate font-medium">{{ t.description }}</span>
                  <span class="inline-flex items-center gap-1 rounded-full bg-muted px-2 py-0.5 text-xs font-medium uppercase tracking-wider text-muted-foreground">
                    <Icon name="arrows-rotate" :size="11" />
                    {{ cadenceLabels[t.cadence] ?? t.cadence }}
                  </span>
                </div>
                <div class="flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
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
            <div class="flex flex-shrink-0 flex-col items-end gap-1.5">
              <span class="text-sm [font-family:var(--font-mono)]">{{ formatMoney(t.amount_cents, t.currency) }}</span>
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
