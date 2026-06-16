<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useAuth } from "@/composables/useAuth";
import { getGroup, listCategories, listRecurring, type Group } from "@/composables/useGroups";
import {
  deleteExpense,
  getExpense,
  getExpenseRevisions,
  restoreExpense,
  updateExpense,
  type Expense,
  type ExpenseRevision,
} from "@/composables/useExpense";
import type { components } from "@/lib/api/schema";
import { moneyFormatter } from "@/lib/currencies";
import AppLayout from "@/components/AppLayout.vue";
import Alert from "@/components/Alert.vue";
import Icon from "@/components/Icon.vue";
import CategoryIcon from "@/components/CategoryIcon.vue";
import MemberAvatar from "@/components/MemberAvatar.vue";
import CurrencyInput from "@/components/CurrencyInput.vue";
import DatePicker from "@/components/DatePicker.vue";
import SplitEditor, { type SplitPayload } from "@/components/SplitEditor.vue";
import ConfirmDialog from "@/components/ConfirmDialog.vue";

type Category = components["schemas"]["Category"];

const route = useRoute();
const router = useRouter();
const { state } = useAuth();

const groupId = computed(() => String(route.params.id));
const expenseId = computed(() => String(route.params.eid));

const group = ref<Group | null>(null);
const expense = ref<Expense | null>(null);
const revisions = ref<ExpenseRevision[]>([]);
const categories = ref<Category[]>([]);
const recurring = ref<components["schemas"]["RecurringExpense"][]>([]);
const loaded = ref(false);
const saveError = ref(false);
const deleteConfirm = ref(false);
const restoreConfirm = ref(false);

const categoryByID = computed(() => new Map(categories.value.map((c) => [c.id, c])));
const memberByID = computed(() => new Map((group.value?.members ?? []).map((m) => [m.user_id, m])));
const nameByID = computed(() => new Map((group.value?.members ?? []).map((m) => [m.user_id, m.display_name])));

const currency = computed(() => expense.value?.currency ?? "EUR");
const moneyFmt = computed(() => moneyFormatter(currency.value));
function money(cents: number) {
  return moneyFmt.value.format(cents / 100);
}

const viewerId = computed(() => state.user?.id);
const isMember = computed(() =>
  (group.value?.members ?? []).some((m) => m.user_id === viewerId.value),
);
const isDeleted = computed(() => !!expense.value?.deleted_at);
const canEdit = computed(() => isMember.value && !isDeleted.value);

const currentCategory = computed(() =>
  expense.value ? categoryByID.value.get(expense.value.category_id) : undefined,
);

const dateFmt = new Intl.DateTimeFormat(undefined, { dateStyle: "medium" });
const dateTimeFmt = new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" });

const cadenceLabels: Record<string, string> = {
  daily: "daily",
  weekly: "weekly",
  biweekly: "every 2 weeks",
  monthly: "monthly",
  yearly: "yearly",
};
const expenseCadence = computed(() => {
  const e = expense.value;
  if (!e) return undefined;
  return recurring.value.find(
    (r) =>
      r.description === e.description &&
      r.amount_cents === e.amount_cents &&
      r.payer_id === e.payer_id &&
      r.currency === e.currency &&
      r.category_id === e.category_id,
  )?.cadence;
});

const deletedAtFmt = computed(() =>
  expense.value?.deleted_at ? dateTimeFmt.format(new Date(expense.value.deleted_at)) : "",
);

// --- Edit form ---------------------------------------------------------------
const form = ref({
  description: "",
  amountCents: 0,
  payerId: "",
  categoryId: "",
  notes: "",
  incurredAt: "",
});
const split = ref<SplitPayload | null>(null);
const splitEditor = ref<InstanceType<typeof SplitEditor> | null>(null);
const submitting = ref(false);
const categoryDialog = ref<HTMLDialogElement | null>(null);

const selectedCategory = computed(() => categoryByID.value.get(form.value.categoryId));
const initialSplits = computed(
  () => expense.value?.splits.map((s) => ({ user_id: s.user_id, share_cents: s.share_cents })) ?? [],
);
const weekStart = computed<0 | 1>(() => (state.user?.week_start === 0 ? 0 : 1));

function isNewGroupRow(i: number): boolean {
  return i === 0 || categories.value[i - 1].group_label !== categories.value[i].group_label;
}
function pickCategory(c: Category) {
  form.value.categoryId = c.id;
  categoryDialog.value?.close();
}

async function onSave() {
  saveError.value = false;
  submitting.value = true;
  // Only send splits when the editor was actually touched (dirty); otherwise
  // the backend keeps/rescales the existing split.
  const touched = splitEditor.value?.dirty ?? false;
  const res = await updateExpense(expenseId.value, {
    description: form.value.description,
    amountCents: form.value.amountCents,
    payerId: form.value.payerId,
    categoryId: form.value.categoryId || undefined,
    notes: form.value.notes,
    incurredAt: form.value.incurredAt,
    split: touched ? split.value : null,
  });
  submitting.value = false;
  if (res.ok) {
    await reload();
  } else {
    saveError.value = true;
  }
}

async function onDelete() {
  const res = await deleteExpense(expenseId.value);
  if (res.ok) await router.replace(`/groups/${groupId.value}`);
}
async function onRestore() {
  const res = await restoreExpense(expenseId.value);
  if (res.ok) await reload();
}

// --- History rendering -------------------------------------------------------
const fieldLabel: Record<string, string> = {
  description: "Description",
  amount_cents: "Amount",
  category_id: "Category",
  payer_id: "Paid by",
  splits: "Splits",
  incurred_at: "Date",
  notes: "Notes",
};
function renderValue(field: string, value: string): string {
  if (field === "amount_cents") {
    const n = Number(value);
    return Number.isFinite(n) ? money(n) : value;
  }
  if (field === "category_id") return categoryByID.value.get(value)?.label ?? value;
  if (field === "payer_id") return nameByID.value.get(value) ?? value;
  if (field === "splits") {
    try {
      const parsed = JSON.parse(value) as { user_id: string; share_cents: number }[];
      if (!Array.isArray(parsed) || parsed.length === 0) return "-";
      return parsed
        .map((s) => `${nameByID.value.get(s.user_id) ?? s.user_id}: ${money(s.share_cents)}`)
        .join(", ");
    } catch {
      return value;
    }
  }
  if (field === "incurred_at") {
    const d = new Date(value);
    return Number.isNaN(d.getTime()) ? value : dateFmt.format(d);
  }
  return value;
}

async function reload() {
  const [e, revs] = await Promise.all([
    getExpense(expenseId.value),
    getExpenseRevisions(expenseId.value),
  ]);
  if (!e) {
    await router.replace(`/groups/${groupId.value}`);
    return;
  }
  expense.value = e;
  revisions.value = revs;
  form.value = {
    description: e.description,
    amountCents: e.amount_cents,
    payerId: e.payer_id,
    categoryId: e.category_id,
    notes: e.notes ?? "",
    incurredAt: new Date(e.incurred_at).toISOString().slice(0, 10),
  };
}

onMounted(async () => {
  const { group: g } = await getGroup(groupId.value);
  if (!g) {
    await router.replace(`/groups/${groupId.value}`);
    return;
  }
  group.value = g;
  [categories.value, recurring.value] = await Promise.all([
    listCategories(),
    listRecurring(groupId.value),
  ]);
  await reload();
  loaded.value = true;
});
</script>

<template>
  <AppLayout v-if="expense && group" :back="{ to: `/groups/${groupId}`, label: group.name }">
    <div class="head">
      <CategoryIcon
        :slug="currentCategory?.slug"
        :group-label="currentCategory?.group_label"
        :size="40"
      />
      <div>
        <div class="head-title">
          <h1 class="title">{{ expense.description }}</h1>
          <span v-if="expenseCadence" class="cadence">
            <Icon name="arrows-rotate" :size="11" />
            Repeats {{ cadenceLabels[expenseCadence] ?? expenseCadence }}
          </span>
        </div>
        <p class="meta">
          <span>{{ dateFmt.format(new Date(expense.incurred_at)) }} · paid by</span>
          <MemberAvatar
            :user-id="expense.payer_id"
            :display-name="nameByID.get(expense.payer_id) ?? '?'"
            :has-avatar="memberByID.get(expense.payer_id)?.has_avatar"
            :avatar-updated-at="memberByID.get(expense.payer_id)?.avatar_updated_at"
            :size="18"
          />
          <span>{{ nameByID.get(expense.payer_id) ?? "?" }}</span>
          <span>· {{ money(expense.amount_cents) }}</span>
        </p>
      </div>
    </div>

    <Alert v-if="saveError" tone="error" class="banner">Could not save. Check your input and try again.</Alert>

    <Alert v-if="isDeleted" tone="info" class="banner">
      <Icon name="trash" :size="14" />
      <span>This expense was deleted on {{ deletedAtFmt }}. It no longer affects balances. Restore it below to bring it back.</span>
    </Alert>

    <!-- Edit -->
    <section v-if="!isDeleted" class="panel">
      <h2 class="panel-title">Edit</h2>
      <form v-if="canEdit" class="edit-form" @submit.prevent="onSave">
        <div class="cat-row">
          <button type="button" class="field-category-trigger" aria-label="Choose category" @click="categoryDialog?.showModal()">
            <CategoryIcon
              :slug="selectedCategory?.slug ?? currentCategory?.slug"
              :group-label="selectedCategory?.group_label ?? currentCategory?.group_label"
              :size="38"
            />
          </button>
          <div class="cat-desc">
            <label class="field">
              <input v-model="form.description" class="field-input" required maxlength="200" placeholder=" " />
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
          <select v-model="form.payerId" class="field-select">
            <option v-for="m in group.members" :key="m.user_id" :value="m.user_id">{{ m.display_name }}</option>
          </select>
        </label>

        <SplitEditor
          ref="splitEditor"
          v-model="split"
          :members="group.members"
          :currency="currency"
          :amount-cents="form.amountCents"
          :payer-id="form.payerId"
          :current-user-id="viewerId"
          :initial-splits="initialSplits"
        />

        <div class="edit-actions">
          <DatePicker v-model="form.incurredAt" variant="compact" :week-start="weekStart" />
          <button type="submit" class="btn-primary" :disabled="submitting">Save changes</button>
        </div>
        <p class="hint">Leave the split as-is to keep current shares (rescaled proportionally if the amount changes).</p>
      </form>
      <p v-else class="muted">Only group members can edit this expense.</p>
    </section>

    <!-- Splits -->
    <section class="panel">
      <h2 class="panel-title">Splits</h2>
      <ul class="splits">
        <li v-for="s in expense.splits" :key="s.user_id" class="split-row">
          <span class="split-who">
            <MemberAvatar
              :user-id="s.user_id"
              :display-name="nameByID.get(s.user_id) ?? s.user_id"
              :has-avatar="memberByID.get(s.user_id)?.has_avatar"
              :avatar-updated-at="memberByID.get(s.user_id)?.avatar_updated_at"
              :size="18"
            />
            <span class="trunc">{{ nameByID.get(s.user_id) ?? s.user_id }}</span>
          </span>
          <span class="split-amt">{{ money(s.share_cents) }}</span>
        </li>
      </ul>
    </section>

    <!-- History -->
    <section class="panel">
      <h2 class="panel-title">History</h2>
      <ul class="history">
        <li class="hist-row">
          <span class="hist-field">Created</span>
          <span class="hist-by">
            <MemberAvatar
              :user-id="expense.created_by"
              :display-name="nameByID.get(expense.created_by) ?? '?'"
              :has-avatar="memberByID.get(expense.created_by)?.has_avatar"
              :avatar-updated-at="memberByID.get(expense.created_by)?.avatar_updated_at"
              :size="16"
            />
            <span>{{ nameByID.get(expense.created_by) ?? "?" }} · {{ dateTimeFmt.format(new Date(expense.created_at)) }}</span>
          </span>
        </li>
        <li v-for="r in revisions" :key="r.id" class="hist-row">
          <span>
            <span class="hist-field">{{ fieldLabel[r.field] ?? r.field }}</span>:
            <span class="strike">{{ renderValue(r.field, r.old_value) }}</span>
            <Icon name="arrow-right" :size="11" class="hist-arrow" />
            <span>{{ renderValue(r.field, r.new_value) }}</span>
          </span>
          <span class="hist-by">
            <MemberAvatar
              :user-id="r.edited_by"
              :display-name="nameByID.get(r.edited_by) ?? '?'"
              :has-avatar="memberByID.get(r.edited_by)?.has_avatar"
              :avatar-updated-at="memberByID.get(r.edited_by)?.avatar_updated_at"
              :size="16"
            />
            <span>{{ nameByID.get(r.edited_by) ?? "?" }} · {{ dateTimeFmt.format(new Date(r.edited_at)) }}</span>
          </span>
        </li>
      </ul>
    </section>

    <!-- Danger / Restore -->
    <section v-if="canEdit" class="rounded-md border border-red-200 bg-card p-4 dark:border-red-900">
      <h2 class="mb-2 text-sm font-semibold uppercase tracking-wide text-red-600 dark:text-red-400">Danger zone</h2>
      <p class="muted mb">Soft-deletes this expense. It will stop affecting balances, but the edit history is kept.</p>
      <div class="right">
        <button type="button" class="btn-danger" @click="deleteConfirm = true">
          <Icon name="trash" /><span>Delete expense</span>
        </button>
      </div>
    </section>

    <section v-if="isMember && isDeleted" class="panel">
      <h2 class="restore-title">Restore</h2>
      <p class="muted mb">Brings this expense back. It will count toward balances again, with its splits and edit history intact.</p>
      <div class="right">
        <button type="button" class="btn-primary" @click="restoreConfirm = true">
          <Icon name="trash-arrow-up" /><span>Restore expense</span>
        </button>
      </div>
    </section>

    <ConfirmDialog
      v-model:open="deleteConfirm"
      title="Delete this expense?"
      message="It will stop affecting balances, but the edit history is kept. You can restore it later from the activity log."
      confirm-label="Delete expense"
      confirm-icon="trash"
      @confirm="onDelete"
    />
    <ConfirmDialog
      v-model:open="restoreConfirm"
      title="Restore this expense?"
      message="It will start affecting balances again."
      confirm-label="Restore expense"
      confirm-variant="primary"
      confirm-icon="trash-arrow-up"
      @confirm="onRestore"
    />

    <dialog ref="categoryDialog" class="cat-dialog" aria-modal="true" aria-label="Choose category">
      <div class="cat-dialog-body">
        <div class="cat-dialog-head">
          <h3 class="cat-dialog-title">Choose category</h3>
          <button type="button" class="cat-close" aria-label="Close" @click="categoryDialog?.close()">
            <Icon name="xmark" :size="14" />
          </button>
        </div>
        <ul class="cat-list">
          <template v-for="(c, i) in categories" :key="c.id">
            <li v-if="isNewGroupRow(i)" class="field-category-group">{{ c.group_label }}</li>
            <li>
              <button type="button" class="field-category-option" @click="pickCategory(c)">
                <CategoryIcon :slug="c.slug" :group-label="c.group_label" :size="28" />
                <span>{{ c.label }}</span>
              </button>
            </li>
          </template>
        </ul>
      </div>
    </dialog>
  </AppLayout>
</template>

<style scoped>
.head {
  margin-bottom: 1.5rem;
  display: flex;
  align-items: center;
  gap: 0.75rem;
}
.head-title {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
}
.title {
  font-size: 1.5rem;
  font-weight: 600;
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
.meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.375rem;
  font-size: 0.875rem;
  color: var(--muted-foreground);
}
.banner {
  margin-bottom: 0.75rem;
  display: flex;
  align-items: center;
  gap: 0.5rem;
}
.panel {
  margin-bottom: 1rem;
  border-radius: 0.375rem;
  border: 1px solid var(--border);
  background: var(--card);
  padding: 0.75rem;
}
.panel-title {
  margin-bottom: 0.75rem;
  font-weight: 500;
}
.edit-form {
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
.edit-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.5rem;
}
.hint,
.muted {
  font-size: 0.75rem;
  color: var(--subtle-foreground);
}
.muted {
  font-size: 0.875rem;
}
.mb {
  margin-bottom: 0.75rem;
}
.splits {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  font-size: 0.875rem;
  list-style: none;
}
.split-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}
.split-who {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.5rem;
}
.trunc {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.split-amt {
  flex-shrink: 0;
  font-family: var(--font-mono);
}
.history {
  display: flex;
  flex-direction: column;
  font-size: 0.875rem;
  list-style: none;
}
.hist-row {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  border-top: 1px solid var(--border);
  padding: 0.5rem 0;
}
.hist-row:first-child {
  border-top: 0;
  padding-top: 0;
}
@media (min-width: 640px) {
  .hist-row {
    flex-direction: row;
    flex-wrap: wrap;
    align-items: baseline;
    justify-content: space-between;
    gap: 0.5rem;
  }
}
.hist-field {
  font-weight: 500;
}
.strike {
  color: var(--muted-foreground);
  text-decoration: line-through;
}
.hist-arrow {
  margin: 0 0.25rem;
  color: var(--muted-foreground);
  vertical-align: middle;
}
.hist-by {
  display: flex;
  align-items: center;
  gap: 0.375rem;
  font-size: 0.75rem;
  color: var(--muted-foreground);
}
.restore-title {
  margin-bottom: 0.5rem;
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--muted-foreground);
}
.right {
  display: flex;
  justify-content: flex-end;
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
.cat-list {
  display: flex;
  max-height: 60vh;
  flex-direction: column;
  gap: 0.125rem;
  overflow: auto;
  list-style: none;
}
</style>
