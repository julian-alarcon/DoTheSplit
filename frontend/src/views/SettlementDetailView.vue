<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useAuth } from "@/composables/useAuth";
import { getGroup, type Group } from "@/composables/useGroups";
import {
  deleteSettlement,
  getSettlement,
  restoreSettlement,
  updateSettlement,
  type Settlement,
} from "@/composables/useSettlements";
import { moneyFormatter } from "@/lib/currencies";
import AppLayout from "@/components/AppLayout.vue";
import Alert from "@/components/Alert.vue";
import Icon from "@/components/Icon.vue";
import MemberAvatar from "@/components/MemberAvatar.vue";
import CurrencyInput from "@/components/CurrencyInput.vue";
import DatePicker from "@/components/DatePicker.vue";
import ConfirmDialog from "@/components/ConfirmDialog.vue";

const route = useRoute();
const router = useRouter();
const { state } = useAuth();

const groupId = computed(() => String(route.params.id));
const settlementId = computed(() => String(route.params.sid));

const group = ref<Group | null>(null);
const settlement = ref<Settlement | null>(null);
const loaded = ref(false);
const saveError = ref(false);
const deleteConfirm = ref(false);
const restoreConfirm = ref(false);

const memberByID = computed(() => new Map((group.value?.members ?? []).map((m) => [m.user_id, m])));
const nameByID = computed(() => new Map((group.value?.members ?? []).map((m) => [m.user_id, m.display_name])));
const currency = computed(() => group.value?.default_currency ?? "EUR");
const moneyFmt = computed(() => moneyFormatter(currency.value));

const viewerId = computed(() => state.user?.id);
const isMember = computed(() => (group.value?.members ?? []).some((m) => m.user_id === viewerId.value));
const isDeleted = computed(() => !!settlement.value?.deleted_at);
const canEdit = computed(() => isMember.value && !isDeleted.value);

const dateFmt = new Intl.DateTimeFormat(undefined, { dateStyle: "medium" });
const dateTimeFmt = new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" });

const deletedAtFmt = computed(() =>
  settlement.value?.deleted_at ? dateTimeFmt.format(new Date(settlement.value.deleted_at)) : "",
);
const settledAtDiffers = computed(() => {
  const s = settlement.value;
  if (!s) return false;
  return new Date(s.created_at).toISOString().slice(0, 10) !== new Date(s.settled_at).toISOString().slice(0, 10);
});

const form = ref({
  fromUserId: "",
  toUserId: "",
  amountCents: 0,
  note: "",
  settledAt: "",
});
const submitting = ref(false);
const weekStart = computed<0 | 1>(() => (state.user?.week_start === 0 ? 0 : 1));

async function onSave() {
  saveError.value = false;
  submitting.value = true;
  const res = await updateSettlement(settlementId.value, {
    fromUserId: form.value.fromUserId,
    toUserId: form.value.toUserId,
    amountCents: form.value.amountCents,
    note: form.value.note,
    settledAt: form.value.settledAt,
  });
  submitting.value = false;
  if (res.ok) await reload();
  else saveError.value = true;
}
async function onDelete() {
  const res = await deleteSettlement(settlementId.value);
  if (res.ok) await router.replace(`/groups/${groupId.value}`);
}
async function onRestore() {
  const res = await restoreSettlement(settlementId.value);
  if (res.ok) await reload();
}

async function reload() {
  const s = await getSettlement(settlementId.value);
  if (!s || s.group_id !== groupId.value) {
    await router.replace(`/groups/${groupId.value}`);
    return;
  }
  settlement.value = s;
  form.value = {
    fromUserId: s.from_user_id,
    toUserId: s.to_user_id,
    amountCents: s.amount_cents,
    note: s.note ?? "",
    settledAt: new Date(s.settled_at).toISOString().slice(0, 10),
  };
}

onMounted(async () => {
  const { group: g } = await getGroup(groupId.value);
  if (!g) {
    await router.replace(`/groups/${groupId.value}`);
    return;
  }
  group.value = g;
  await reload();
  loaded.value = true;
});
</script>

<template>
  <AppLayout v-if="settlement && group" :back="{ to: `/groups/${groupId}`, label: group.name }">
    <div class="mb-6 flex items-center gap-3">
      <span class="inline-flex h-10 w-10 items-center justify-center rounded-full bg-emerald-100 text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300" title="Settlement"><Icon name="arrow-right" :size="18" /></span>
      <div>
        <h1 class="text-2xl font-semibold">Settlement</h1>
        <p class="text-sm text-muted-foreground">
          {{ dateFmt.format(new Date(settlement.settled_at)) }} ·
          {{ moneyFmt.format(settlement.amount_cents / 100) }}
        </p>
      </div>
    </div>

    <Alert v-if="saveError" tone="error" class="mb-3 flex items-center gap-2">Could not save. Check your input and try again.</Alert>
    <Alert v-if="isDeleted" tone="info" class="mb-3 flex items-center gap-2">
      <Icon name="trash" :size="14" />
      <span>This settlement was deleted on {{ deletedAtFmt }}. Balances no longer count it. Restore it below to bring it back.</span>
    </Alert>

    <section class="mb-4 rounded-md border border-border bg-card p-3">
      <h2 class="mb-3 font-medium">{{ canEdit ? "Edit" : "Details" }}</h2>
      <form v-if="canEdit" class="flex flex-col gap-4" @submit.prevent="onSave">
        <label class="field-select-row">
          <span>Paid by</span>
          <select v-model="form.fromUserId" class="field-select">
            <option v-for="m in group.members" :key="m.user_id" :value="m.user_id">{{ m.display_name }}</option>
          </select>
        </label>
        <label class="field-select-row">
          <span>Paid to</span>
          <select v-model="form.toUserId" class="field-select">
            <option v-for="m in group.members" :key="m.user_id" :value="m.user_id">{{ m.display_name }}</option>
          </select>
        </label>
        <CurrencyInput
          v-model="form.amountCents"
          :label="`Amount (${currency})`"
          :currency="currency"
          required
          error="Enter an amount greater than 0"
        />
        <label class="field">
          <input v-model="form.note" maxlength="200" placeholder=" " class="field-input" />
          <span class="field-label">Note</span>
        </label>
        <div class="flex items-center justify-end gap-2">
          <DatePicker v-model="form.settledAt" variant="compact" :week-start="weekStart" />
          <button type="submit" class="btn-primary" :disabled="submitting">Save changes</button>
        </div>
      </form>
      <dl v-else class="flex flex-col text-sm">
        <div class="flex flex-col gap-1 border-t border-border py-2 first:border-t-0 first:pt-0 sm:flex-row sm:items-center sm:justify-between sm:gap-2">
          <dt class="text-subtle-foreground">From</dt>
          <dd class="flex items-center gap-1.5">
            <MemberAvatar
              :user-id="settlement.from_user_id"
              :display-name="nameByID.get(settlement.from_user_id) ?? '?'"
              :has-avatar="memberByID.get(settlement.from_user_id)?.has_avatar"
              :avatar-updated-at="memberByID.get(settlement.from_user_id)?.avatar_updated_at"
              :size="20"
            />
            <span>{{ nameByID.get(settlement.from_user_id) ?? "?" }}</span>
          </dd>
        </div>
        <div class="flex flex-col gap-1 border-t border-border py-2 first:border-t-0 first:pt-0 sm:flex-row sm:items-center sm:justify-between sm:gap-2">
          <dt class="text-subtle-foreground">To</dt>
          <dd class="flex items-center gap-1.5">
            <MemberAvatar
              :user-id="settlement.to_user_id"
              :display-name="nameByID.get(settlement.to_user_id) ?? '?'"
              :has-avatar="memberByID.get(settlement.to_user_id)?.has_avatar"
              :avatar-updated-at="memberByID.get(settlement.to_user_id)?.avatar_updated_at"
              :size="20"
            />
            <span>{{ nameByID.get(settlement.to_user_id) ?? "?" }}</span>
          </dd>
        </div>
        <div class="flex flex-col gap-1 border-t border-border py-2 first:border-t-0 first:pt-0 sm:flex-row sm:items-center sm:justify-between sm:gap-2">
          <dt class="text-subtle-foreground">Amount</dt>
          <dd class="[font-family:var(--font-mono)]">{{ moneyFmt.format(settlement.amount_cents / 100) }}</dd>
        </div>
        <div class="flex flex-col gap-1 border-t border-border py-2 first:border-t-0 first:pt-0 sm:flex-row sm:items-center sm:justify-between sm:gap-2">
          <dt class="text-subtle-foreground">Date</dt>
          <dd>{{ dateFmt.format(new Date(settlement.settled_at)) }}</dd>
        </div>
        <div v-if="settlement.note" class="flex flex-col gap-1 border-t border-border py-2 first:border-t-0 first:pt-0 sm:flex-row sm:items-center sm:justify-between sm:gap-2">
          <dt class="text-subtle-foreground">Note</dt>
          <dd class="break-words">{{ settlement.note }}</dd>
        </div>
      </dl>
    </section>

    <section class="mb-4 rounded-md border border-border bg-card p-3">
      <h2 class="mb-3 font-medium">History</h2>
      <ul class="flex list-none flex-col text-sm">
        <li class="flex flex-col gap-1 py-2 first:pt-0 sm:flex-row sm:flex-wrap sm:items-baseline sm:justify-between sm:gap-2">
          <span class="font-medium">Recorded</span>
          <span class="text-xs text-subtle-foreground">{{ dateTimeFmt.format(new Date(settlement.created_at)) }}</span>
        </li>
        <li v-if="settledAtDiffers" class="py-2 text-xs text-muted-foreground">
          Backdated to {{ dateFmt.format(new Date(settlement.settled_at)) }}.
        </li>
      </ul>
    </section>

    <section v-if="canEdit" class="rounded-md border border-red-200 bg-card p-4 dark:border-red-900">
      <h2 class="mb-2 text-sm font-semibold uppercase tracking-wide text-red-600 dark:text-red-400">Danger zone</h2>
      <p class="mb-3 text-sm text-subtle-foreground">Soft-deletes this settlement. Balances will revert to the state before this payment was recorded.</p>
      <div class="flex justify-end">
        <button type="button" class="btn-danger" @click="deleteConfirm = true">
          <Icon name="trash" /><span>Delete settlement</span>
        </button>
      </div>
    </section>

    <section v-if="isMember && isDeleted" class="mb-4 rounded-md border border-border bg-card p-3">
      <h2 class="mb-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">Restore</h2>
      <p class="mb-3 text-sm text-subtle-foreground">Brings this settlement back. Balances will count this payment again.</p>
      <div class="flex justify-end">
        <button type="button" class="btn-primary" @click="restoreConfirm = true">
          <Icon name="trash-arrow-up" /><span>Restore settlement</span>
        </button>
      </div>
    </section>

    <ConfirmDialog
      v-model:open="deleteConfirm"
      title="Delete this settlement?"
      message="It will stop affecting balances. You can restore it later from the activity log."
      confirm-label="Delete settlement"
      confirm-icon="trash"
      @confirm="onDelete"
    />
    <ConfirmDialog
      v-model:open="restoreConfirm"
      title="Restore this settlement?"
      message="Balances will count this payment again."
      confirm-label="Restore settlement"
      confirm-variant="primary"
      confirm-icon="trash-arrow-up"
      @confirm="onRestore"
    />
  </AppLayout>
</template>
