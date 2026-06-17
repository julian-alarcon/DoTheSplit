<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useAuth } from "@/composables/useAuth";
import {
  addMember,
  deleteGroup,
  getBalances,
  getGroup,
  hasTransactions,
  removeMember,
  updateGroup,
  type Group,
} from "@/composables/useGroups";
import { formatMoney } from "@/lib/currencies";
import AppLayout from "@/components/AppLayout.vue";
import Alert from "@/components/Alert.vue";
import Icon from "@/components/Icon.vue";
import Field from "@/components/Field.vue";
import CurrencySelect from "@/components/CurrencySelect.vue";
import MemberAvatar from "@/components/MemberAvatar.vue";
import ConfirmDialog from "@/components/ConfirmDialog.vue";

const route = useRoute();
const router = useRouter();
const { state } = useAuth();
const groupId = computed(() => String(route.params.id));

const group = ref<Group | null>(null);
const currencyLocked = ref(false);
const myNetCents = ref(0);
const loaded = ref(false);
const error = ref<string | null>(null);

const myId = computed(() => state.user?.id ?? "");
const isCreator = computed(() => group.value?.created_by === myId.value);
const members = computed(() => group.value?.members ?? []);
const isPair = computed(() => members.value.length === 2);
const hasDefaultSplit = computed(() => (group.value?.default_split?.length ?? 0) === 2);

// Rename + currency form.
const name = ref("");
const currency = ref("EUR");

// Default-split form (2-member only).
const split1 = ref("50");
const split2 = ref("50");

// Add-member + transfer.
const newEmail = ref("");
const newOwnerId = ref("");

// Confirm dialogs.
const removeTarget = ref<string | null>(null);
const leaveConfirm = ref(false);
const transferConfirm = ref(false);
const deleteConfirm = ref(false);

async function reload() {
  const { group: g } = await getGroup(groupId.value);
  if (!g) {
    await router.replace("/groups");
    return;
  }
  group.value = g;
  name.value = g.name;
  currency.value = g.default_currency;
  const ds = g.default_split ?? [];
  if (ds.length === 2) {
    const m0 = g.members[0]?.user_id;
    const e0 = ds.find((e) => e.user_id === m0) ?? ds[0];
    const e1 = ds.find((e) => e.user_id !== e0.user_id) ?? ds[1];
    split1.value = (e0.basis_points / 100).toString();
    split2.value = (e1.basis_points / 100).toString();
  } else {
    split1.value = "50";
    split2.value = "50";
  }
}

async function onSaveRename() {
  error.value = null;
  const patch: { name: string; default_currency?: string } = { name: name.value };
  if (!currencyLocked.value) patch.default_currency = currency.value;
  const res = await updateGroup(groupId.value, patch);
  if (!res.ok) error.value = "Could not update the group. Check your input and try again.";
  else await reload();
}

async function onAddMember() {
  error.value = null;
  const res = await addMember(groupId.value, newEmail.value.trim().toLowerCase());
  if (!res.ok) {
    error.value = "Could not add the member. They must already be registered.";
    return;
  }
  newEmail.value = "";
  await reload();
}

async function onRemoveMember() {
  error.value = null;
  const userId = removeTarget.value;
  removeTarget.value = null;
  if (!userId) return;
  const res = await removeMember(groupId.value, userId);
  if (res.ok) await reload();
  else error.value = res.message || "Could not remove the member. Try again.";
}

async function onSaveSplit() {
  error.value = null;
  const bp1 = Math.round(Number(split1.value) * 100);
  const bp2 = Math.round(Number(split2.value) * 100);
  if (bp1 + bp2 !== 10000) {
    error.value = "The two percentages must sum to 100.";
    return;
  }
  const res = await updateGroup(groupId.value, {
    default_split: [
      { user_id: members.value[0].user_id, basis_points: bp1 },
      { user_id: members.value[1].user_id, basis_points: bp2 },
    ],
  });
  if (res.ok) await reload();
  else error.value = "Could not save the default split.";
}

async function onClearSplit() {
  const res = await updateGroup(groupId.value, { default_split: [] });
  if (res.ok) await reload();
}

async function onLeave() {
  const res = await removeMember(groupId.value, myId.value);
  if (res.ok) await router.replace("/groups");
}

async function onTransfer() {
  if (!newOwnerId.value) return;
  const res = await updateGroup(groupId.value, { created_by: newOwnerId.value });
  if (res.ok) await reload();
}

async function onDeleteGroup() {
  const res = await deleteGroup(groupId.value);
  if (res.ok) await router.replace("/groups");
}

async function load() {
  loaded.value = false;
  const target = groupId.value;
  await reload();
  const [locked, bal] = await Promise.all([
    hasTransactions(target),
    getBalances(target),
  ]);
  if (groupId.value !== target) return;
  currencyLocked.value = locked;
  myNetCents.value = bal.net.find((b) => b.user_id === myId.value)?.net_cents ?? 0;
  loaded.value = true;
}

onMounted(load);
// vue-router reuses this instance when only :id changes; reload on id change.
watch(groupId, load);
</script>

<template>
  <AppLayout v-if="group" :back="{ to: `/groups/${groupId}`, label: group.name }">
    <h1 class="mb-2 text-2xl font-semibold">Group settings</h1>
    <p class="mb-6 text-sm text-subtle-foreground">
      {{ members.length }} member{{ members.length === 1 ? "" : "s" }} ·
      default currency {{ group.default_currency }}
    </p>

    <Alert v-if="error" tone="error" class="mb-3">{{ error }}</Alert>

    <!-- Rename & currency -->
    <section class="mb-4 rounded-md border border-border bg-card p-3">
      <h2 class="mb-3 font-medium">Rename &amp; currency</h2>
      <form class="flex flex-col gap-3" @submit.prevent="onSaveRename">
        <Field v-model="name" label="Group name" type="text" required minlength="1" maxlength="80" error="Required" />
        <label class="field-select-row">
          <span>Default currency</span>
          <CurrencySelect v-model="currency" :disabled="currencyLocked" />
        </label>
        <p v-if="currencyLocked" class="text-xs text-subtle-foreground">
          Currency is fixed after the first expense or settlement. To change it, delete all transactions or create a new group.
        </p>
        <p v-else class="text-xs text-subtle-foreground">Each group uses a single currency. Multi-currency groups are not supported.</p>
        <button type="submit" class="btn-primary self-end">Save</button>
      </form>
    </section>

    <!-- Members -->
    <section class="mb-4 rounded-md border border-border bg-card p-3">
      <h2 class="mb-3 font-medium">Members</h2>
      <ul class="mb-3 flex list-none flex-col gap-2 text-sm">
        <li v-for="m in members" :key="m.user_id" class="flex items-center justify-between gap-2">
          <span class="flex min-w-0 items-center gap-2">
            <MemberAvatar
              :user-id="m.user_id"
              :display-name="m.display_name"
              :has-avatar="m.has_avatar"
              :avatar-updated-at="m.avatar_updated_at"
              :size="20"
            />
            <span class="truncate">{{ m.display_name }}</span>
          </span>
          <span class="flex shrink-0 items-center gap-2">
            <span v-if="m.user_id === group.created_by" class="text-xs uppercase tracking-wider text-muted-foreground">creator</span>
            <button
              v-if="isCreator && m.user_id !== group.created_by"
              type="button"
              class="btn-danger-outline btn-sm"
              @click="removeTarget = m.user_id"
            >
              <Icon name="trash" /><span>Remove</span>
            </button>
          </span>
        </li>
      </ul>
      <Alert v-if="isPair && hasDefaultSplit" tone="info" class="my-2">
        Adding a 3rd member will clear the pinned default split. New expenses will fall back to an equal split until you set a new default.
      </Alert>
      <form class="flex items-end gap-2" @submit.prevent="onAddMember">
        <div class="flex-1">
          <Field v-model="newEmail" label="New member email" type="email" required error="Enter a valid email address" />
        </div>
        <button type="submit" class="btn-primary">Add</button>
      </form>
      <p class="text-xs text-subtle-foreground">The invitee must already be registered.</p>
    </section>

    <!-- Default split -->
    <section class="mb-4 rounded-md border border-border bg-card p-3">
      <h2 class="mb-3 font-medium">Default split</h2>
      <p class="mb-3 text-sm text-subtle-foreground">
        {{
          !isPair
            ? "Only available for 2-member groups. New expenses default to an equal split."
            : hasDefaultSplit
              ? "New expenses prefill with this percentage split. The split editor still lets you change it per-expense."
              : "Pin a percentage split (e.g. 60/40) to prefill new expenses. Cleared automatically if a 3rd member joins."
        }}
      </p>
      <form v-if="isPair" class="flex flex-col gap-2" @submit.prevent="onSaveSplit">
        <div class="flex items-center gap-2 text-sm">
          <span class="flex-1">{{ members[0].display_name }}</span>
          <input v-model="split1" type="number" step="0.01" min="0" max="100" required class="field-input-num w-20" />
          <span class="text-sm text-subtle-foreground">%</span>
        </div>
        <div class="flex items-center gap-2 text-sm">
          <span class="flex-1">{{ members[1].display_name }}</span>
          <input v-model="split2" type="number" step="0.01" min="0" max="100" required class="field-input-num w-20" />
          <span class="text-sm text-subtle-foreground">%</span>
        </div>
        <p class="text-xs text-subtle-foreground">Must sum to 100.</p>
        <div class="flex justify-end">
          <button type="submit" class="btn-primary btn-sm">{{ hasDefaultSplit ? "Update default" : "Pin default" }}</button>
        </div>
      </form>
      <div v-if="hasDefaultSplit && isPair" class="mt-2 flex justify-end">
        <button type="button" class="btn-secondary btn-sm" @click="onClearSplit">Clear default</button>
      </div>
    </section>

    <!-- Export / Import (wired in D6) -->
    <section class="mb-4 rounded-md border border-border bg-card p-3">
      <h2 class="mb-3 font-medium">Export</h2>
      <p class="mb-3 text-sm text-subtle-foreground">
        Download every expense and settlement in this group as a CSV file. The format is a superset of Splitwise's export.
      </p>
      <div class="flex justify-end">
        <RouterLink :to="`/groups/${groupId}/export`" class="btn-primary">
          <Icon name="download" /><span>Export CSV</span>
        </RouterLink>
      </div>
    </section>

    <section class="mb-4 rounded-md border border-border bg-card p-3">
      <h2 class="mb-3 font-medium">Import expenses</h2>
      <p class="mb-3 text-sm text-subtle-foreground">
        Bulk-add expenses from a DoTheSplit-shaped CSV. Splits use this group's default rule.
      </p>
      <div class="flex justify-end">
        <RouterLink :to="`/groups/${groupId}/import-expenses`" class="btn-primary">
          <Icon name="upload" /><span>Import expenses</span>
        </RouterLink>
      </div>
    </section>

    <!-- Leave (non-creator) -->
    <section v-if="!isCreator" class="rounded-md border border-red-200 bg-card p-4 dark:border-red-900">
      <h2 class="mb-2 text-sm font-semibold uppercase tracking-wide text-red-600 dark:text-red-400">Leave group</h2>
      <p class="mb-3 text-sm text-subtle-foreground">Removes you from this group. Your past expenses, splits, and settlements stay in the ledger.</p>
      <Alert v-if="myNetCents !== 0" tone="info" class="mb-3">
        You have a non-zero balance of <span class="[font-family:var(--font-mono)]">{{ formatMoney(myNetCents, group.default_currency) }}</span> in this group.
        {{ myNetCents > 0 ? "Leaving without settling up means others won't be reminded to pay you back." : "Leaving without settling up means you won't be reminded to pay your share." }}
        Consider settling up first.
      </Alert>
      <div class="flex justify-end">
        <button type="button" class="btn-danger" @click="leaveConfirm = true">
          <Icon name="right-from-bracket" /><span>Leave group</span>
        </button>
      </div>
    </section>

    <!-- Transfer ownership (creator, >1 member) -->
    <section v-if="isCreator && members.length > 1" class="mb-4 rounded-md border border-border bg-card p-3">
      <h2 class="mb-3 font-medium">Transfer ownership</h2>
      <p class="mb-3 text-sm text-subtle-foreground">
        Hand the group to another member. You stay in the group as a regular member, and the new owner takes over creator-only powers.
      </p>
      <form class="flex flex-col gap-2 sm:flex-row sm:items-end" @submit.prevent="transferConfirm = true">
        <label class="field-select-row sm:flex-1">
          <span>New owner <span class="text-destructive">*</span></span>
          <select v-model="newOwnerId" required class="field-select">
            <option value="">Pick a member…</option>
            <option v-for="m in members.filter((x) => x.user_id !== group!.created_by)" :key="m.user_id" :value="m.user_id">
              {{ m.display_name }}
            </option>
          </select>
        </label>
        <button type="submit" class="btn-primary">Transfer</button>
      </form>
    </section>

    <!-- Delete (creator) -->
    <section v-if="isCreator" class="rounded-md border border-red-200 bg-card p-4 dark:border-red-900">
      <h2 class="mb-2 text-sm font-semibold uppercase tracking-wide text-red-600 dark:text-red-400">Danger zone</h2>
      <p class="mb-3 text-sm text-subtle-foreground">Deleting the group removes all its expenses, settlements and recurring templates. This cannot be undone.</p>
      <div class="flex justify-end">
        <button type="button" class="btn-danger" @click="deleteConfirm = true">
          <Icon name="trash" /><span>Delete group</span>
        </button>
      </div>
    </section>

    <ConfirmDialog
      :open="removeTarget !== null"
      title="Remove member?"
      message="Their past expenses, splits and settlements stay in the group ledger. They can be re-added later."
      confirm-label="Remove member"
      confirm-icon="trash"
      @update:open="(v) => { if (!v) removeTarget = null; }"
      @confirm="onRemoveMember"
    />
    <ConfirmDialog
      v-model:open="leaveConfirm"
      title="Leave this group?"
      :message="myNetCents !== 0 ? 'You have a non-zero balance in this group. Leaving without settling up keeps the ledger entry pointing at you. Continue?' : 'You will be removed from the member list. Your past expenses, splits and settlements stay in the ledger.'"
      confirm-label="Leave group"
      confirm-icon="right-from-bracket"
      @confirm="onLeave"
    />
    <ConfirmDialog
      v-model:open="transferConfirm"
      title="Transfer ownership?"
      message="The selected member becomes the new owner. You will keep being a regular member of the group."
      confirm-label="Transfer ownership"
      confirm-variant="primary"
      @confirm="onTransfer"
    />
    <ConfirmDialog
      v-model:open="deleteConfirm"
      title="Delete this group?"
      message="All expenses, settlements and recurring templates in this group are removed permanently. This cannot be undone."
      confirm-label="Delete group"
      confirm-icon="trash"
      @confirm="onDeleteGroup"
    />
  </AppLayout>
</template>
