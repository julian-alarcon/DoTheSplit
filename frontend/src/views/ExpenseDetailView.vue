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
    <div class="mb-6 flex items-center gap-3">
      <CategoryIcon
        :slug="currentCategory?.slug"
        :group-label="currentCategory?.group_label"
        :size="40"
      />
      <div>
        <div class="flex flex-wrap items-center gap-2">
          <h1 class="text-2xl font-semibold">{{ expense.description }}</h1>
          <span v-if="expenseCadence" class="inline-flex items-center gap-1 rounded-full bg-muted px-2 py-0.5 text-xs font-medium uppercase tracking-wider text-muted-foreground">
            <Icon name="arrows-rotate" :size="11" />
            Repeats {{ cadenceLabels[expenseCadence] ?? expenseCadence }}
          </span>
        </div>
        <p class="flex flex-wrap items-center gap-1.5 text-sm text-muted-foreground">
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

    <Alert v-if="saveError" tone="error" class="mb-3 flex items-center gap-2">Could not save. Check your input and try again.</Alert>

    <Alert v-if="isDeleted" tone="info" class="mb-3 flex items-center gap-2">
      <Icon name="trash" :size="14" />
      <span>This expense was deleted on {{ deletedAtFmt }}. It no longer affects balances. Restore it below to bring it back.</span>
    </Alert>

    <!-- Edit -->
    <section v-if="!isDeleted" class="mb-4 rounded-md border border-border bg-card p-3">
      <h2 class="mb-3 font-medium">Edit</h2>
      <form v-if="canEdit" class="flex flex-col gap-4" @submit.prevent="onSave">
        <div class="flex items-end gap-2">
          <button type="button" class="field-category-trigger" aria-label="Choose category" @click="categoryDialog?.showModal()">
            <CategoryIcon
              :slug="selectedCategory?.slug ?? currentCategory?.slug"
              :group-label="selectedCategory?.group_label ?? currentCategory?.group_label"
              :size="38"
            />
          </button>
          <div class="flex flex-1 flex-col">
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
          v-model:notes="form.notes"
          :members="group.members"
          :currency="currency"
          :amount-cents="form.amountCents"
          :payer-id="form.payerId"
          :current-user-id="viewerId"
          :initial-splits="initialSplits"
        />

        <div class="flex items-center justify-end gap-2">
          <DatePicker v-model="form.incurredAt" variant="compact" :week-start="weekStart" />
          <button type="submit" class="btn-primary" :disabled="submitting">Save changes</button>
        </div>
        <p class="text-xs text-subtle-foreground">Leave the split as-is to keep current shares (rescaled proportionally if the amount changes).</p>
      </form>
      <p v-else class="text-sm text-subtle-foreground">Only group members can edit this expense.</p>
    </section>

    <!-- Splits -->
    <section class="mb-4 rounded-md border border-border bg-card p-3">
      <h2 class="mb-3 font-medium">Splits</h2>
      <ul class="flex list-none flex-col gap-2 text-sm">
        <li v-for="s in expense.splits" :key="s.user_id" class="flex items-center justify-between gap-2">
          <span class="flex min-w-0 items-center gap-2">
            <MemberAvatar
              :user-id="s.user_id"
              :display-name="nameByID.get(s.user_id) ?? s.user_id"
              :has-avatar="memberByID.get(s.user_id)?.has_avatar"
              :avatar-updated-at="memberByID.get(s.user_id)?.avatar_updated_at"
              :size="18"
            />
            <span class="truncate">{{ nameByID.get(s.user_id) ?? s.user_id }}</span>
          </span>
          <span class="shrink-0 [font-family:var(--font-mono)]">{{ money(s.share_cents) }}</span>
        </li>
      </ul>
    </section>

    <!-- History -->
    <section class="mb-4 rounded-md border border-border bg-card p-3">
      <h2 class="mb-3 font-medium">History</h2>
      <ul class="flex list-none flex-col text-sm">
        <li class="flex flex-col gap-1 border-t border-border py-2 first:border-t-0 first:pt-0 sm:flex-row sm:flex-wrap sm:items-baseline sm:justify-between sm:gap-2">
          <span class="font-medium">Created</span>
          <span class="flex items-center gap-1.5 text-xs text-muted-foreground">
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
        <li v-for="r in revisions" :key="r.id" class="flex flex-col gap-1 border-t border-border py-2 first:border-t-0 first:pt-0 sm:flex-row sm:flex-wrap sm:items-baseline sm:justify-between sm:gap-2">
          <span>
            <span class="font-medium">{{ fieldLabel[r.field] ?? r.field }}</span>:
            <span class="text-muted-foreground line-through">{{ renderValue(r.field, r.old_value) }}</span>
            <Icon name="arrow-right" :size="11" class="mx-1 align-middle text-muted-foreground" />
            <span>{{ renderValue(r.field, r.new_value) }}</span>
          </span>
          <span class="flex items-center gap-1.5 text-xs text-muted-foreground">
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
      <p class="mb-3 text-sm text-subtle-foreground">Soft-deletes this expense. It will stop affecting balances, but the edit history is kept.</p>
      <div class="flex justify-end">
        <button type="button" class="btn-danger" @click="deleteConfirm = true">
          <Icon name="trash" /><span>Delete expense</span>
        </button>
      </div>
    </section>

    <section v-if="isMember && isDeleted" class="mb-4 rounded-md border border-border bg-card p-3">
      <h2 class="mb-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">Restore</h2>
      <p class="mb-3 text-sm text-subtle-foreground">Brings this expense back. It will count toward balances again, with its splits and edit history intact.</p>
      <div class="flex justify-end">
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

    <dialog
      ref="categoryDialog"
      class="fixed inset-0 m-auto w-[calc(100%-2rem)] max-w-96 rounded-md border border-border bg-popover p-0 text-popover-foreground shadow-[0_20px_50px_rgba(0,0,0,0.35)] backdrop:bg-backdrop"
      aria-modal="true"
      aria-label="Choose category"
    >
      <div class="flex flex-col gap-3 p-5">
        <div class="flex items-center justify-between gap-3">
          <h3 class="text-lg font-medium">Choose category</h3>
          <button type="button" class="cursor-pointer rounded-md px-2 py-1 text-muted-foreground hover:bg-muted" aria-label="Close" @click="categoryDialog?.close()">
            <Icon name="xmark" :size="14" />
          </button>
        </div>
        <ul class="flex max-h-[60vh] list-none flex-col gap-0.5 overflow-auto">
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
