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
    <div class="head">
      <span class="head-icon"><Icon name="arrow-right" :size="18" /></span>
      <div>
        <h1 class="title">Settlement</h1>
        <p class="meta">
          {{ dateFmt.format(new Date(settlement.settled_at)) }} ·
          {{ moneyFmt.format(settlement.amount_cents / 100) }}
        </p>
      </div>
    </div>

    <Alert v-if="saveError" tone="error" class="banner">Could not save. Check your input and try again.</Alert>
    <Alert v-if="isDeleted" tone="info" class="banner">
      <Icon name="trash" :size="14" />
      <span>This settlement was deleted on {{ deletedAtFmt }}. Balances no longer count it. Restore it below to bring it back.</span>
    </Alert>

    <section class="panel">
      <h2 class="panel-title">{{ canEdit ? "Edit" : "Details" }}</h2>
      <form v-if="canEdit" class="form" @submit.prevent="onSave">
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
        <div class="actions">
          <DatePicker v-model="form.settledAt" variant="compact" :week-start="weekStart" />
          <button type="submit" class="btn-primary" :disabled="submitting">Save changes</button>
        </div>
      </form>
      <dl v-else class="details">
        <div class="detail">
          <dt class="muted">From</dt>
          <dd class="detail-who">
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
        <div class="detail">
          <dt class="muted">To</dt>
          <dd class="detail-who">
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
        <div class="detail">
          <dt class="muted">Amount</dt>
          <dd class="mono">{{ moneyFmt.format(settlement.amount_cents / 100) }}</dd>
        </div>
        <div class="detail">
          <dt class="muted">Date</dt>
          <dd>{{ dateFmt.format(new Date(settlement.settled_at)) }}</dd>
        </div>
        <div v-if="settlement.note" class="detail">
          <dt class="muted">Note</dt>
          <dd class="note-val">{{ settlement.note }}</dd>
        </div>
      </dl>
    </section>

    <section class="panel">
      <h2 class="panel-title">History</h2>
      <ul class="history">
        <li class="hist-row">
          <span class="hist-field">Recorded</span>
          <span class="muted small">{{ dateTimeFmt.format(new Date(settlement.created_at)) }}</span>
        </li>
        <li v-if="settledAtDiffers" class="hist-note">
          Backdated to {{ dateFmt.format(new Date(settlement.settled_at)) }}.
        </li>
      </ul>
    </section>

    <section v-if="canEdit" class="rounded-md border border-red-200 bg-card p-4 dark:border-red-900">
      <h2 class="mb-2 text-sm font-semibold uppercase tracking-wide text-red-600 dark:text-red-400">Danger zone</h2>
      <p class="muted mb">Soft-deletes this settlement. Balances will revert to the state before this payment was recorded.</p>
      <div class="right">
        <button type="button" class="btn-danger" @click="deleteConfirm = true">
          <Icon name="trash" /><span>Delete settlement</span>
        </button>
      </div>
    </section>

    <section v-if="isMember && isDeleted" class="panel">
      <h2 class="restore-title">Restore</h2>
      <p class="muted mb">Brings this settlement back. Balances will count this payment again.</p>
      <div class="right">
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

<style scoped>
.head {
  margin-bottom: 1.5rem;
  display: flex;
  align-items: center;
  gap: 0.75rem;
}
.head-icon {
  display: inline-flex;
  height: 2.5rem;
  width: 2.5rem;
  align-items: center;
  justify-content: center;
  border-radius: 9999px;
  background: color-mix(in oklch, var(--primary) 20%, var(--card));
  color: var(--primary);
}
.title {
  font-size: 1.5rem;
  font-weight: 600;
}
.meta {
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
.form {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}
.actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.5rem;
}
.details {
  display: flex;
  flex-direction: column;
  font-size: 0.875rem;
}
.detail {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  border-top: 1px solid var(--border);
  padding: 0.5rem 0;
}
.detail:first-child {
  border-top: 0;
  padding-top: 0;
}
@media (min-width: 640px) {
  .detail {
    flex-direction: row;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
  }
}
.detail-who {
  display: flex;
  align-items: center;
  gap: 0.375rem;
}
.muted {
  color: var(--subtle-foreground);
}
.small {
  font-size: 0.75rem;
}
.mb {
  margin-bottom: 0.75rem;
}
.mono {
  font-family: var(--font-mono);
}
.note-val {
  word-break: break-word;
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
  padding: 0.5rem 0;
}
.hist-row:first-child {
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
.hist-note {
  padding: 0.5rem 0;
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
</style>
