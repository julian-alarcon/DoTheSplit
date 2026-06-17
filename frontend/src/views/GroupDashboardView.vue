<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useAuth } from "@/composables/useAuth";
import { getGroup, type Group } from "@/composables/useGroups";
import {
  createExpense,
  loadDashboard,
  loadMoreTransactions,
  type Category,
  type RecurringExpense,
  type SimplifiedDebt,
  type TransactionItem,
} from "@/composables/useDashboard";
import { moneyFormatter, formatMoney } from "@/lib/currencies";
import { shortName } from "@/lib/short-name";
import { withMonthHeaders } from "@/lib/transaction-month";
import AppLayout from "@/components/AppLayout.vue";
import Alert from "@/components/Alert.vue";
import Icon from "@/components/Icon.vue";
import CategoryIcon from "@/components/CategoryIcon.vue";
import MemberAvatar from "@/components/MemberAvatar.vue";
import CurrencyInput from "@/components/CurrencyInput.vue";
import DatePicker from "@/components/DatePicker.vue";
import SplitEditor, { type SplitPayload } from "@/components/SplitEditor.vue";

const route = useRoute();
const router = useRouter();
const { state } = useAuth();

const groupId = computed(() => String(route.params.id));
const viewerId = computed(() => state.user?.id ?? "");

const group = ref<Group | null>(null);
const simplified = ref<SimplifiedDebt[]>([]);
const categories = ref<Category[]>([]);
const recurring = ref<RecurringExpense[]>([]);
const transactions = ref<TransactionItem[]>([]);
const nextCursor = ref("");
const loaded = ref(false);
const loadError = ref(false);
const formError = ref<string | null>(null);

const inviteFailed = computed(() => {
  const raw = route.query.invite_failed;
  return typeof raw === "string" && /^\d+$/.test(raw) ? Number(raw) : 0;
});

const currency = computed(() => group.value?.default_currency ?? "EUR");
const members = computed(() => group.value?.members ?? []);
const memberByID = computed(
  () => new Map(members.value.map((m) => [m.user_id, m])),
);
const nameByID = computed(
  () => new Map(members.value.map((m) => [m.user_id, m.display_name])),
);
const categoryByID = computed(
  () => new Map(categories.value.map((c) => [c.id, c])),
);

const cadenceLabels: Record<string, string> = {
  daily: "daily",
  weekly: "weekly",
  biweekly: "every 2 weeks",
  monthly: "monthly",
  yearly: "yearly",
};

const localeFmtMonth = new Intl.DateTimeFormat(undefined, { month: "short" });
const localeFmtDay = new Intl.DateTimeFormat(undefined, { day: "2-digit" });

// --- Balances (viewer perspective) ------------------------------------------
type ViewerDebt = { otherID: string; amountCents: number; theyOweMe: boolean };
const myDebts = computed<ViewerDebt[]>(() =>
  simplified.value
    .filter(
      (d) =>
        d.from_user_id === viewerId.value || d.to_user_id === viewerId.value,
    )
    .map((d) => ({
      otherID:
        d.from_user_id === viewerId.value ? d.to_user_id : d.from_user_id,
      amountCents: d.amount_cents,
      theyOweMe: d.to_user_id === viewerId.value,
    })),
);

const settledMessages = [
  "All settled up. Nice!",
  "Squeaky clean. No debts here!",
  "You're square with everyone!",
  "Zero balances, zero worries.",
  "Debt-free and loving it!",
  "Everyone's even. High five!",
  "Nothing owed, nothing owing.",
  "Balances at zero. Treat yourself!",
  "All paid up. Smooth sailing!",
  "Perfectly balanced, as all things should be.",
];
const settledMessage =
  settledMessages[Math.floor(Math.random() * settledMessages.length)];

const settledIcons = [
  "champagne-glasses",
  "face-smile",
  "thumbs-up",
  "star",
  "heart",
  "trophy",
  "hand-peace",
  "mug-hot",
  "check-double",
  "face-grin-stars",
];
const settledIcon =
  settledIcons[Math.floor(Math.random() * settledIcons.length)];

// --- Transaction feed --------------------------------------------------------
const feedRows = computed(() =>
  withMonthHeaders(
    transactions.value,
    (item) => item.expense?.incurred_at ?? item.settlement?.settled_at ?? "",
    typeof navigator !== "undefined" ? navigator.language : "en-US",
  ),
);

type ViewerStake =
  | { kind: "lent"; cents: number }
  | { kind: "owes"; cents: number }
  | { kind: "none" };
function viewerStake(e: {
  payer_id: string;
  amount_cents: number;
  splits: Array<{ user_id: string; share_cents: number }>;
}): ViewerStake {
  const me = viewerId.value;
  if (!me) return { kind: "none" };
  const myShare = e.splits.find((s) => s.user_id === me)?.share_cents ?? 0;
  const iPaid = e.payer_id === me;
  const involved = iPaid || e.splits.some((s) => s.user_id === me);
  if (!involved) return { kind: "none" };
  if (iPaid) {
    const lent = e.amount_cents - myShare;
    return lent > 0 ? { kind: "lent", cents: lent } : { kind: "none" };
  }
  return myShare > 0 ? { kind: "owes", cents: myShare } : { kind: "none" };
}

function dayParts(iso: string) {
  const d = new Date(iso);
  return { month: localeFmtMonth.format(d), day: localeFmtDay.format(d) };
}
function rowMoney(cents: number, cur: string) {
  return moneyFormatter(cur).format(cents / 100);
}

const loadingMore = ref(false);
async function onLoadMore() {
  if (!nextCursor.value || loadingMore.value) return;
  loadingMore.value = true;
  const res = await loadMoreTransactions(groupId.value, nextCursor.value);
  transactions.value = [...transactions.value, ...res.items];
  nextCursor.value = res.nextCursor;
  loadingMore.value = false;
}

// --- Add-expense form --------------------------------------------------------
const defaultCategory = computed(
  () => categories.value.find((c) => c.slug === "other") ?? categories.value[0],
);

const form = ref({
  description: "",
  amountCents: 0,
  payerId: "",
  categoryId: "",
  incurredAt: new Date().toISOString().slice(0, 10),
  cadence: "",
  notes: "",
});
const split = ref<SplitPayload | null>(null);
const submitting = ref(false);
const categoryDialog = ref<HTMLDialogElement | null>(null);

const selectedCategory = computed(() =>
  categoryByID.value.get(form.value.categoryId),
);

function openCategoryPicker() {
  categoryDialog.value?.showModal();
}
function pickCategory(c: Category) {
  form.value.categoryId = c.id;
  categoryDialog.value?.close();
}

function isNewGroupRow(i: number): boolean {
  return (
    i === 0 ||
    categories.value[i - 1].group_label !== categories.value[i].group_label
  );
}

async function onSubmit() {
  formError.value = null;
  if (!split.value) return;
  submitting.value = true;
  const res = await createExpense(groupId.value, {
    description: form.value.description,
    amountCents: form.value.amountCents,
    payerId: form.value.payerId,
    categoryId: form.value.categoryId || undefined,
    notes: form.value.notes || undefined,
    incurredAt: form.value.incurredAt,
    cadence: form.value.cadence || undefined,
    split: split.value,
  });
  submitting.value = false;
  if (res.ok) {
    await reload();
    resetForm();
  } else {
    formError.value =
      res.code === "recurring_create"
        ? "The expense was saved, but the recurring template could not be created."
        : "Could not add the expense. Please check your input and try again.";
  }
}

function resetForm() {
  form.value.description = "";
  form.value.amountCents = 0;
  form.value.categoryId = defaultCategory.value?.id ?? "";
  form.value.cadence = "";
  form.value.notes = "";
  form.value.incurredAt = new Date().toISOString().slice(0, 10);
}

const weekStart = computed<0 | 1>(() => (state.user?.week_start === 0 ? 0 : 1));

async function reload() {
  // Capture the group the load was started for; if the user navigates to a
  // different group before the request resolves, drop the stale result.
  const target = groupId.value;
  const data = await loadDashboard(target);
  if (groupId.value !== target) return;
  loadError.value = data.error;
  simplified.value = data.simplified;
  categories.value = data.categories;
  recurring.value = data.recurring;
  transactions.value = data.transactions;
  nextCursor.value = data.nextCursor;
}

async function loadGroup() {
  loaded.value = false;
  const target = groupId.value;
  const { group: g, error } = await getGroup(target);
  if (groupId.value !== target) return;
  if (!g) {
    // A real fetch failure shouldn't bounce the user to /groups; only a
    // confirmed "group not found" (successful list without this id) does.
    if (error) {
      loadError.value = true;
      loaded.value = true;
      return;
    }
    await router.replace("/groups");
    return;
  }
  group.value = g;
  await reload();
  form.value.payerId = viewerId.value || members.value[0]?.user_id || "";
  form.value.categoryId = defaultCategory.value?.id ?? "";
  loaded.value = true;
}

onMounted(loadGroup);

// vue-router reuses this component instance when only :id changes (no remount),
// so reload when the resolved group id changes. Watching the computed fires
// after navigation settles, so groupId.value is already the new id.
watch(groupId, loadGroup);
</script>

<template>
  <AppLayout v-if="group" wide :back="{ to: '/groups', label: 'Groups' }">
    <div class="mb-2 flex items-baseline justify-between gap-3">
      <h1 class="min-w-0 flex-1 truncate text-2xl font-semibold">
        {{ group.name }}
      </h1>
      <div class="flex shrink-0 items-center gap-2">
        <RouterLink
          :to="`/groups/${groupId}/recurring`"
          class="btn-secondary btn-sm h-9"
          :aria-label="`Recurring expenses (${recurring.length})`"
          title="Recurring expenses"
        >
          <Icon name="arrows-rotate" />
          <span class="hidden sm:inline">Recurring</span>
          <span v-if="recurring.length > 0" class="tabular-nums"
            >({{ recurring.length }})</span
          >
        </RouterLink>
        <RouterLink
          :to="`/groups/${groupId}/settings`"
          class="btn-secondary btn-sm h-9"
          aria-label="Group settings"
          title="Group settings"
        >
          <Icon name="gear" />
          <span class="hidden sm:inline">Group settings</span>
        </RouterLink>
      </div>
    </div>
    <p class="mb-3 text-sm text-subtle-foreground">
      {{ members.length }} member{{ members.length === 1 ? "" : "s" }} · default
      currency {{ currency }}
    </p>

    <Alert
      v-if="formError"
      tone="error"
      class="mb-4 flex flex-wrap items-center justify-between gap-2"
      >{{ formError }}</Alert
    >
    <Alert
      v-if="inviteFailed > 0"
      tone="info"
      class="mb-4 flex flex-wrap items-center justify-between gap-2"
    >
      <span>
        {{
          inviteFailed === 1
            ? "1 invite was skipped: that email isn't a registered user yet."
            : `${inviteFailed} invites were skipped: those emails aren't registered users yet.`
        }}
      </span>
      <RouterLink
        :to="`/groups/${groupId}/settings`"
        class="font-medium underline"
        >Add members</RouterLink
      >
    </Alert>

    <div
      class="grid grid-cols-[minmax(0,1fr)] items-start gap-3 lg:grid-cols-[20.5rem_minmax(0,1fr)_20.5rem]"
    >
      <!-- Balances -->
      <section class="lg:order-1">
        <div class="flex items-start justify-between gap-3 text-sm">
          <div class="flex min-w-0 flex-1 flex-col gap-2">
            <p
              v-if="myDebts.length === 0"
              class="flex items-center gap-2 text-emerald-700 dark:text-emerald-400"
            >
              <Icon :name="settledIcon" :size="16" />
              <span class="font-medium">{{ settledMessage }}</span>
            </p>
            <ul v-else class="flex list-none flex-col gap-2">
              <li
                v-for="(d, i) in myDebts"
                :key="i"
                class="flex min-w-0 flex-wrap items-center gap-1.5"
              >
                <template v-if="d.theyOweMe">
                  <span class="flex min-w-0 items-center gap-1.5">
                    <MemberAvatar
                      :user-id="d.otherID"
                      :display-name="nameByID.get(d.otherID) ?? '?'"
                      :has-avatar="memberByID.get(d.otherID)?.has_avatar"
                      :avatar-updated-at="
                        memberByID.get(d.otherID)?.avatar_updated_at
                      "
                      :size="18"
                    />
                    <span class="truncate">{{
                      shortName(nameByID.get(d.otherID))
                    }}</span>
                  </span>
                  <span class="text-subtle-foreground">owes you</span>
                </template>
                <template v-else>
                  <span class="text-subtle-foreground">You owe</span>
                  <span class="flex min-w-0 items-center gap-1.5">
                    <MemberAvatar
                      :user-id="d.otherID"
                      :display-name="nameByID.get(d.otherID) ?? '?'"
                      :has-avatar="memberByID.get(d.otherID)?.has_avatar"
                      :avatar-updated-at="
                        memberByID.get(d.otherID)?.avatar_updated_at
                      "
                      :size="18"
                    />
                    <span class="truncate">{{
                      shortName(nameByID.get(d.otherID))
                    }}</span>
                  </span>
                </template>
                <span
                  class="[font-family:var(--font-mono)]"
                  :class="
                    d.theyOweMe
                      ? 'text-emerald-700 dark:text-emerald-400'
                      : 'text-red-700 dark:text-red-400'
                  "
                >
                  {{ formatMoney(d.amountCents, currency) }}
                </span>
              </li>
            </ul>
          </div>
          <RouterLink
            :to="`/groups/${groupId}/settle`"
            class="btn-secondary btn-xs shrink-0"
          >
            Settle up
          </RouterLink>
        </div>
      </section>

      <!-- Add expense -->
      <section class="rounded-md border border-border bg-card p-3 lg:order-3">
        <h2 class="mb-3 font-medium">Add expense</h2>
        <form class="flex flex-col gap-4" @submit.prevent="onSubmit">
          <div class="flex items-end gap-2">
            <button
              type="button"
              class="field-category-trigger"
              aria-label="Choose category"
              title="Choose category"
              @click="openCategoryPicker"
            >
              <CategoryIcon
                :slug="selectedCategory?.slug ?? defaultCategory?.slug"
                :group-label="
                  selectedCategory?.group_label ?? defaultCategory?.group_label
                "
                :size="38"
              />
            </button>
            <div class="flex flex-1 flex-col">
              <label class="field">
                <input
                  v-model="form.description"
                  class="field-input"
                  required
                  maxlength="200"
                  placeholder=" "
                />
                <span class="field-label" data-required>What was it?</span>
              </label>
              <p class="field-error">Required</p>
            </div>
          </div>

          <CurrencyInput
            v-model="form.amountCents"
            :label="`Amount (${currency})`"
            :currency="currency"
            required
            error="Enter an amount greater than 0"
          />

          <label class="field-select-row">
            <span>Paid by</span>
            <select v-model="form.payerId" required class="field-select">
              <option v-for="m in members" :key="m.user_id" :value="m.user_id">
                {{ m.display_name }}
              </option>
            </select>
          </label>

          <SplitEditor
            v-model="split"
            v-model:notes="form.notes"
            :members="members"
            :currency="currency"
            :amount-cents="form.amountCents"
            :payer-id="form.payerId"
            :current-user-id="viewerId"
            :default-split="group.default_split"
          />

          <div class="flex items-center justify-end gap-2">
            <DatePicker
              v-model="form.incurredAt"
              v-model:cadence="form.cadence"
              variant="compact"
              show-cadence
              :week-start="weekStart"
            />
            <button type="submit" class="btn-primary" :disabled="submitting">
              Add expense
            </button>
          </div>
        </form>
      </section>

      <!-- Transactions -->
      <section class="relative pt-4 lg:order-2">
        <RouterLink
          v-if="!(loaded && transactions.length === 0)"
          :to="`/groups/${groupId}/activity`"
          class="btn-secondary btn-xs absolute right-0 top-0 z-6"
        >
          <Icon name="clock-rotate-left" :size="12" />
          <span>See activity</span>
        </RouterLink>

        <p
          v-if="loaded && transactions.length === 0"
          class="text-sm text-muted-foreground"
        >
          No expenses or settlements yet.
        </p>

        <ul v-else class="flex list-none flex-col gap-1">
          <template v-for="(row, i) in feedRows" :key="i">
            <li
              v-if="row.kind === 'month-header'"
              class="px-1 pt-3 text-xs font-semibold uppercase tracking-wider text-muted-foreground first:pt-1"
            >
              {{ row.label }}
            </li>
            <li v-else-if="row.item.kind === 'expense' && row.item.expense">
              <RouterLink
                :to="`/groups/${groupId}/expenses/${row.item.expense.id}`"
                class="flex items-stretch justify-between gap-1.5 rounded-md border border-border bg-card px-3 py-1.5 hover:bg-hover-surface focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
              >
                <div class="flex min-w-0 items-center gap-3">
                  <span
                    class="inline-flex w-7 shrink-0 flex-col items-center justify-center leading-none"
                    :title="
                      categoryByID.get(row.item.expense.category_id)?.label ??
                      ''
                    "
                  >
                    <CategoryIcon
                      :slug="
                        categoryByID.get(row.item.expense.category_id)?.slug
                      "
                      :group-label="
                        categoryByID.get(row.item.expense.category_id)
                          ?.group_label
                      "
                      :size="28"
                    />
                    <span
                      class="mt-0.5 text-[8px] font-semibold uppercase tracking-wider text-muted-foreground"
                      >{{ dayParts(row.item.expense.incurred_at).month }}</span
                    >
                    <span
                      class="text-xs font-semibold tabular-nums text-muted-foreground"
                      >{{ dayParts(row.item.expense.incurred_at).day }}</span
                    >
                  </span>
                  <div class="flex min-w-0 flex-col gap-0.5">
                    <div
                      class="flex min-w-0 flex-wrap items-center gap-1.5 font-medium"
                    >
                      <span class="min-w-0 truncate">{{
                        row.item.expense.description
                      }}</span>
                      <span
                        v-if="row.item.cadence"
                        class="inline-flex items-center gap-0.5 rounded-full bg-muted px-1.5 text-[10px] font-medium uppercase tracking-wider text-muted-foreground"
                        :title="`Repeats ${cadenceLabels[row.item.cadence] ?? row.item.cadence}`"
                      >
                        <Icon name="arrows-rotate" :size="9" />
                        {{
                          cadenceLabels[row.item.cadence] ?? row.item.cadence
                        }}
                      </span>
                    </div>
                    <div
                      class="flex min-w-0 flex-wrap items-center gap-1.5 text-[11px] text-muted-foreground"
                    >
                      <span>paid by</span>
                      <MemberAvatar
                        :user-id="row.item.expense.payer_id"
                        :display-name="
                          nameByID.get(row.item.expense.payer_id) ?? '?'
                        "
                        :has-avatar="
                          memberByID.get(row.item.expense.payer_id)?.has_avatar
                        "
                        :avatar-updated-at="
                          memberByID.get(row.item.expense.payer_id)
                            ?.avatar_updated_at
                        "
                        :size="12"
                      />
                      <span class="truncate">{{
                        shortName(nameByID.get(row.item.expense.payer_id))
                      }}</span>
                    </div>
                  </div>
                </div>
                <div
                  class="flex shrink-0 flex-col items-end justify-between gap-1"
                >
                  <span class="text-[11px] tabular-nums">
                    <template
                      v-if="viewerStake(row.item.expense).kind === 'lent'"
                    >
                      <span class="text-emerald-900 dark:text-emerald-200"
                        >you lent
                      </span>
                      <span class="[font-family:var(--font-mono)]">{{
                        rowMoney(
                          (viewerStake(row.item.expense) as { cents: number })
                            .cents,
                          row.item.expense.currency,
                        )
                      }}</span>
                    </template>
                    <template
                      v-else-if="viewerStake(row.item.expense).kind === 'owes'"
                    >
                      <span class="text-amber-700 dark:text-amber-200"
                        >you owe
                      </span>
                      <span class="[font-family:var(--font-mono)]">{{
                        rowMoney(
                          (viewerStake(row.item.expense) as { cents: number })
                            .cents,
                          row.item.expense.currency,
                        )
                      }}</span>
                    </template>
                    <span v-else class="text-subtle-foreground"
                      >not involved</span
                    >
                  </span>
                  <span
                    class="shrink-0 self-center text-lg tabular-nums [font-family:var(--font-mono)]"
                    >{{
                      rowMoney(
                        row.item.expense.amount_cents,
                        row.item.expense.currency,
                      )
                    }}</span
                  >
                </div>
              </RouterLink>
            </li>
            <li v-else-if="row.item.settlement">
              <RouterLink
                :to="`/groups/${groupId}/settlements/${row.item.settlement.id}`"
                class="flex items-stretch justify-between gap-1.5 rounded-md border border-emerald-200 bg-emerald-50 px-3 py-1.5 hover:bg-emerald-100 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-emerald-400 dark:border-emerald-900 dark:bg-emerald-950/40 dark:hover:bg-emerald-950/60 dark:focus-visible:outline-emerald-700"
              >
                <div class="flex min-w-0 items-center gap-3">
                  <span
                    class="inline-flex w-7 shrink-0 flex-col items-center justify-center leading-none"
                    title="Settlement"
                  >
                    <span
                      class="inline-flex h-7 w-7 items-center justify-center rounded-full bg-emerald-100 text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300"
                      ><Icon name="arrow-right" :size="14"
                    /></span>
                    <span
                      class="mt-0.5 text-[8px] font-semibold uppercase tracking-wider text-muted-foreground"
                      >{{
                        dayParts(row.item.settlement.settled_at).month
                      }}</span
                    >
                    <span
                      class="text-xs font-semibold tabular-nums text-muted-foreground"
                      >{{ dayParts(row.item.settlement.settled_at).day }}</span
                    >
                  </span>
                  <div class="flex min-w-0 flex-col gap-0.5">
                    <div
                      class="flex min-w-0 flex-wrap items-center gap-1.5 font-medium"
                    >
                      <MemberAvatar
                        :user-id="row.item.settlement.from_user_id"
                        :display-name="
                          nameByID.get(row.item.settlement.from_user_id) ?? '?'
                        "
                        :has-avatar="
                          memberByID.get(row.item.settlement.from_user_id)
                            ?.has_avatar
                        "
                        :avatar-updated-at="
                          memberByID.get(row.item.settlement.from_user_id)
                            ?.avatar_updated_at
                        "
                        :size="16"
                      />
                      <span class="truncate">{{
                        shortName(
                          nameByID.get(row.item.settlement.from_user_id),
                        )
                      }}</span>
                      <span class="text-subtle-foreground">paid</span>
                      <MemberAvatar
                        :user-id="row.item.settlement.to_user_id"
                        :display-name="
                          nameByID.get(row.item.settlement.to_user_id) ?? '?'
                        "
                        :has-avatar="
                          memberByID.get(row.item.settlement.to_user_id)
                            ?.has_avatar
                        "
                        :avatar-updated-at="
                          memberByID.get(row.item.settlement.to_user_id)
                            ?.avatar_updated_at
                        "
                        :size="16"
                      />
                      <span class="truncate">{{
                        shortName(nameByID.get(row.item.settlement.to_user_id))
                      }}</span>
                    </div>
                    <div
                      class="flex min-w-0 flex-wrap items-center gap-1.5 text-[11px] text-muted-foreground"
                    >
                      <span>settlement</span>
                      <span v-if="row.item.settlement.note" class="truncate"
                        >· {{ row.item.settlement.note }}</span
                      >
                    </div>
                  </div>
                </div>
                <span
                  class="shrink-0 self-center text-lg tabular-nums [font-family:var(--font-mono)]"
                  >{{
                    formatMoney(row.item.settlement.amount_cents, currency)
                  }}</span
                >
              </RouterLink>
            </li>
          </template>
        </ul>

        <div v-if="nextCursor" class="mt-3 flex justify-center">
          <button
            type="button"
            class="btn-secondary btn-sm"
            :disabled="loadingMore"
            @click="onLoadMore"
          >
            Load more
          </button>
        </div>
      </section>
    </div>

    <!-- Category picker dialog -->
    <dialog
      ref="categoryDialog"
      class="fixed inset-0 m-auto w-[calc(100%-2rem)] max-w-96 rounded-md border border-border bg-popover p-0 text-popover-foreground shadow-[0_20px_50px_rgba(0,0,0,0.35)] backdrop:bg-backdrop"
      aria-modal="true"
      aria-label="Choose category"
    >
      <div class="flex flex-col gap-3 p-5">
        <div class="flex items-center justify-between gap-3">
          <h3 class="text-lg font-medium">Choose category</h3>
          <button
            type="button"
            class="cursor-pointer rounded-md px-2 py-1 text-muted-foreground hover:bg-muted"
            aria-label="Close"
            @click="categoryDialog?.close()"
          >
            <Icon name="xmark" :size="14" />
          </button>
        </div>
        <ul class="flex max-h-[60vh] list-none flex-col gap-0.5 overflow-auto">
          <template v-for="(c, i) in categories" :key="c.id">
            <li v-if="isNewGroupRow(i)" class="field-category-group">
              {{ c.group_label }}
            </li>
            <li>
              <button
                type="button"
                class="field-category-option"
                @click="pickCategory(c)"
              >
                <CategoryIcon
                  :slug="c.slug"
                  :group-label="c.group_label"
                  :size="28"
                />
                <span>{{ c.label }}</span>
              </button>
            </li>
          </template>
        </ul>
      </div>
    </dialog>
  </AppLayout>

  <AppLayout
    v-else-if="loaded && loadError"
    :back="{ to: '/groups', label: 'Groups' }"
  >
    <Alert tone="error">
      Couldn't load this group. Check your connection and try again.
    </Alert>
  </AppLayout>
</template>
