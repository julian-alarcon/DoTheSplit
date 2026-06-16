<script setup lang="ts">
// One row in the group activity feed: who did what, what it was, when, and the
// amount. Links to the expense/settlement detail page.
import { computed } from "vue";
import { RouterLink } from "vue-router";
import { formatMoney } from "@/lib/currencies";
import { shortName } from "@/lib/short-name";
import type { components } from "@/lib/api/schema";
import Icon from "@/components/Icon.vue";
import CategoryIcon from "@/components/CategoryIcon.vue";
import MemberAvatar from "@/components/MemberAvatar.vue";

type ActivityItem = components["schemas"]["ActivityItem"];
type GroupMember = components["schemas"]["GroupMember"];

const props = defineProps<{
  item: ActivityItem;
  groupId: string;
  viewerId: string;
  members: GroupMember[];
}>();

const memberByID = computed(() => new Map(props.members.map((m) => [m.user_id, m])));
const nameByID = computed(() => new Map(props.members.map((m) => [m.user_id, m.display_name])));

const isSettlement = computed(() => props.item.target_kind === "settlement");
const href = computed(() =>
  isSettlement.value
    ? `/groups/${props.groupId}/settlements/${props.item.target_id}`
    : `/groups/${props.groupId}/expenses/${props.item.target_id}`,
);

const who = computed(() => {
  const { actor, recurring } = props.item;
  if (!actor) return recurring ? "Recurring" : "System";
  if (actor.user_id === props.viewerId) return "You";
  return shortName(actor.display_name);
});

const verb = computed(() => {
  const a = props.item.action;
  if (a.endsWith(".created")) return "added";
  if (a.endsWith(".updated")) return "updated";
  if (a.endsWith(".restored")) return "restored";
  return "deleted";
});

const recurringPrefix = computed(() => props.item.recurring && props.item.action === "expense.created");

const dateFmt = new Intl.DateTimeFormat(undefined, {
  year: "numeric",
  month: "short",
  day: "2-digit",
  hour: "2-digit",
  minute: "2-digit",
});

const fromMember = computed(() => (props.item.from_user_id ? memberByID.value.get(props.item.from_user_id) : undefined));
const toMember = computed(() => (props.item.to_user_id ? memberByID.value.get(props.item.to_user_id) : undefined));
</script>

<template>
  <li>
    <RouterLink :to="href" class="row">
      <div class="row-left">
        <span v-if="isSettlement" class="settle-icon" title="Settlement">
          <Icon name="arrow-right" :size="16" />
        </span>
        <span v-else class="cat-icon" :title="item.category_slug ?? ''">
          <CategoryIcon :slug="item.category_slug" :group-label="item.category_group_label" :size="32" />
        </span>
        <div class="row-body">
          <div class="line-who">
            <MemberAvatar
              v-if="item.actor"
              :user-id="item.actor.user_id"
              :display-name="item.actor.display_name"
              :has-avatar="item.actor.has_avatar"
              :avatar-updated-at="item.actor.avatar_updated_at"
              :size="16"
            />
            <span class="who-text">
              <span v-if="recurringPrefix" class="recurring-prefix">New recurring expense: </span>
              <span class="who-name">{{ who }}</span>
              <span class="muted">&nbsp;{{ verb }}</span>
            </span>
          </div>

          <div v-if="isSettlement" class="line-settle">
            <span class="who-pair">
              <MemberAvatar
                :user-id="item.from_user_id ?? ''"
                :display-name="fromMember?.display_name ?? '?'"
                :has-avatar="fromMember?.has_avatar"
                :avatar-updated-at="fromMember?.avatar_updated_at"
                :size="16"
              />
              <span class="trunc strong">{{ shortName(nameByID.get(item.from_user_id ?? "")) }}</span>
            </span>
            <span class="muted">paid</span>
            <span class="who-pair">
              <MemberAvatar
                :user-id="item.to_user_id ?? ''"
                :display-name="toMember?.display_name ?? '?'"
                :has-avatar="toMember?.has_avatar"
                :avatar-updated-at="toMember?.avatar_updated_at"
                :size="16"
              />
              <span class="trunc strong">{{ shortName(nameByID.get(item.to_user_id ?? "")) }}</span>
            </span>
            <span v-if="item.description" class="settle-note">{{ item.description }}</span>
          </div>
          <span v-else class="line-desc">&ldquo;{{ item.description }}&rdquo;</span>

          <span class="line-when">{{ dateFmt.format(new Date(item.occurred_at)) }}</span>
        </div>
      </div>
      <span class="amount">{{ formatMoney(item.amount_cents, item.currency) }}</span>
    </RouterLink>
  </li>
</template>

<style scoped>
.row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.625rem 0.25rem;
  transition: background-color 120ms ease;
}
.row:hover {
  background: var(--hover-surface);
}
.row:focus-visible {
  outline: 2px solid var(--ring);
  outline-offset: 2px;
}
.row-left {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.75rem;
}
.settle-icon {
  flex-shrink: 0;
  display: inline-flex;
  height: 2rem;
  width: 2rem;
  align-items: center;
  justify-content: center;
  border-radius: 9999px;
  background: oklch(95% 0.052 163.051); /* emerald-100 */
  color: oklch(50.8% 0.118 165.612); /* emerald-700 */
}
:root[data-theme="dark"] .settle-icon,
:root[data-theme="high-contrast"] .settle-icon {
  background: oklch(37.8% 0.077 168.94); /* emerald-900 */
  color: oklch(84.5% 0.143 164.978); /* emerald-300 */
}
.cat-icon {
  flex-shrink: 0;
  display: inline-flex;
  height: 2rem;
  width: 2rem;
  align-items: center;
  justify-content: center;
}
.row-body {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 0.125rem;
}
.line-who {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.375rem;
  font-size: 0.875rem;
}
.who-text {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.recurring-prefix {
  font-weight: 500;
  color: var(--muted-foreground);
}
.who-name {
  font-weight: 500;
}
.muted {
  color: var(--muted-foreground);
}
.line-settle {
  display: flex;
  min-width: 0;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.125rem 0.375rem;
  font-size: 0.875rem;
}
.who-pair {
  display: inline-flex;
  min-width: 0;
  align-items: center;
  gap: 0.25rem;
}
.trunc {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.strong {
  font-weight: 500;
}
.settle-note {
  min-width: 0;
  flex-basis: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 11px;
  color: var(--muted-foreground);
}
.line-desc {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.875rem;
  font-weight: 500;
}
.line-when {
  font-size: 11px;
  font-variant-numeric: tabular-nums;
  color: var(--muted-foreground);
}
.amount {
  flex-shrink: 0;
  font-family: var(--font-mono);
  font-size: 1rem;
  font-variant-numeric: tabular-nums;
}
</style>
