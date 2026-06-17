<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useAuth } from "@/composables/useAuth";
import { listCategories, listGroups } from "@/composables/useGroups";
import { search, type SearchGroupRef } from "@/composables/useSearch";
import type { components } from "@/lib/api/schema";
import { moneyFormatter, formatMoney } from "@/lib/currencies";
import { shortName } from "@/lib/short-name";
import AppLayout from "@/components/AppLayout.vue";
import Alert from "@/components/Alert.vue";
import Icon from "@/components/Icon.vue";
import CategoryIcon from "@/components/CategoryIcon.vue";
import MemberAvatar from "@/components/MemberAvatar.vue";

type SearchItem = components["schemas"]["TransactionItem"];
type Category = components["schemas"]["Category"];

const route = useRoute();
const router = useRouter();
const { state } = useAuth();

const q = ref(typeof route.query.q === "string" ? route.query.q : "");
const groupFilter = ref(typeof route.query.group_id === "string" ? route.query.group_id : "");
const categoryFilter = ref(typeof route.query.category_id === "string" ? route.query.category_id : "");

const items = ref<SearchItem[]>([]);
const resultGroups = ref<SearchGroupRef[]>([]);
const availableCategoryIDs = ref<string[]>([]);
const usedQuery = ref("");
const queryError = ref<string | null>(null);
const hasSearched = ref(false);

const allGroups = ref<SearchGroupRef[]>([]);
const categories = ref<Category[]>([]);
const filtersOpen = ref(false);

const groupDialog = ref<HTMLDialogElement | null>(null);
const categoryDialog = ref<HTMLDialogElement | null>(null);

const categoryByID = computed(() => new Map(categories.value.map((c) => [c.id, c])));
const viewerId = computed(() => state.user?.id ?? "");

const groupByID = computed(() => {
  const m = new Map<string, SearchGroupRef>();
  for (const g of resultGroups.value) m.set(g.id, g);
  for (const g of allGroups.value) if (!m.has(g.id)) m.set(g.id, g);
  return m;
});

const selectedGroupLabel = computed(
  () => allGroups.value.find((g) => g.id === groupFilter.value)?.name ?? "All",
);
const selectedCategory = computed(() => categoryByID.value.get(categoryFilter.value));
const selectedCategoryLabel = computed(() => selectedCategory.value?.label ?? "All");

const visibleCategories = computed(() => {
  if (!hasSearched.value) return categories.value;
  const set = new Set(availableCategoryIDs.value);
  if (categoryFilter.value) set.add(categoryFilter.value);
  return categories.value.filter((c) => set.has(c.id));
});

const monthFmt = new Intl.DateTimeFormat(undefined, { month: "short" });
const dayFmt = new Intl.DateTimeFormat(undefined, { day: "2-digit" });
const yearFmt = new Intl.DateTimeFormat(undefined, { year: "numeric" });

type ViewerStake = { kind: "lent"; cents: number } | { kind: "owes"; cents: number } | { kind: "none" };
function viewerStake(e: {
  payer_id: string;
  amount_cents: number;
  splits: Array<{ user_id: string; share_cents: number }>;
}): ViewerStake {
  const me = viewerId.value;
  if (!me) return { kind: "none" };
  const myShare = e.splits.find((s) => s.user_id === me)?.share_cents ?? 0;
  const iPaid = e.payer_id === me;
  if (!iPaid && !e.splits.some((s) => s.user_id === me)) return { kind: "none" };
  if (iPaid) {
    const lent = e.amount_cents - myShare;
    return lent > 0 ? { kind: "lent", cents: lent } : { kind: "none" };
  }
  return myShare > 0 ? { kind: "owes", cents: myShare } : { kind: "none" };
}

function groupForItem(item: SearchItem): SearchGroupRef | undefined {
  const gid = item.expense?.group_id ?? item.settlement?.group_id;
  return gid ? groupByID.value.get(gid) : undefined;
}

// Build group-header + item rows from the flat result list.
type Row =
  | { kind: "group-header"; group: SearchGroupRef; count: number }
  | { kind: "item"; item: SearchItem };
const rows = computed<Row[]>(() => {
  const out: Row[] = [];
  let lastGroupID: string | null = null;
  let headerIndex = -1;
  let running = 0;
  for (const item of items.value) {
    const g = groupForItem(item);
    if (!g) continue;
    if (g.id !== lastGroupID) {
      if (headerIndex >= 0) (out[headerIndex] as Extract<Row, { kind: "group-header" }>).count = running;
      out.push({ kind: "group-header", group: g, count: 0 });
      headerIndex = out.length - 1;
      running = 0;
      lastGroupID = g.id;
    }
    out.push({ kind: "item", item });
    running++;
  }
  if (headerIndex >= 0) (out[headerIndex] as Extract<Row, { kind: "group-header" }>).count = running;
  return out;
});

function memberByID(g: SearchGroupRef) {
  return new Map(g.members.map((m) => [m.user_id, m]));
}

async function runSearch() {
  const term = q.value.trim();
  queryError.value = null;
  if (term.length === 1) {
    queryError.value = "Type at least 2 characters to search.";
    items.value = [];
    return;
  }
  if (term.length < 2) {
    items.value = [];
    hasSearched.value = false;
    return;
  }
  hasSearched.value = true;
  const { data, status } = await search(term, {
    groupId: groupFilter.value || undefined,
    categoryId: categoryFilter.value || undefined,
  });
  if (!data) {
    queryError.value = status === 400 ? "Search query is too short." : "Search failed.";
    items.value = [];
    return;
  }
  items.value = data.items;
  resultGroups.value = data.groups;
  availableCategoryIDs.value = data.available_category_ids;
  usedQuery.value = data.query;
}

// Reflect filters in the URL (shareable), then re-search.
function syncUrl() {
  const query: Record<string, string> = {};
  if (q.value.trim()) query.q = q.value.trim();
  if (groupFilter.value) query.group_id = groupFilter.value;
  if (categoryFilter.value) query.category_id = categoryFilter.value;
  router.replace({ query });
}

function onSubmit() {
  syncUrl();
  runSearch();
}
function pickGroup(id: string) {
  groupFilter.value = id;
  groupDialog.value?.close();
  syncUrl();
  runSearch();
}
function pickCategory(id: string) {
  categoryFilter.value = id;
  categoryDialog.value?.close();
  syncUrl();
  runSearch();
}

function isNewGroupRow(list: Category[], i: number): boolean {
  return i === 0 || list[i - 1].group_label !== list[i].group_label;
}

watch(
  () => route.query,
  () => {
    q.value = typeof route.query.q === "string" ? route.query.q : "";
    groupFilter.value = typeof route.query.group_id === "string" ? route.query.group_id : "";
    categoryFilter.value = typeof route.query.category_id === "string" ? route.query.category_id : "";
  },
);

onMounted(async () => {
  filtersOpen.value = Boolean(groupFilter.value || categoryFilter.value);
  const [groupsRes, cats] = await Promise.all([listGroups(), listCategories()]);
  allGroups.value = groupsRes.groups.map((g) => ({
    id: g.id,
    name: g.name,
    default_currency: g.default_currency,
    members: g.members,
  }));
  categories.value = cats;
  if (q.value.trim().length >= 2) await runSearch();
});
</script>

<template>
  <AppLayout :back="{ to: '/groups', label: 'Groups' }">
    <h1 class="mb-4 text-2xl font-semibold">Search</h1>

    <form class="mb-4 flex flex-col gap-3" @submit.prevent="onSubmit">
      <label class="field">
        <input
          v-model="q"
          type="search"
          minlength="2"
          maxlength="200"
          autocomplete="off"
          placeholder=" "
          class="field-input"
        />
        <span class="field-label">Search expense and settlement notes</span>
      </label>
      <div class="flex justify-end">
        <button type="submit" class="btn-primary">
          <Icon name="magnifying-glass" /><span>Search</span>
        </button>
      </div>

      <details :open="filtersOpen" class="flex flex-col gap-3">
        <summary class="btn-secondary btn-xs w-fit list-none [&::-webkit-details-marker]:hidden">
          <Icon name="filter" :size="12" />
          <span>Filter</span>
        </summary>
        <div class="mt-3 flex flex-wrap items-center gap-2">
          <span class="text-xs font-semibold uppercase tracking-wider text-muted-foreground">Filter:</span>
          <button type="button" class="inline-flex cursor-pointer items-center gap-1.5 rounded-full border border-border bg-card px-3 py-1 text-xs font-medium text-card-foreground transition-colors hover:bg-hover-surface hover:text-hover-foreground" @click="groupDialog?.showModal()">
            <span class="text-muted-foreground">Group:</span>
            <span class="max-w-40 truncate">{{ selectedGroupLabel }}</span>
            <Icon name="chevron-down" :size="10" />
          </button>
          <button type="button" class="inline-flex cursor-pointer items-center gap-1.5 rounded-full border border-border bg-card px-3 py-1 text-xs font-medium text-card-foreground transition-colors hover:bg-hover-surface hover:text-hover-foreground" @click="categoryDialog?.showModal()">
            <span class="text-muted-foreground">Category:</span>
            <CategoryIcon
              v-if="selectedCategory"
              :slug="selectedCategory.slug"
              :group-label="selectedCategory.group_label"
              :size="14"
            />
            <span class="max-w-40 truncate">{{ selectedCategoryLabel }}</span>
            <Icon name="chevron-down" :size="10" />
          </button>
        </div>
      </details>
    </form>

    <Alert v-if="queryError" tone="info" class="mb-4">{{ queryError }}</Alert>

    <p v-if="q.trim().length < 2" class="text-muted-foreground">
      Type at least 2 characters to search across all your groups.
    </p>
    <p v-else-if="items.length === 0 && !queryError" class="text-muted-foreground">
      No matches for <span class="font-medium">{{ usedQuery }}</span
      >.
    </p>

    <ul v-else class="flex list-none flex-col gap-2">
      <template v-for="(row, i) in rows" :key="i">
        <li v-if="row.kind === 'group-header'" class="flex items-baseline justify-between gap-3 px-1 pt-3 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
          <RouterLink :to="`/groups/${row.group.id}`" class="truncate hover:underline">{{ row.group.name }}</RouterLink>
          <span class="tabular-nums normal-case">{{ row.count }} {{ row.count === 1 ? "match" : "matches" }}</span>
        </li>
        <li v-else-if="row.item.expense">
          <RouterLink
            :to="`/groups/${row.item.expense.group_id}/expenses/${row.item.expense.id}`"
            class="flex items-stretch justify-between gap-1.5 rounded-md border border-border bg-card px-4 py-3 hover:bg-hover-surface"
          >
            <div class="flex min-w-0 items-center gap-3">
              <span class="inline-flex w-7 shrink-0 flex-col items-center justify-center leading-none" :title="categoryByID.get(row.item.expense.category_id)?.label ?? ''">
                <CategoryIcon
                  :slug="categoryByID.get(row.item.expense.category_id)?.slug"
                  :group-label="categoryByID.get(row.item.expense.category_id)?.group_label"
                  :size="28"
                />
                <span class="text-[8px] font-semibold uppercase tracking-wider text-muted-foreground">{{ monthFmt.format(new Date(row.item.expense.incurred_at)) }}</span>
                <span class="text-xs font-semibold tabular-nums text-muted-foreground">{{ dayFmt.format(new Date(row.item.expense.incurred_at)) }}</span>
                <span class="text-[8px] font-semibold uppercase tracking-wider text-muted-foreground">{{ yearFmt.format(new Date(row.item.expense.incurred_at)) }}</span>
              </span>
              <div class="min-w-0">
                <div class="truncate font-medium">{{ row.item.expense.description }}</div>
                <div class="flex flex-wrap items-center gap-1.5 text-[11px] text-muted-foreground">
                  <span>paid by</span>
                  <MemberAvatar
                    :user-id="row.item.expense.payer_id"
                    :display-name="groupForItem(row.item) ? memberByID(groupForItem(row.item)!).get(row.item.expense.payer_id)?.display_name ?? '?' : '?'"
                    :has-avatar="groupForItem(row.item) ? memberByID(groupForItem(row.item)!).get(row.item.expense.payer_id)?.has_avatar : false"
                    :avatar-updated-at="groupForItem(row.item) ? memberByID(groupForItem(row.item)!).get(row.item.expense.payer_id)?.avatar_updated_at : null"
                    :size="12"
                  />
                  <span class="truncate">{{ shortName(groupForItem(row.item) ? memberByID(groupForItem(row.item)!).get(row.item.expense.payer_id)?.display_name : "?") }}</span>
                </div>
              </div>
            </div>
            <div class="flex shrink-0 flex-col items-end justify-between gap-1">
              <span class="text-[11px] tabular-nums">
                <template v-if="viewerStake(row.item.expense).kind === 'lent'">
                  <span class="text-emerald-900 dark:text-emerald-200">you lent </span>
                  <span class="[font-family:var(--font-mono)]">{{ moneyFormatter(row.item.expense.currency).format((viewerStake(row.item.expense) as { cents: number }).cents / 100) }}</span>
                </template>
                <template v-else-if="viewerStake(row.item.expense).kind === 'owes'">
                  <span class="text-amber-700 dark:text-amber-200">you owe </span>
                  <span class="[font-family:var(--font-mono)]">{{ moneyFormatter(row.item.expense.currency).format((viewerStake(row.item.expense) as { cents: number }).cents / 100) }}</span>
                </template>
                <span v-else class="text-muted-foreground">not involved</span>
              </span>
              <span class="shrink-0 self-center text-lg tabular-nums [font-family:var(--font-mono)]">{{ moneyFormatter(row.item.expense.currency).format(row.item.expense.amount_cents / 100) }}</span>
            </div>
          </RouterLink>
        </li>
        <li v-else-if="row.item.settlement">
          <RouterLink
            :to="`/groups/${row.item.settlement.group_id}/settlements/${row.item.settlement.id}`"
            class="flex items-stretch justify-between gap-1.5 rounded-md border border-emerald-200 bg-emerald-50 px-4 py-3 hover:bg-emerald-100 dark:border-emerald-900 dark:bg-emerald-950/40 dark:hover:bg-emerald-950/60"
          >
            <div class="flex min-w-0 items-center gap-3">
              <span class="inline-flex w-7 shrink-0 flex-col items-center justify-center leading-none" title="Settlement">
                <span class="inline-flex h-7 w-7 items-center justify-center rounded-full bg-emerald-100 text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300"><Icon name="arrow-right" :size="14" /></span>
                <span class="text-[8px] font-semibold uppercase tracking-wider text-muted-foreground">{{ monthFmt.format(new Date(row.item.settlement.settled_at)) }}</span>
                <span class="text-xs font-semibold tabular-nums text-muted-foreground">{{ dayFmt.format(new Date(row.item.settlement.settled_at)) }}</span>
                <span class="text-[8px] font-semibold uppercase tracking-wider text-muted-foreground">{{ yearFmt.format(new Date(row.item.settlement.settled_at)) }}</span>
              </span>
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-1.5 font-medium">
                  <MemberAvatar
                    :user-id="row.item.settlement.from_user_id"
                    :display-name="groupForItem(row.item) ? memberByID(groupForItem(row.item)!).get(row.item.settlement.from_user_id)?.display_name ?? '?' : '?'"
                    :has-avatar="groupForItem(row.item) ? memberByID(groupForItem(row.item)!).get(row.item.settlement.from_user_id)?.has_avatar : false"
                    :avatar-updated-at="groupForItem(row.item) ? memberByID(groupForItem(row.item)!).get(row.item.settlement.from_user_id)?.avatar_updated_at : null"
                    :size="16"
                  />
                  <span class="truncate">{{ shortName(groupForItem(row.item) ? memberByID(groupForItem(row.item)!).get(row.item.settlement.from_user_id)?.display_name : "?") }}</span>
                  <span class="text-muted-foreground">paid</span>
                  <MemberAvatar
                    :user-id="row.item.settlement.to_user_id"
                    :display-name="groupForItem(row.item) ? memberByID(groupForItem(row.item)!).get(row.item.settlement.to_user_id)?.display_name ?? '?' : '?'"
                    :has-avatar="groupForItem(row.item) ? memberByID(groupForItem(row.item)!).get(row.item.settlement.to_user_id)?.has_avatar : false"
                    :avatar-updated-at="groupForItem(row.item) ? memberByID(groupForItem(row.item)!).get(row.item.settlement.to_user_id)?.avatar_updated_at : null"
                    :size="16"
                  />
                  <span class="truncate">{{ shortName(groupForItem(row.item) ? memberByID(groupForItem(row.item)!).get(row.item.settlement.to_user_id)?.display_name : "?") }}</span>
                </div>
                <div class="flex flex-wrap items-center gap-1.5 text-[11px] text-muted-foreground">
                  <span>settlement</span>
                  <span v-if="row.item.settlement.note" class="truncate">· {{ row.item.settlement.note }}</span>
                </div>
              </div>
            </div>
            <span class="shrink-0 self-center text-lg tabular-nums [font-family:var(--font-mono)]">{{ formatMoney(row.item.settlement.amount_cents, groupForItem(row.item)?.default_currency ?? "EUR") }}</span>
          </RouterLink>
        </li>
      </template>
    </ul>

    <!-- Group filter dialog -->
    <dialog
      ref="groupDialog"
      class="fixed inset-0 m-auto w-[calc(100%-2rem)] max-w-96 rounded-md border border-border bg-popover p-0 text-popover-foreground shadow-[0_20px_50px_rgba(0,0,0,0.35)] backdrop:bg-backdrop"
      aria-modal="true"
      aria-label="Filter by group"
    >
      <div class="flex flex-col gap-3 p-5">
        <div class="flex items-center justify-between gap-3">
          <h3 class="text-lg font-medium">Filter by group</h3>
          <button type="button" class="cursor-pointer rounded-md px-2 py-1 text-muted-foreground" aria-label="Close" @click="groupDialog?.close()">
            <Icon name="xmark" :size="14" />
          </button>
        </div>
        <ul class="flex max-h-[60vh] list-none flex-col gap-0.5 overflow-auto">
          <li>
            <button type="button" class="field-category-option" @click="pickGroup('')">
              <Icon name="layer-group" :size="16" /><span>All groups</span>
            </button>
          </li>
          <li v-for="g in allGroups" :key="g.id">
            <button type="button" class="field-category-option" @click="pickGroup(g.id)">
              <Icon name="users" :size="16" /><span>{{ g.name }}</span>
            </button>
          </li>
        </ul>
      </div>
    </dialog>

    <!-- Category filter dialog -->
    <dialog
      ref="categoryDialog"
      class="fixed inset-0 m-auto w-[calc(100%-2rem)] max-w-96 rounded-md border border-border bg-popover p-0 text-popover-foreground shadow-[0_20px_50px_rgba(0,0,0,0.35)] backdrop:bg-backdrop"
      aria-modal="true"
      aria-label="Filter by category"
    >
      <div class="flex flex-col gap-3 p-5">
        <div class="flex items-center justify-between gap-3">
          <h3 class="text-lg font-medium">Filter by category</h3>
          <button type="button" class="cursor-pointer rounded-md px-2 py-1 text-muted-foreground" aria-label="Close" @click="categoryDialog?.close()">
            <Icon name="xmark" :size="14" />
          </button>
        </div>
        <ul class="flex max-h-[60vh] list-none flex-col gap-0.5 overflow-auto">
          <li>
            <button type="button" class="field-category-option" @click="pickCategory('')">
              <Icon name="layer-group" :size="16" /><span>All categories</span>
            </button>
          </li>
          <template v-for="(c, i) in visibleCategories" :key="c.id">
            <li v-if="isNewGroupRow(visibleCategories, i)" class="field-category-group">{{ c.group_label }}</li>
            <li>
              <button type="button" class="field-category-option" @click="pickCategory(c.id)">
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
