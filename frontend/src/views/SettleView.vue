<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useAuth } from "@/composables/useAuth";
import { getBalances, getGroup, type Group, type SimplifiedDebt } from "@/composables/useGroups";
import { createSettlement } from "@/composables/useSettlements";
import { useGroupMembers } from "@/composables/useGroupMembers";
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
const { memberByID, nameByID } = useGroupMembers(members);

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
    <div class="mx-auto flex max-w-xl flex-col gap-6">
      <div>
        <h1 class="text-2xl font-semibold">Settle up</h1>
        <p class="text-sm text-muted-foreground">Record a payment between two members of {{ group.name }}.</p>
      </div>

      <Alert v-if="error" tone="error">
        Could not record the settlement. Check the payer, recipient, and amount and try again.
      </Alert>

      <p v-if="members.length < 2" class="rounded-md border border-border bg-card p-4 text-sm text-muted-foreground">
        This group needs at least two members before anyone can settle up. Add someone under
        <RouterLink :to="`/groups/${groupId}/settings`" class="underline"> Group settings</RouterLink>.
      </p>

      <form v-else class="flex flex-col gap-4 rounded-md border border-border bg-card p-3" @submit.prevent="onSubmit">
        <label class="field-select-row">
          <span>Paid by <span class="text-destructive">*</span></span>
          <select v-model="form.fromUserId" required class="field-select">
            <option v-for="m in members" :key="m.user_id" :value="m.user_id">{{ m.display_name }}</option>
          </select>
        </label>

        <label class="field-select-row">
          <span>Paid to <span class="text-destructive">*</span></span>
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
          <input v-model="form.note" maxlength="200" autocomplete="off" placeholder=" " class="field-input" />
          <span class="field-label">Note</span>
        </label>

        <div class="flex items-center justify-end gap-2">
          <DatePicker v-model="form.settledAt" variant="compact" :week-start="weekStart" />
          <RouterLink :to="`/groups/${groupId}`" class="btn-secondary">Cancel</RouterLink>
          <button type="submit" class="btn-primary" :disabled="submitting">Record payment</button>
        </div>
      </form>

      <section v-if="simplified.length > 0" class="rounded-md border border-border bg-card p-3">
        <h2 class="mb-1 text-xs font-semibold uppercase tracking-wider text-muted-foreground">Suggested transfers</h2>
        <p class="mb-3 text-xs text-muted-foreground">Tap one to prefill the form above.</p>
        <ul class="flex list-none flex-col gap-2 text-sm">
          <li v-for="d in simplified" :key="`${d.from_user_id}-${d.to_user_id}`">
            <button
              type="button"
              class="flex w-full cursor-pointer items-center justify-between gap-2 rounded-md border border-border px-2 py-1.5 text-left transition-colors hover:bg-hover-surface"
              title="Use this transfer in the form above"
              @click="applySuggestion(d)"
            >
              <span class="flex min-w-0 items-center gap-2">
                <MemberAvatar
                  :user-id="d.from_user_id"
                  :display-name="nameByID.get(d.from_user_id) ?? '?'"
                  :has-avatar="memberByID.get(d.from_user_id)?.has_avatar"
                  :avatar-updated-at="memberByID.get(d.from_user_id)?.avatar_updated_at"
                  :size="18"
                />
                <span class="truncate">{{ nameByID.get(d.from_user_id) }}</span>
                <Icon name="arrow-right" :size="12" class="text-muted-foreground" />
                <MemberAvatar
                  :user-id="d.to_user_id"
                  :display-name="nameByID.get(d.to_user_id) ?? '?'"
                  :has-avatar="memberByID.get(d.to_user_id)?.has_avatar"
                  :avatar-updated-at="memberByID.get(d.to_user_id)?.avatar_updated_at"
                  :size="18"
                />
                <span class="truncate">{{ nameByID.get(d.to_user_id) }}</span>
              </span>
              <span class="flex-shrink-0 [font-family:var(--font-mono)]">{{ formatMoney(d.amount_cents, currency) }}</span>
            </button>
          </li>
        </ul>
      </section>
    </div>
  </AppLayout>
</template>
