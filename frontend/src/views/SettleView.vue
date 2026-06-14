<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useAuth } from "@/composables/useAuth";
import { getBalances, getGroup, type Group, type SimplifiedDebt } from "@/composables/useGroups";
import { createSettlement } from "@/composables/useSettlements";
import { formatMoney } from "@/lib/currencies";
import AppLayout from "@/components/AppLayout.vue";
import Alert from "@/components/Alert.vue";
import Icon from "@/components/Icon.vue";
import MemberAvatar from "@/components/MemberAvatar.vue";
import CurrencyInput from "@/components/CurrencyInput.vue";
import DatePicker from "@/components/DatePicker.vue";

const route = useRoute();
const router = useRouter();
const { state } = useAuth();
const groupId = computed(() => String(route.params.id));

const group = ref<Group | null>(null);
const simplified = ref<SimplifiedDebt[]>([]);
const loaded = ref(false);
const error = ref(false);
const submitting = ref(false);

const currency = computed(() => group.value?.default_currency ?? "EUR");
const members = computed(() => group.value?.members ?? []);
const nameByID = computed(() => new Map(members.value.map((m) => [m.user_id, m.display_name])));
const memberByID = computed(() => new Map(members.value.map((m) => [m.user_id, m])));

const form = ref({
  fromUserId: "",
  toUserId: "",
  amountCents: 0,
  note: "",
  settledAt: new Date().toISOString().slice(0, 10),
});
const weekStart = computed<0 | 1>(() => (state.user?.week_start === 0 ? 0 : 1));

function applySuggestion(d: SimplifiedDebt) {
  form.value.fromUserId = d.from_user_id;
  form.value.toUserId = d.to_user_id;
  form.value.amountCents = d.amount_cents;
  window.scrollTo({ top: 0, behavior: "smooth" });
}

async function onSubmit() {
  error.value = false;
  submitting.value = true;
  const res = await createSettlement(groupId.value, {
    fromUserId: form.value.fromUserId,
    toUserId: form.value.toUserId,
    amountCents: form.value.amountCents,
    note: form.value.note || undefined,
    settledAt: form.value.settledAt,
  });
  submitting.value = false;
  if (res.ok) {
    await router.replace(`/groups/${groupId.value}`);
  } else {
    error.value = true;
  }
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
  const bal = await getBalances(target);
  if (groupId.value !== target) return;
  simplified.value = bal.simplified;
  form.value.fromUserId = state.user?.id ?? members.value[0]?.user_id ?? "";
  loaded.value = true;
}

onMounted(load);
// vue-router reuses this instance when only :id changes; reload on id change.
watch(groupId, load);
</script>

<template>
  <AppLayout v-if="group" :back="{ to: `/groups/${groupId}`, label: group.name }">
    <div class="wrap">
      <div>
        <h1 class="title">Settle up</h1>
        <p class="lead">Record a payment between two members of {{ group.name }}.</p>
      </div>

      <Alert v-if="error" tone="error">
        Could not record the settlement. Check the payer, recipient, and amount and try again.
      </Alert>

      <p v-if="members.length < 2" class="note">
        This group needs at least two members before anyone can settle up. Add someone under
        <RouterLink :to="`/groups/${groupId}/settings`" class="link"> Group settings</RouterLink>.
      </p>

      <form v-else class="form" @submit.prevent="onSubmit">
        <label class="field-select-row">
          <span>Paid by <span class="req">*</span></span>
          <select v-model="form.fromUserId" required class="field-select">
            <option v-for="m in members" :key="m.user_id" :value="m.user_id">{{ m.display_name }}</option>
          </select>
        </label>

        <label class="field-select-row">
          <span>Paid to <span class="req">*</span></span>
          <select v-model="form.toUserId" required class="field-select">
            <option value="">Pick a member…</option>
            <option v-for="m in members" :key="m.user_id" :value="m.user_id">{{ m.display_name }}</option>
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
          <RouterLink :to="`/groups/${groupId}`" class="btn-secondary">Cancel</RouterLink>
          <button type="submit" class="btn-primary" :disabled="submitting">Record payment</button>
        </div>
      </form>

      <section v-if="simplified.length > 0" class="suggest">
        <h2 class="suggest-title">Suggested transfers</h2>
        <p class="suggest-sub">Tap one to prefill the form above.</p>
        <ul class="suggest-list">
          <li v-for="(d, i) in simplified" :key="i">
            <button type="button" class="suggest-row" title="Use this transfer in the form above" @click="applySuggestion(d)">
              <span class="suggest-who">
                <MemberAvatar
                  :user-id="d.from_user_id"
                  :display-name="nameByID.get(d.from_user_id) ?? '?'"
                  :has-avatar="memberByID.get(d.from_user_id)?.has_avatar"
                  :avatar-updated-at="memberByID.get(d.from_user_id)?.avatar_updated_at"
                  :size="18"
                />
                <span class="trunc">{{ nameByID.get(d.from_user_id) }}</span>
                <Icon name="arrow-right" :size="12" class="muted" />
                <MemberAvatar
                  :user-id="d.to_user_id"
                  :display-name="nameByID.get(d.to_user_id) ?? '?'"
                  :has-avatar="memberByID.get(d.to_user_id)?.has_avatar"
                  :avatar-updated-at="memberByID.get(d.to_user_id)?.avatar_updated_at"
                  :size="18"
                />
                <span class="trunc">{{ nameByID.get(d.to_user_id) }}</span>
              </span>
              <span class="suggest-amount">{{ formatMoney(d.amount_cents, currency) }}</span>
            </button>
          </li>
        </ul>
      </section>
    </div>
  </AppLayout>
</template>

<style scoped>
.wrap {
  margin-inline: auto;
  display: flex;
  max-width: 36rem;
  flex-direction: column;
  gap: 1.5rem;
}
.title {
  font-size: 1.5rem;
  font-weight: 600;
}
.lead {
  font-size: 0.875rem;
  color: var(--muted-foreground);
}
.note {
  border-radius: 0.375rem;
  border: 1px solid var(--border);
  background: var(--card);
  padding: 1rem;
  font-size: 0.875rem;
  color: var(--muted-foreground);
}
.link {
  text-decoration: underline;
}
.req {
  color: var(--destructive);
}
.form {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  border-radius: 0.375rem;
  border: 1px solid var(--border);
  background: var(--card);
  padding: 0.75rem;
}
.actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.5rem;
}
.suggest {
  border-radius: 0.375rem;
  border: 1px solid var(--border);
  background: var(--card);
  padding: 0.75rem;
}
.suggest-title {
  margin-bottom: 0.25rem;
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--muted-foreground);
}
.suggest-sub {
  margin-bottom: 0.75rem;
  font-size: 0.75rem;
  color: var(--muted-foreground);
}
.suggest-list {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  font-size: 0.875rem;
  list-style: none;
}
.suggest-row {
  display: flex;
  width: 100%;
  cursor: pointer;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  border-radius: 0.375rem;
  border: 1px solid var(--border);
  padding: 0.375rem 0.5rem;
  text-align: left;
  transition: background-color 120ms ease;
}
.suggest-row:hover {
  background: var(--muted);
}
:root[data-theme="dark"] .suggest-row:hover,
:root[data-theme="high-contrast"] .suggest-row:hover {
  background: var(--accent);
}
.suggest-who {
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
.muted {
  color: var(--muted-foreground);
}
.suggest-amount {
  flex-shrink: 0;
  font-family: var(--font-mono);
}
</style>
