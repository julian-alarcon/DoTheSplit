<script setup lang="ts">
// One row in the group activity feed: who did what, what it was, when, and the
// amount. Links to the expense/settlement detail page.
import { computed } from "vue";
import { RouterLink } from "vue-router";
import { useGroupMembers } from "@/composables/useGroupMembers";
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

const { memberByID, nameByID } = useGroupMembers(() => props.members);

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
    <RouterLink :to="href" class="flex items-center justify-between gap-3 px-1 py-2.5 transition-colors hover:bg-hover-surface focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring">
      <div class="flex min-w-0 items-center gap-3">
        <span v-if="isSettlement" class="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-emerald-100 text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300" title="Settlement">
          <Icon name="arrow-right" :size="16" />
        </span>
        <span v-else class="inline-flex h-8 w-8 shrink-0 items-center justify-center" :title="item.category_slug ?? ''">
          <CategoryIcon :slug="item.category_slug" :group-label="item.category_group_label" :size="32" />
        </span>
        <div class="flex min-w-0 flex-col gap-0.5">
          <div class="flex min-w-0 items-center gap-1.5 text-sm">
            <MemberAvatar
              v-if="item.actor"
              :user-id="item.actor.user_id"
              :display-name="item.actor.display_name"
              :has-avatar="item.actor.has_avatar"
              :avatar-updated-at="item.actor.avatar_updated_at"
              :size="16"
            />
            <span class="min-w-0 truncate">
              <span v-if="recurringPrefix" class="font-medium text-muted-foreground">New recurring expense: </span>
              <span class="font-medium">{{ who }}</span>
              <span class="text-muted-foreground">&nbsp;{{ verb }}</span>
            </span>
          </div>

          <div v-if="isSettlement" class="flex min-w-0 flex-wrap items-center gap-x-1.5 gap-y-0.5 text-sm">
            <span class="inline-flex min-w-0 items-center gap-1">
              <MemberAvatar
                :user-id="item.from_user_id ?? ''"
                :display-name="fromMember?.display_name ?? '?'"
                :has-avatar="fromMember?.has_avatar"
                :avatar-updated-at="fromMember?.avatar_updated_at"
                :size="16"
              />
              <span class="truncate font-medium">{{ shortName(nameByID.get(item.from_user_id ?? "")) }}</span>
            </span>
            <span class="text-muted-foreground">paid</span>
            <span class="inline-flex min-w-0 items-center gap-1">
              <MemberAvatar
                :user-id="item.to_user_id ?? ''"
                :display-name="toMember?.display_name ?? '?'"
                :has-avatar="toMember?.has_avatar"
                :avatar-updated-at="toMember?.avatar_updated_at"
                :size="16"
              />
              <span class="truncate font-medium">{{ shortName(nameByID.get(item.to_user_id ?? "")) }}</span>
            </span>
            <span v-if="item.description" class="min-w-0 basis-full truncate text-[11px] text-muted-foreground">{{ item.description }}</span>
          </div>
          <span v-else class="min-w-0 truncate text-sm font-medium">&ldquo;{{ item.description }}&rdquo;</span>

          <span class="text-[11px] tabular-nums text-muted-foreground">{{ dateFmt.format(new Date(item.occurred_at)) }}</span>
        </div>
      </div>
      <span class="shrink-0 text-base tabular-nums [font-family:var(--font-mono)]">{{ formatMoney(item.amount_cents, item.currency) }}</span>
    </RouterLink>
  </li>
</template>
