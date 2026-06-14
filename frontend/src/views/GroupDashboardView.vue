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
    <div class="head">
      <h1 class="title">{{ group.name }}</h1>
      <div class="head-actions">
        <RouterLink
          :to="`/groups/${groupId}/recurring`"
          class="btn-secondary btn-sm head-btn"
          :aria-label="`Recurring expenses (${recurring.length})`"
          title="Recurring expenses"
        >
          <Icon name="arrows-rotate" />
          <span class="head-btn-label">Recurring</span>
          <span v-if="recurring.length > 0" class="tnum"
            >({{ recurring.length }})</span
          >
        </RouterLink>
        <RouterLink
          :to="`/groups/${groupId}/settings`"
          class="btn-secondary btn-sm head-btn"
          aria-label="Group settings"
          title="Group settings"
        >
          <Icon name="gear" />
          <span class="head-btn-label">Group settings</span>
        </RouterLink>
      </div>
    </div>
    <p class="subhead">
      {{ members.length }} member{{ members.length === 1 ? "" : "s" }} · default
      currency {{ currency }}
    </p>

    <Alert v-if="formError" tone="error" class="banner">{{ formError }}</Alert>
    <Alert v-if="inviteFailed > 0" tone="info" class="banner">
      <span>
        {{
          inviteFailed === 1
            ? "1 invite was skipped: that email isn't a registered user yet."
            : `${inviteFailed} invites were skipped: those emails aren't registered users yet.`
        }}
      </span>
      <RouterLink :to="`/groups/${groupId}/settings`" class="link"
        >Add members</RouterLink
      >
    </Alert>

    <div class="triptych">
      <!-- Balances -->
      <section class="col-balances">
        <div class="bal-head">
          <div class="bal-list">
            <p v-if="myDebts.length === 0" class="settled">
              <Icon :name="settledIcon" :size="16" />
              <span class="settled-msg">{{ settledMessage }}</span>
            </p>
            <ul v-else class="debts">
              <li v-for="(d, i) in myDebts" :key="i" class="debt">
                <template v-if="d.theyOweMe">
                  <span class="who">
                    <MemberAvatar
                      :user-id="d.otherID"
                      :display-name="nameByID.get(d.otherID) ?? '?'"
                      :has-avatar="memberByID.get(d.otherID)?.has_avatar"
                      :avatar-updated-at="
                        memberByID.get(d.otherID)?.avatar_updated_at
                      "
                      :size="18"
                    />
                    <span class="who-name">{{
                      shortName(nameByID.get(d.otherID))
                    }}</span>
                  </span>
                  <span class="muted">owes you</span>
                </template>
                <template v-else>
                  <span class="muted">You owe</span>
                  <span class="who">
                    <MemberAvatar
                      :user-id="d.otherID"
                      :display-name="nameByID.get(d.otherID) ?? '?'"
                      :has-avatar="memberByID.get(d.otherID)?.has_avatar"
                      :avatar-updated-at="
                        memberByID.get(d.otherID)?.avatar_updated_at
                      "
                      :size="18"
                    />
                    <span class="who-name">{{
                      shortName(nameByID.get(d.otherID))
                    }}</span>
                  </span>
                </template>
                <span class="amount" :class="d.theyOweMe ? 'pos' : 'neg'">
                  {{ formatMoney(d.amountCents, currency) }}
                </span>
              </li>
            </ul>
          </div>
          <RouterLink
            :to="`/groups/${groupId}/settle`"
            class="btn-secondary btn-sm settle-btn"
          >
            Settle up
          </RouterLink>
        </div>
      </section>

      <!-- Add expense -->
      <section class="col-add">
        <h2 class="add-title">Add expense</h2>
        <form class="add-form" @submit.prevent="onSubmit">
          <div class="cat-row">
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
            <div class="cat-desc">
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
            :members="members"
            :currency="currency"
            :amount-cents="form.amountCents"
            :payer-id="form.payerId"
            :current-user-id="viewerId"
            :default-split="group.default_split"
          />

          <div class="add-actions">
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
      <section class="col-transactions">
        <RouterLink :to="`/groups/${groupId}/activity`" class="activity-link">
          <Icon name="clock-rotate-left" :size="12" />
          <span>See activity</span>
        </RouterLink>

        <p v-if="loaded && transactions.length === 0" class="empty">
          No expenses or settlements yet.
        </p>

        <ul v-else class="feed">
          <template v-for="(row, i) in feedRows" :key="i">
            <li v-if="row.kind === 'month-header'" class="month-header">
              {{ row.label }}
            </li>
            <li v-else-if="row.item.kind === 'expense' && row.item.expense">
              <RouterLink
                :to="`/groups/${groupId}/expenses/${row.item.expense.id}`"
                class="tx tx-expense"
              >
                <div class="tx-left">
                  <span
                    class="tx-date"
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
                    <span class="tx-month">{{
                      dayParts(row.item.expense.incurred_at).month
                    }}</span>
                    <span class="tx-day">{{
                      dayParts(row.item.expense.incurred_at).day
                    }}</span>
                  </span>
                  <div class="tx-body">
                    <div class="tx-title">
                      <span class="tx-desc">{{
                        row.item.expense.description
                      }}</span>
                      <span
                        v-if="row.item.cadence"
                        class="cadence-badge"
                        :title="`Repeats ${cadenceLabels[row.item.cadence] ?? row.item.cadence}`"
                      >
                        <Icon name="arrows-rotate" :size="9" />
                        {{
                          cadenceLabels[row.item.cadence] ?? row.item.cadence
                        }}
                      </span>
                    </div>
                    <div class="tx-sub">
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
                      <span class="tx-payer">{{
                        shortName(nameByID.get(row.item.expense.payer_id))
                      }}</span>
                    </div>
                  </div>
                </div>
                <div class="tx-right">
                  <span class="tx-stake">
                    <template
                      v-if="viewerStake(row.item.expense).kind === 'lent'"
                    >
                      <span class="stake-lent">you lent </span>
                      <span class="stake-amt">{{
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
                      <span class="stake-owes">you owe </span>
                      <span class="stake-amt">{{
                        rowMoney(
                          (viewerStake(row.item.expense) as { cents: number })
                            .cents,
                          row.item.expense.currency,
                        )
                      }}</span>
                    </template>
                    <span v-else class="muted">not involved</span>
                  </span>
                  <span class="tx-amount">{{
                    rowMoney(
                      row.item.expense.amount_cents,
                      row.item.expense.currency,
                    )
                  }}</span>
                </div>
              </RouterLink>
            </li>
            <li v-else-if="row.item.settlement">
              <RouterLink
                :to="`/groups/${groupId}/settlements/${row.item.settlement.id}`"
                class="tx tx-settlement"
              >
                <div class="tx-left">
                  <span class="tx-date" title="Settlement">
                    <span class="settle-icon"
                      ><Icon name="arrow-right" :size="14"
                    /></span>
                    <span class="tx-month">{{
                      dayParts(row.item.settlement.settled_at).month
                    }}</span>
                    <span class="tx-day">{{
                      dayParts(row.item.settlement.settled_at).day
                    }}</span>
                  </span>
                  <div class="tx-body">
                    <div class="tx-title settle-title">
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
                      <span class="tx-payer">{{
                        shortName(
                          nameByID.get(row.item.settlement.from_user_id),
                        )
                      }}</span>
                      <span class="muted">paid</span>
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
                      <span class="tx-payer">{{
                        shortName(nameByID.get(row.item.settlement.to_user_id))
                      }}</span>
                    </div>
                    <div class="tx-sub">
                      <span>settlement</span>
                      <span v-if="row.item.settlement.note" class="tx-note"
                        >· {{ row.item.settlement.note }}</span
                      >
                    </div>
                  </div>
                </div>
                <span class="tx-amount">{{
                  formatMoney(row.item.settlement.amount_cents, currency)
                }}</span>
              </RouterLink>
            </li>
          </template>
        </ul>

        <div v-if="nextCursor" class="load-more">
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
      class="cat-dialog"
      aria-modal="true"
      aria-label="Choose category"
    >
      <div class="cat-dialog-body">
        <div class="cat-dialog-head">
          <h3 class="cat-dialog-title">Choose category</h3>
          <button
            type="button"
            class="cat-close"
            aria-label="Close"
            @click="categoryDialog?.close()"
          >
            <Icon name="xmark" :size="14" />
          </button>
        </div>
        <ul class="cat-list">
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

  <AppLayout v-else-if="loaded && loadError" :back="{ to: '/groups', label: 'Groups' }">
    <Alert tone="error">
      Couldn't load this group. Check your connection and try again.
    </Alert>
  </AppLayout>
</template>

<style scoped>
.head {
  margin-bottom: 0.5rem;
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 0.75rem;
}
.title {
  min-width: 0;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 1.5rem;
  font-weight: 600;
}
.head-actions {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  gap: 0.5rem;
}
.head-btn {
  height: 2.25rem;
}
.head-btn-label {
  display: none;
}
@media (min-width: 640px) {
  .head-btn-label {
    display: inline;
  }
}
.tnum {
  font-variant-numeric: tabular-nums;
}
.subhead {
  margin-bottom: 0.75rem;
  font-size: 0.875rem;
  color: var(--muted-foreground);
}
.banner {
  margin-bottom: 1rem;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}
.link {
  font-weight: 500;
  text-decoration: underline;
}

.triptych {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  align-items: start;
  gap: 0.75rem;
}
@media (min-width: 1024px) {
  .triptych {
    grid-template-columns: 20.5rem minmax(0, 1fr) 20.5rem;
  }
  .col-balances {
    order: 1;
  }
  .col-transactions {
    order: 2;
  }
  .col-add {
    order: 3;
  }
}

.bal-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
  font-size: 0.875rem;
}
.bal-list {
  display: flex;
  min-width: 0;
  flex: 1;
  flex-direction: column;
  gap: 0.5rem;
}
.settled {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  color: oklch(0.508 0.118 165.612);
}
:root[data-theme="dark"] .settled,
:root[data-theme="high-contrast"] .settled {
  color: oklch(0.765 0.177 163.223);
}
.settled-msg {
  font-weight: 500;
}
.debts {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  list-style: none;
}
.debt {
  display: flex;
  min-width: 0;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.375rem;
}
.who {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.375rem;
}
.who-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.muted {
  color: var(--muted-foreground);
}
.amount {
  font-family: var(--font-mono);
}
.amount.pos {
  color: var(--primary);
}
.amount.neg {
  color: var(--destructive);
}
.settle-btn {
  flex-shrink: 0;
}

.col-transactions {
  position: relative;
}
.activity-link {
  position: absolute;
  right: 0;
  top: -0.75rem;
  z-index: 10;
  display: inline-flex;
  align-items: center;
  gap: 0.375rem;
  border-radius: 0.375rem;
  border: 1px solid var(--border);
  background: var(--card);
  padding: 0.375rem 0.75rem;
  font-size: 0.75rem;
  font-weight: 500;
  color: var(--muted-foreground);
}
.activity-link:hover {
  background: var(--muted);
}
.empty {
  font-size: 0.875rem;
  color: var(--muted-foreground);
}
.feed {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  list-style: none;
}
.month-header {
  padding: 0.75rem 0.25rem 0;
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--muted-foreground);
}
.month-header:first-child {
  padding-top: 0.25rem;
}
.tx {
  display: flex;
  align-items: stretch;
  justify-content: space-between;
  gap: 0.375rem;
  border-radius: 0.375rem;
  border: 1px solid var(--border);
  background: var(--card);
  padding: 0.375rem 0.75rem;
}
.tx:hover {
  background: var(--muted);
}
:root[data-theme="dark"] .tx:hover,
:root[data-theme="high-contrast"] .tx:hover {
  background: var(--accent);
}
.tx:focus-visible {
  outline: 2px solid var(--ring);
  outline-offset: 2px;
}
.tx-settlement {
  border-color: color-mix(in oklch, var(--primary) 40%, var(--border));
}
.tx-left {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.75rem;
}
.tx-date {
  flex-shrink: 0;
  display: inline-flex;
  width: 1.75rem;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  line-height: 1;
}
.settle-icon {
  display: inline-flex;
  height: 1.75rem;
  width: 1.75rem;
  align-items: center;
  justify-content: center;
  border-radius: 9999px;
  background: color-mix(in oklch, var(--primary) 20%, var(--card));
  color: var(--primary);
}
.tx-month {
  margin-top: 0.125rem;
  font-size: 8px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--muted-foreground);
}
.tx-day {
  font-size: 0.75rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  color: var(--muted-foreground);
}
.tx-body {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 0.125rem;
}
.tx-title {
  display: flex;
  min-width: 0;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.375rem;
  font-weight: 500;
}
.settle-title {
  font-weight: 500;
}
.tx-desc {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.cadence-badge {
  display: inline-flex;
  align-items: center;
  gap: 0.125rem;
  border-radius: 9999px;
  background: var(--muted);
  padding: 0 0.375rem;
  font-size: 10px;
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--muted-foreground);
}
.tx-sub {
  display: flex;
  min-width: 0;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.375rem;
  font-size: 11px;
  color: var(--muted-foreground);
}
.tx-payer {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.tx-note {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.tx-right {
  display: flex;
  flex-shrink: 0;
  flex-direction: column;
  align-items: flex-end;
  justify-content: space-between;
  gap: 0.25rem;
}
.tx-stake {
  font-size: 11px;
  font-variant-numeric: tabular-nums;
}
.stake-lent {
  color: var(--primary);
}
.stake-owes {
  color: oklch(0.6 0.13 60);
}
.stake-amt {
  font-family: var(--font-mono);
}
.tx-amount {
  align-self: center;
  flex-shrink: 0;
  font-family: var(--font-mono);
  font-size: 1.125rem;
  font-variant-numeric: tabular-nums;
}
.load-more {
  margin-top: 0.75rem;
  display: flex;
  justify-content: center;
}

.col-add {
  border-radius: 0.375rem;
  border: 1px solid var(--border);
  background: var(--card);
  padding: 0.75rem;
}
.add-title {
  margin-bottom: 0.75rem;
  font-weight: 500;
}
.add-form {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}
.cat-row {
  display: flex;
  align-items: flex-end;
  gap: 0.5rem;
}
.cat-desc {
  display: flex;
  flex: 1;
  flex-direction: column;
}
.add-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.5rem;
}

.cat-dialog {
  position: fixed;
  inset: 0;
  margin: auto;
  width: calc(100% - 2rem);
  max-width: 24rem;
  border: 1px solid var(--border);
  border-radius: 0.375rem;
  background: var(--popover);
  color: var(--popover-foreground);
  padding: 0;
  box-shadow: 0 20px 50px rgba(0, 0, 0, 0.35);
}
.cat-dialog::backdrop {
  background: var(--backdrop);
}
.cat-dialog-body {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  padding: 1.25rem;
}
.cat-dialog-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}
.cat-dialog-title {
  font-size: 1.125rem;
  font-weight: 500;
}
.cat-close {
  border-radius: 0.375rem;
  padding: 0.25rem 0.5rem;
  color: var(--muted-foreground);
  cursor: pointer;
}
.cat-close:hover {
  background: var(--muted);
}
.cat-list {
  display: flex;
  max-height: 60vh;
  flex-direction: column;
  gap: 0.125rem;
  overflow: auto;
  list-style: none;
}
</style>
